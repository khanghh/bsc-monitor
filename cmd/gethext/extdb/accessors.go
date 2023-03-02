//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

func ReadStateRoot(db ethdb.KeyValueReader) common.Hash {
	data, _ := db.Get(LastIndexStateKey)
	if len(data) != common.HashLength {
		return nilHash
	}
	return common.BytesToHash(data)
}

func WriteIndexStateRoot(db ethdb.KeyValueWriter, root common.Hash) {
	if err := db.Put(LastIndexStateKey, root[:]); err != nil {
		log.Crit("Failed to store snapshot root", "err", err)
	}
}

func ReadLastIndexBlock(db ethdb.KeyValueReader) common.Hash {
	data, _ := db.Get(LastIndexBlockKey)
	if len(data) != common.HashLength {
		return nilHash
	}
	return common.BytesToHash(data)
}

func WriteLastIndexBlock(db ethdb.KeyValueWriter, blockHash common.Hash) {
	if err := db.Put(LastIndexBlockKey, blockHash[:]); err != nil {
		log.Crit("Failed to write last index block", "err", err)
	}
}

func ReadAccountExtState(db ethdb.KeyValueReader, hash common.Hash) []byte {
	data, _ := db.Get(AccountExtStateKey(hash))
	return data
}

func WriteAccountExtState(db ethdb.KeyValueWriter, hash common.Hash, data []byte) {
	if err := db.Put(AccountExtStateKey(hash), data); err != nil {
		log.Crit("Failed to store account state", "err", err)
	}
}

func ReadAccountInfo(db ethdb.KeyValueReader, addr common.Address) []byte {
	data, _ := db.Get(AccountInfoKey(addr))
	return data
}

func WriteAccountInfo(db ethdb.KeyValueWriter, addr common.Address, entry []byte) {
	if err := db.Put(AccountInfoKey(addr), entry); err != nil {
		log.Crit("Failed to store account snapshot", "err", err)
	}
}

func WriteAccountSentTx(db ethdb.KeyValueWriter, addr common.Address, tx common.Hash, index uint64) {
	if err := db.Put(AccountSentTxKey(addr, index), tx.Bytes()); err != nil {
		log.Crit("Failed to store account snapshot", "err", err)
	}
}

func WriteAccountInternalTx(db ethdb.KeyValueWriter, addr common.Address, tx common.Hash, index uint64) {
	if err := db.Put(AccountInternalTxKey(addr, index), tx.Bytes()); err != nil {
		log.Crit("Failed to store account snapshot", "err", err)
	}
}
