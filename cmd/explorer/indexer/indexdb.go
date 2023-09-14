package indexer

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/extdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type indexData struct {
	blockNum uint64
	// data extracted from block processing
	accounts    map[common.Address]*AccountInfo
	contracts   map[common.Address]*ContractInfo
	accountData map[common.Address]*AccountIndexData
	// generated data for easy access and iteration
	accountStats map[common.Address]*AccountStats
	accountRefs  map[common.Address]*AccountIndexRefs
}

type indexLayer interface {
	Root() common.Hash
	Parent() indexLayer
	AccountInfo(addr common.Address) (*AccountInfo, error)
	ContractInfo(addr common.Address) (*ContractInfo, error)
	AccountStats(addr common.Address) (*AccountStats, error)
	Stale() bool
}

// IndexDB store index data for account, also take care of caching things
type IndexDB struct {
	diskdb ethdb.Database
	triedb *trie.Database
	base   *diskLayer
	layers map[common.Hash]indexLayer
	lock   sync.RWMutex
}

func (db *IndexDB) DiskDB() ethdb.Database {
	return db.diskdb
}

func (db *IndexDB) DiskRoot() common.Hash {
	return db.base.root
}

func (db *IndexDB) Layers() int {
	return len(db.layers)
}

func loadOrCreateDiskLayer(diskdb ethdb.KeyValueStore, triedb *trie.Database, root common.Hash, cache int) (*diskLayer, error) {
	indexRoot := extdb.ReadLastIndexRoot(diskdb)
	if (indexRoot != common.Hash{}) && indexRoot != root {
		indexBlock := extdb.ReadLastIndexBlock(diskdb)
		log.Warn("Index disk root is not continuous with chain", "root", indexRoot.Hex(), "number", indexBlock, "chainroot", root)
	}
	return &diskLayer{
		root:   root,
		diskdb: diskdb,
		triedb: triedb,
		cache:  fastcache.New(cache * 1024 * 1024),
	}, nil
}

func (db *IndexDB) getLayer(root common.Hash) indexLayer {
	db.lock.RLock()
	defer db.lock.RUnlock()
	return db.layers[root]
}

func (db *IndexDB) hasLayer(root common.Hash) bool {
	db.lock.RLock()
	defer db.lock.RUnlock()
	return db.layers[root] != nil
}

// commit traveses downward diffLayer branch until the diskLayer was met and write the index data of every diffLayer to the disk database
func (db *IndexDB) commit(target *diffLayer) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	commitedAccounts := make(map[common.Address]bool)
	commitedContracts := make(map[common.Address]bool)

	commitIndexData := func(batch ethdb.Batch, data *indexData) error {
		// Write all AccountInfo from the layer to database
		for addr, account := range data.accounts {
			if !commitedAccounts[addr] {
				enc, _ := rlp.EncodeToBytes(account)
				extdb.WriteAccountInfo(batch, addr, enc)
				db.base.cache.Set(extdb.AccountInfoKey(addr), enc)
				commitedAccounts[addr] = true
			}
		}
		// Write all ContractInfo from the layer to database
		for addr, contract := range data.contracts {
			if !commitedContracts[addr] {
				enc, _ := rlp.EncodeToBytes(contract)
				extdb.WriteContractInfo(batch, addr, enc)
				db.base.cache.Set(extdb.ContractInfoKey(addr), enc)
				commitedAccounts[addr] = true
			}
		}
		if batch.ValueSize() > ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to persist indexed account/contract info", "error", err)
				return err
			}
			batch.Reset()
		}
		// Write collectted index data for each account
		for addr, index := range data.accountData {
			for idx, txHash := range index.SentTxs {
				ref := extdb.IndexItemRef(data.blockNum, uint64(idx))
				refNum := binary.BigEndian.Uint64(ref)
				extdb.WriteAccountSentTx(batch, addr, refNum, txHash)
			}
			for idx, txHash := range index.InternalTxs {
				ref := extdb.IndexItemRef(data.blockNum, uint64(idx))
				refNum := binary.BigEndian.Uint64(ref)
				extdb.WriteAccountInternalTx(batch, addr, refNum, txHash)
			}
			for idx, txHash := range index.TokenTxs {
				ref := extdb.IndexItemRef(data.blockNum, uint64(idx))
				refNum := binary.BigEndian.Uint64(ref)
				extdb.WriteAccountTokenTx(batch, addr, refNum, txHash)
			}
			for idx, addr := range index.Holders {
				ref := extdb.IndexItemRef(data.blockNum, uint64(idx))
				refNum := binary.BigEndian.Uint64(ref)
				extdb.WriteTokenHolderAddr(batch, addr, refNum, addr)
			}
			if stats, ok := data.accountStats[addr]; ok {
				enc, _ := rlp.EncodeToBytes(stats)
				extdb.WriteAccountStats(batch, addr, enc)
				db.base.cache.Set(extdb.AccountStatsKey(addr), enc)
			}
			if batch.ValueSize() > ethdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					log.Error("Failed to persist index data of account", "account", addr, "error", err)
					return err
				}
				batch.Reset()
			}
		}
		if batch.ValueSize() > 0 {
			return batch.Write()
		}
		return nil
	}

	batch := db.diskdb.NewBatch()
	for layer := indexLayer(target); layer != nil; layer = layer.Parent() {
		if diff, ok := layer.(*diffLayer); ok {
			if err := commitIndexData(batch, diff.data); err != nil {
				return err
			}
			diff.stale = true
		}
		if disk, ok := layer.(*diskLayer); ok {
			disk.stale = true
		}
		delete(db.layers, layer.Root())
	}

	db.layers[target.root] = &diskLayer{
		root:   target.root,
		diskdb: db.diskdb,
		triedb: db.triedb,
		cache:  db.base.cache,
	}

	children := make(map[common.Hash][]common.Hash)
	for root, layer := range db.layers {
		if diff, ok := layer.(*diffLayer); ok {
			parent := diff.parent.Root()
			children[parent] = append(children[parent], root)
		}
	}
	var remove func(root common.Hash)
	remove = func(root common.Hash) {
		delete(db.layers, root)
		for _, child := range children[root] {
			remove(child)
		}
		delete(children, root)
	}
	for root, snap := range db.layers {
		if snap.Stale() {
			remove(root)
		}
	}

	return nil
}

// Cap traverses downwards the tree branch from the given node until the number of allowed layers are crossed, all layers beyond the permitted number
// are persisted into diskdb. The given root hash was consider as a cannonical state root and other branch on the tree will be pruned, remain the given number
// of `tokeep` diffLayer
func (db *IndexDB) Cap(root common.Hash, tokeep int) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	layer := db.getLayer(root)
	if layer == nil {
		return fmt.Errorf("layer not found [%#x]", root)
	}

	diff, ok := layer.(*diffLayer)
	if !ok {
		return fmt.Errorf("root [%#x] is disk layer ", root)
	}

	for i := 0; i < tokeep-1; i++ {
		if parent, ok := diff.parent.(*diffLayer); ok {
			diff = parent
		} else {
			return nil
		}
	}

	return db.commit(diff)
}

// update create new index layer descent from the parent root hash
func (db *IndexDB) update(parentRoot, childRoot common.Hash, data *indexData) error {
	if db.hasLayer(childRoot) || childRoot == parentRoot {
		return ErrCircularUpdate
	}

	parent := db.getLayer(parentRoot)
	if parent == nil {
		return fmt.Errorf("parent layer missing: [#%x]", parentRoot)
	}
	layer := newDiffLayer(parent, childRoot, data)

	db.lock.Lock()
	defer db.lock.Unlock()
	db.layers[childRoot] = layer
	return nil
}

func NewIndexDB(diskdb ethdb.Database, triedb *trie.Database, root common.Hash, cache int) (*IndexDB, error) {
	indexdb := &IndexDB{
		diskdb: diskdb,
		triedb: triedb,
		layers: make(map[common.Hash]indexLayer),
	}
	diskLayer, err := loadOrCreateDiskLayer(diskdb, triedb, root, cache)
	if err != nil {
		return nil, err
	}
	indexdb.base = diskLayer
	indexdb.layers[root] = diskLayer
	return indexdb, nil
}
