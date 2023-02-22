//
// Created on 2023/2/22 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
)

type TxTableIterator struct {
	indexdb *IndexDB
	it      *extdb.TableItemIterator
}

func (it *TxTableIterator) Next() bool {
	return it.it.Next()
}

func (it *TxTableIterator) Prev() bool {
	return it.it.Prev()
}

func (it *TxTableIterator) Value() common.Hash {
	return common.HexToHash(string(it.it.Value()))
}

func (it *TxTableIterator) Error() error {
	return it.it.Error()
}

func newTxTableIterator(db *IndexDB, prefix []byte, index uint64) *TxTableIterator {
	return &TxTableIterator{
		indexdb: db,
		it:      extdb.NewTableItemIterator(db.diskdb, prefix, index),
	}
}

func NewSentTxIterator(db *IndexDB, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(extdb.AccountSentTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}

func NewReceivedTxIterator(db *IndexDB, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(extdb.AccountReceivedTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}

func NewTokenTxIterator(db *IndexDB, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(extdb.AccountTokenTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}
