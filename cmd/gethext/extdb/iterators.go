//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

type StateItemIterator struct {
	diskdb ethdb.Database
	prefix []byte
}

func NewStateItemIterator(db ethdb.Database, prefix []byte, addr common.Address) *StateItemIterator {
	db.NewIterator(prefix, nil)
	return &StateItemIterator{
		diskdb: db,
		prefix: prefix,
	}
}

type SentTxsIterator struct {
	db      *SnapshotDB
	current common.Hash
}

func (it *SentTxsIterator) Next() bool {
	return false
}

func (it *SentTxsIterator) Value() common.Hash {
	return it.current
}

func NewSentTxsIterator(db *SnapshotDB) *SentTxsIterator {
	return &SentTxsIterator{}
}
