//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type ExtState interface {
	MarshalRLP() []byte
	UnmarshalRLP(data []byte) error
}

type IndexDB struct {
	diskdb ethdb.Database
	triedb *trie.Database // account trie database
	mtx    sync.Mutex
}

func (db *IndexDB) getAccountStateRLP(root common.Hash, addr common.Address) ([]byte, error) {
	tr, err := trie.New(root, db.triedb)
	if err != nil {
		return nil, &MissingTrieError{root}
	}
	enc, err := tr.TryGet(addr.Bytes())
	if err != nil && len(enc) > 0 {
		return nil, &NoAccountStateError{root, addr}
	}
	return enc, nil
}

func (db *IndexDB) AccountInfo(addr common.Address) (*AccountInfo, error) {
	enc := extdb.ReadAccountInfo(db.diskdb, addr)
	accInfo := new(AccountInfo)
	if err := rlp.DecodeBytes(enc, &accInfo); err != nil {
		log.Error("Could not decode account info", "addr", addr, "err", err)
		return nil, &NoAccountInfoError{addr}
	}
	return accInfo, nil
}

func (db *IndexDB) ContractInfo(addr common.Address) (*ContractInfo, error) {
	enc := extdb.ReadAccountInfo(db.diskdb, addr)
	contractInfo := new(ContractInfo)
	if err := rlp.DecodeBytes(enc, &contractInfo); err != nil {
		log.Error("Could not decode contract info", "addr", addr, "err", err)
		return nil, &NoAccountInfoError{addr}
	}
	return contractInfo, nil
}

func (db *IndexDB) AccountExtState(root common.Hash, addr common.Address, val ExtState) error {
	enc, err := db.getAccountStateRLP(root, addr)
	if err != nil {
		return err
	}
	exthash := crypto.Keccak256Hash(addr.Bytes(), enc)
	extData := extdb.ReadAccountExtState(db.diskdb, exthash)
	return val.UnmarshalRLP(extData)
}

func (db *IndexDB) UpdateExtState(root common.Hash, states map[common.Address]ExtState) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	batch := db.diskdb.NewBatch()
	tr, err := trie.New(root, db.triedb)
	if err != nil {
		return err
	}
	for addr, state := range states {
		enc, err := tr.TryGet(addr.Bytes())
		if err != nil {
			return &NoAccountStateError{root, addr}
		}
		hash := crypto.Keccak256Hash(addr.Bytes(), enc)
		extdb.WriteAccountExtState(batch, hash, state.MarshalRLP())
	}
	if err = batch.Write(); err != nil {
		log.Error("Failed to batch write account state", "err", err)
		return err
	}
	return nil
}

func NewIndexDB(diskdb ethdb.Database, triedb *trie.Database) *IndexDB {
	return &IndexDB{
		diskdb: diskdb,
		triedb: triedb,
	}
}
