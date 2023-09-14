package indexer

import (
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/extdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// diskLayer represents persisted index layer, provides direct access to the index data in the database
type diskLayer struct {
	root   common.Hash
	diskdb ethdb.KeyValueStore
	triedb *trie.Database
	cache  *fastcache.Cache
	stale  bool
	lock   sync.RWMutex
}

func (dl *diskLayer) Root() common.Hash {
	return dl.root
}

func (dl *diskLayer) Parent() indexLayer {
	return nil
}

func (dl *diskLayer) Stale() bool {
	return dl.stale
}

func (dl *diskLayer) AccountInfo(addr common.Address) (*AccountInfo, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if dl.stale {
		return nil, ErrIndexLayerStale
	}

	accKey := extdb.AccountInfoKey(addr)
	blob, found := dl.cache.HasGet(nil, accKey[:])
	if !found {
		blob, _ := dl.diskdb.Get(accKey)
		dl.cache.Set(accKey[:], blob)
	}
	if len(blob) == 0 {
		return nil, ErrNoAccountInfo
	}
	accInfo := new(AccountInfo)
	if err := rlp.DecodeBytes(blob, accInfo); err != nil {
		panic(err)
	}
	return accInfo, nil
}

func (dl *diskLayer) ContractInfo(addr common.Address) (*ContractInfo, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if dl.stale {
		return nil, ErrIndexLayerStale
	}

	contractKey := extdb.ContractInfoKey(addr)
	blob, found := dl.cache.HasGet(nil, contractKey[:])
	if !found {
		blob, _ := dl.diskdb.Get(contractKey)
		dl.cache.Set(contractKey[:], blob)
	}
	if len(blob) == 0 {
		return nil, ErrNoContractInfo
	}
	contractInfo := new(ContractInfo)
	if err := rlp.DecodeBytes(blob, contractInfo); err != nil {
		panic(err)
	}
	return contractInfo, nil
}

func (dl *diskLayer) AccountStats(addr common.Address) (*AccountStats, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if dl.stale {
		return nil, ErrIndexLayerStale
	}

	statsKey := extdb.AccountStatsKey(addr)
	blob, found := dl.cache.HasGet(nil, statsKey[:])
	if !found {
		blob, _ := dl.diskdb.Get(statsKey)
		dl.cache.Set(statsKey[:], blob)
	}
	if len(blob) == 0 {
		return nil, ErrNoAccountStats
	}
	accStats := new(AccountStats)
	if err := rlp.DecodeBytes(blob, accStats); err != nil {
		panic(err)
	}
	return accStats, nil
}
