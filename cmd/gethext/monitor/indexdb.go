//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
)

const (
	maxAccountCacheSize  = 1000
	maxMetadataCacheSize = 1000
	purgeInterval        = 10 * time.Minute
)

// IndexDB store index data for account, also take care of caching things
type IndexDB struct {
	diskdb     ethdb.Database
	trieCache  state.Database
	accCache   *lru.Cache // caching AccountDetail
	stateCache *lru.Cache // caching AccountIndexState
}

func (db *IndexDB) DiskDB() ethdb.Database {
	return db.diskdb
}

func (db *IndexDB) NewBatch() ethdb.Batch {
	return db.diskdb.NewBatch()
}

func (db *IndexDB) readAccountInfo(addr common.Address) (*AccountInfo, error) {
	if enc := extdb.ReadAccountInfo(db.diskdb, addr); len(enc) > 0 {
		accInfo := new(AccountInfo)
		if err := rlp.DecodeBytes(enc, &accInfo); err != nil {
			return nil, err
		}
		return accInfo, nil
	}
	return nil, ErrNoAccountInfo
}

func (db *IndexDB) readContractInfo(addr common.Address) (*ContractInfo, error) {
	if enc := extdb.ReadContractInfo(db.diskdb, addr); len(enc) > 0 {
		contractInfo := new(ContractInfo)
		if err := rlp.DecodeBytes(enc, &contractInfo); err != nil {
			return nil, err
		}
		return contractInfo, nil
	}
	return nil, ErrNoContractInfo
}

func (db *IndexDB) AccountDetail(addr common.Address) (*AccountDetail, error) {
	if cached, ok := db.accCache.Get(addr); ok {
		return cached.(*AccountDetail), nil
	}
	accInfo, err := db.readAccountInfo(addr)
	if err != nil {
		return nil, err
	}
	contractInfo, err := db.readContractInfo(addr)
	if err != ErrNoContractInfo {
		return nil, err
	}
	detail := &AccountDetail{
		Address:      addr,
		AccountInfo:  accInfo,
		ContractInfo: contractInfo,
	}
	db.cacheAccountDetail(addr, detail)
	return detail, nil
}

func (db *IndexDB) cacheAccountDetail(addr common.Address, detail *AccountDetail) {
	db.accCache.Add(addr, detail)
}

func (db *IndexDB) OpenTrie(root common.Hash) (state.Trie, error) {
	return db.trieCache.OpenTrie(root)
}

func (db *IndexDB) OpenIndexTrie(root common.Hash) (*extdb.TrieExt, error) {
	tr, err := db.trieCache.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return extdb.NewTrieExt(db.diskdb, tr, extdb.AccountIndexStatePrefix), nil
}

func (db *IndexDB) cacheAccountIndexState(hash common.Hash, data *AccountIndexState) {
	db.stateCache.Add(hash, data)
}

func (db *IndexDB) AccountIndexState(hash common.Hash) (*AccountIndexState, error) {
	if cached, ok := db.stateCache.Get(hash); ok {
		return cached.(*AccountIndexState), nil
	}
	if enc := extdb.ReadAccountIndexState(db.diskdb, hash); len(enc) > 0 {
		stats := new(AccountIndexState)
		if err := rlp.DecodeBytes(enc, &stats); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (db *IndexDB) PurgeCache() {
	if db.accCache != nil {
		db.accCache.Purge()
	}
	if db.stateCache != nil {
		db.stateCache.Purge()
	}
}

func NewIndexDB(diskdb ethdb.Database, trieCache state.Database) *IndexDB {
	accCache, _ := lru.New(maxAccountCacheSize)
	metaCache, _ := lru.New(maxMetadataCacheSize)
	return &IndexDB{
		diskdb:     diskdb,
		trieCache:  trieCache,
		accCache:   accCache,
		stateCache: metaCache,
	}
}
