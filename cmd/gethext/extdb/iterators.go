//
// Created on 2023/2/22 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

type TableItemIterator struct {
	diskdb       ethdb.Database
	prefix       []byte
	currentIndex uint64
	currentValue []byte
	err          error
}

func (it *TableItemIterator) resolveValue(index uint64) ([]byte, error) {
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, it.prefix)
	binary.Write(keyBuf, binary.BigEndian, index)
	return it.diskdb.Get(keyBuf.Bytes())
}

func (it *TableItemIterator) Next() bool {
	val, err := it.resolveValue(it.currentIndex + 1)
	it.err = err
	if err != nil {
		return false
	}
	it.currentValue = val
	it.currentIndex += 1
	return true
}

func (it *TableItemIterator) Prev() bool {
	if it.currentIndex == 0 {
		return false
	}
	val, err := it.resolveValue(it.currentIndex - 1)
	it.err = err
	if err != nil {
		return false
	}
	it.currentValue = val
	it.currentIndex -= 1
	return false
}

func (it *TableItemIterator) Error() error {
	return it.err
}

func (it *TableItemIterator) Value() []byte {
	if it.currentValue == nil && it.err == nil {
		it.currentValue, it.err = it.resolveValue(it.currentIndex)
	}
	return it.currentValue
}

func NewTableItemIterator(db ethdb.Database, prefix []byte, index uint64) *TableItemIterator {
	return &TableItemIterator{
		diskdb:       db,
		currentIndex: index,
	}
}

type TxTableIterator struct {
	it *TableItemIterator
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

func newTxTableIterator(db ethdb.Database, prefix []byte, index uint64) *TxTableIterator {
	return &TxTableIterator{
		it: NewTableItemIterator(db, prefix, index),
	}
}

func NewSentTxIterator(db ethdb.Database, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(AccountSentTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}

func NewReceivedTxIterator(db ethdb.Database, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(AccountInternalTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}

func NewTokenTxIterator(db ethdb.Database, addr common.Address, nonce uint64) *TxTableIterator {
	prefix := append(AccountTokenTxPrefix, addr.Bytes()...)
	return newTxTableIterator(db, prefix, nonce)
}
