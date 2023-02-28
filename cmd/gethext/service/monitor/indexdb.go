//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
)

const (
	maxInfoCacheSize  = 1000
	maxStateCacheSize = 1000
	purgeInterval     = 10 * time.Minute
)

// IndexDB store additional indexing state along side state trie node for indexing purpose,
// also take care of caching things
type IndexDB struct {
	diskdb     ethdb.Database
	trieCache  state.Database
	infoCache  *lru.Cache
	stateCache *lru.Cache // account indexing state cache
	mtx        sync.Mutex
}

func (db *IndexDB) AccountInfo(addr common.Address) (*AccountInfo, error) {
	cacheKey := extdb.AccountInfoKey(addr)
	var enc []byte
	if cached, exist := db.infoCache.Get(cacheKey); exist {
		enc = cached.([]byte)
	} else {
		enc = extdb.ReadAccountInfo(db.diskdb, addr)
		db.infoCache.Add(cacheKey, enc)
	}
	accInfo := new(AccountInfo)
	if err := rlp.DecodeBytes(enc, &accInfo); err != nil {
		return nil, &NoAccountInfoError{addr}
	}
	return accInfo, nil
}

func (db *IndexDB) ContractInfo(addr common.Address) (*ContractInfo, error) {
	cacheKey := extdb.AccountInfoKey(addr)
	var enc []byte
	if cached, exist := db.infoCache.Get(cacheKey); exist {
		enc = cached.([]byte)
	} else {
		enc = extdb.ReadAccountInfo(db.diskdb, addr)
		db.infoCache.Add(cacheKey, enc)
	}
	contractInfo := new(ContractInfo)
	if err := rlp.DecodeBytes(enc, &contractInfo); err != nil {
		return nil, &NoContractInfoError{addr}
	}
	return contractInfo, nil
}

func (db *IndexDB) getAccountStateRLP(root common.Hash, addr common.Address) ([]byte, error) {
	tr, err := db.trieCache.OpenTrie(root)
	if err != nil {
		return nil, &MissingTrieError{root}
	}
	enc, err := tr.TryGet(addr.Bytes())
	if err != nil && len(enc) > 0 {
		return nil, &NoAccountStateError{root, addr}
	}
	return enc, nil
}

func (db *IndexDB) getIndexStateHash(root common.Hash, addr common.Address) (common.Hash, error) {
	enc, err := db.getAccountStateRLP(root, addr)
	if err != nil {
		return nilHash, err
	}
	return crypto.Keccak256Hash(addr.Bytes(), enc), nil
}

func (db *IndexDB) AccountExtState(root common.Hash, addr common.Address) (*AccountIndexState, error) {
	cacheKey := crypto.Keccak256Hash(root.Bytes(), addr.Bytes())
	var stateEnc []byte
	if cached, exist := db.stateCache.Get(cacheKey); exist {
		stateEnc = cached.([]byte)
	} else {
		enc, err := db.getAccountStateRLP(root, addr)
		if err != nil {
			return nil, err
		}
		hash := crypto.Keccak256Hash(addr.Bytes(), enc)
		stateEnc = extdb.ReadAccountExtState(db.diskdb, hash)
	}
	indexState := new(AccountIndexState)
	if err := rlp.DecodeBytes(stateEnc, indexState); err != nil {
		return nil, &NoContractInfoError{addr}
	}
	return indexState, nil
}

func (db *IndexDB) PurgeCache() {
	if db.infoCache != nil {
		db.infoCache.Purge()
	}
	if db.stateCache != nil {
		db.stateCache.Purge()
	}
}

func (db *IndexDB) OpenTrie(root common.Hash) (state.Trie, error) {
	return db.trieCache.OpenTrie(root)
}

func NewIndexDB(diskdb ethdb.Database, trieCache state.Database) *IndexDB {
	infoCache, _ := lru.New(maxInfoCacheSize)
	stateCache, _ := lru.New(maxStateCacheSize)
	db := &IndexDB{
		diskdb:     diskdb,
		trieCache:  trieCache,
		infoCache:  infoCache,
		stateCache: stateCache,
	}
	return db
}
