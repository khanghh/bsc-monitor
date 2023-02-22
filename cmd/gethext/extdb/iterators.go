//
// Created on 2023/2/22 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"bytes"
	"encoding/binary"

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
