//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	errAccStateNotFound = errors.New("state not found")
	errAccInfoNotFound  = errors.New("account not found")
)

type SnapshotDB struct {
	diskdb ethdb.Database
	triedb *trie.Database // account trie database
	mtx    sync.Mutex
}

func (s *SnapshotDB) getAccountInfo(addr common.Address) (*AccountInfo, error) {
	enc := ReadAccountInfo(s.diskdb, addr)
	accInfo := new(AccountInfo)
	if err := rlp.DecodeBytes(enc, &accInfo); err != nil {
		log.Error("Could not decode account info", "addr", addr, "err", err)
		return nil, err
	}
	return accInfo, nil
}

func (s *SnapshotDB) getAccountState(tr *trie.Trie, addr common.Address) (*AccountState, error) {
	enc, err := tr.TryGet(addr.Bytes())
	if err != nil {
		return nil, err
	}
	state := new(types.StateAccount)
	if err := rlp.DecodeBytes(enc, &state); err != nil {
		log.Error("Failed to decode account state", "addr", addr, "err", err)
		return nil, err
	}
	hash := crypto.Keccak256Hash(addr.Bytes(), enc)
	data := ReadAccountState(s.diskdb, hash)
	return &AccountState{
		StateAccount: state,
		ExtraData:    data,
	}, nil
}

// GetAccountAt returns account and its state at specific state root
func (s *SnapshotDB) GetAccount(root common.Hash, addr common.Address) (*Account, error) {
	trie, err := trie.New(root, s.triedb)
	if err != nil {
		return nil, err
	}
	accState, err := s.getAccountState(trie, addr)
	if err != nil {
		return nil, errAccStateNotFound
	}
	accInfo, err := s.getAccountInfo(addr)
	if err != nil {
		return nil, errAccInfoNotFound
	}
	return &Account{
		AccountInfo:  *accInfo,
		AccountState: *accState,
	}, nil
}

func (s *SnapshotDB) Update(root common.Hash, states map[common.Address]ExtSateRLP) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	batch := s.diskdb.NewBatch()
	tr, err := trie.New(root, s.triedb)
	if err != nil {
		return err
	}
	for addr, state := range states {
		enc, err := tr.TryGet(addr.Bytes())
		if err != nil {
			return errAccStateNotFound
		}
		hash := crypto.Keccak256Hash(addr.Bytes(), enc)
		WriteAccountState(batch, hash, state.MarshalRLP())
	}
	if err = batch.Write(); err != nil {
		log.Crit("Failed to disable snapshots", "err", err)
		return err
	}
	return nil
}

func NewSnapshotDB(diskdb ethdb.Database, triedb *trie.Database) *SnapshotDB {
	return &SnapshotDB{
		diskdb: diskdb,
		triedb: triedb,
	}
}
