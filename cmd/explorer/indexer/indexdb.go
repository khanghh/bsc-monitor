package indexer

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/extdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/labstack/gommon/log"
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
	Update(root common.Hash, data *indexData) *diffLayer
}

// IndexDB store index data for account, also take care of caching things
type IndexDB struct {
	diskdb   ethdb.Database
	triedb   *trie.Database
	cache    int
	diskRoot common.Hash
	layers   map[common.Hash]indexLayer
	lock     sync.RWMutex
}

func (db *IndexDB) DiskDB() ethdb.Database {
	return db.diskdb
}

func (db *IndexDB) DiskRoot() common.Hash {
	return db.diskRoot
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
	return newDiskLayer(diskdb, triedb, root, cache), nil
}

func (db *IndexDB) Commit(root common.Hash) error {
	return nil
}

func (db *IndexDB) getLayer(root common.Hash) indexLayer {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.layers[root]
}

func (db *IndexDB) hasLayer(root common.Hash) bool {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.layers[root] != nil
}

// Cap traverses downwards the tree from the given node until the number of allowed layers are crossed.
// All layers ersisted all layers beyond the root
func (db *IndexDB) cap(root common.Hash, layers uint64) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	return nil
}

// update create new index layer descent from the parent root hash
func (db *IndexDB) update(parentRoot, childRoot common.Hash, data *indexData) error {
	if childRoot == parentRoot {
		return ErrCircularUpdate
	}
	if db.hasLayer(childRoot) {
		return ErrCircularUpdate
	}

	parent := db.getLayer(parentRoot)
	if parent == nil {
		return fmt.Errorf("parent layer missing: [#%x]", parentRoot)
	}
	layer := parent.Update(childRoot, data)

	db.lock.Lock()
	defer db.lock.Unlock()
	db.layers[childRoot] = layer
	return nil
}

func NewIndexDB(diskdb ethdb.Database, triedb *trie.Database, root common.Hash, cache int) (*IndexDB, error) {
	indexdb := &IndexDB{
		diskdb: diskdb,
		triedb: triedb,
		cache:  cache,
		layers: make(map[common.Hash]indexLayer),
	}
	diskLayer, err := loadOrCreateDiskLayer(diskdb, triedb, root, cache)
	if err != nil {
		return nil, err
	}
	indexdb.diskRoot = root
	indexdb.layers[root] = diskLayer
	return indexdb, nil
}
