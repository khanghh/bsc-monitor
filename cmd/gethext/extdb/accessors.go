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

type RLPMarshaler interface {
	MarshalJSON() ([]byte, error)
}

type RLPUnmarshaler interface {
	UnmarshalRLP([]byte) error
}

func ReadLastIndexRoot(db ethdb.KeyValueReader) common.Hash {
	data, _ := db.Get(LastIndexStateKey)
	if len(data) != common.HashLength {
		return nilHash
	}
	return common.BytesToHash(data)
}

func WriteLastIndexRoot(db ethdb.KeyValueWriter, root common.Hash) {
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

func ReadAccountInfo(db ethdb.KeyValueReader, addr common.Address) []byte {
	data, _ := db.Get(AccountInfoKey(addr))
	return data
}

func WriteAccountInfo(db ethdb.KeyValueWriter, addr common.Address, entry []byte) {
	if err := db.Put(AccountInfoKey(addr), entry); err != nil {
		log.Crit("Failed to write account info", "err", err)
	}
}

func ReadContractInfo(db ethdb.KeyValueReader, addr common.Address) []byte {
	data, _ := db.Get(ContractInfoKey(addr))
	return data
}

func WriteContractInfo(db ethdb.KeyValueWriter, addr common.Address, entry []byte) {
	if err := db.Put(ContractInfoKey(addr), entry); err != nil {
		log.Crit("Failed to write contract info", "err", err)
	}
}

func ReadAccountIndexState(db ethdb.KeyValueReader, hash common.Hash) []byte {
	data, _ := db.Get(AccountIndexStateKey(hash))
	return data
}

func WriteAccountIndexState(db ethdb.KeyValueWriter, hash common.Hash, entry []byte) {
	if err := db.Put(AccountIndexStateKey(hash), entry); err != nil {
		log.Crit("Failed to write account index state", "err", err)
	}
}

func WriteAccountSentTx(db ethdb.KeyValueWriter, addr common.Address, ref uint64, tx common.Hash) {
	if err := db.Put(AccountSentTxKey(addr, ref), tx.Bytes()); err != nil {
		log.Crit("Failed to write account sent transaction", "err", err)
	}
}

func WriteAccountInternalTx(db ethdb.KeyValueWriter, addr common.Address, ref uint64, tx common.Hash) {
	if err := db.Put(AccountInternalTxKey(addr, ref), tx.Bytes()); err != nil {
		log.Crit("Failed to write internal transaction", "err", err)
	}
}

func WriteAccountTokenTx(db ethdb.KeyValueWriter, addr common.Address, ref uint64, tx common.Hash) {
	if err := db.Put(AccountInternalTxKey(addr, ref), tx.Bytes()); err != nil {
		log.Crit("Failed to write token transaction", "err", err)
	}
}

func WriteTokenHolderAddr(db ethdb.KeyValueWriter, tknAddr common.Address, ref uint64, holderAddr common.Address) {
	if err := db.Put(TokenHolderAddrKey(tknAddr, ref), holderAddr.Bytes()); err != nil {
		log.Crit("Failed to write token holder address", "err", err)
	}
}

func ReadInterfaceList(db ethdb.KeyValueReader) []byte {
	data, _ := db.Get(InterfaceListKey)
	return data
}

func WriteInterfaceList(db ethdb.KeyValueWriter, data []byte) {
	if err := db.Put(InterfaceListKey, data); err != nil {
		log.Crit("Failed to write contract interface list", "err", err)
	}
}

func ReadFourBytesABIs(db ethdb.KeyValueReader, fourBytes []byte) []byte {
	data, _ := db.Get(FourBytesABIsKey(fourBytes))
	return data
}

func WriteFourBytesABIs(db ethdb.KeyValueWriter, fourBytes []byte, data []byte) {
	if err := db.Put(FourBytesABIsKey(fourBytes), data); err != nil {
		log.Crit("Failed to write 4-byes method abis", "err", err)
	}
}
