//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	LastIndexStateKey = []byte("LastIndexState") // LastIndexState tracks the hash of the last indexed statedb
	LastIndexBlockKey = []byte("LastIndexBlock") // LastIndexBlock tracks the hash of the last indexed block
	TotalAccountsKey  = []byte("TotalAccounts")
	TotalContractsKey = []byte("TotalContracts")

	AccountInfoPrefix       = []byte("a") // AccountInfoPrefix + address -> account info
	ContractABIPrefix       = []byte("c") // ContractInfoPrefix + address -> contract info
	AccountIndexStatePrefix = []byte("s") // AccountIndexStatePrefix + hash(StateAccount) -> account index state
	AccountSentTxPrefix     = []byte("t") // AccountSentTxPrefix + address + block num + index -> transaction hash
	AccountInternalTxPrefix = []byte("i") // AccountInternalTxPrefix + address + block num + index -> transaction hash
	AccountTokenTxPrefix    = []byte("x") // AccountTokenTxPrefix + address + index -> transaction hash
	TokenHolderAddrPrefix   = []byte("h") // TokenHolderPrefix + token address + index -> account address
)

var (
	nilHash = common.Hash{}
)

func AccountInfoKey(addr common.Address) []byte {
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	return append(AccountInfoPrefix, addrHash.Bytes()...)
}

func ContractInfoKey(addr common.Address) []byte {
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	return append(ContractABIPrefix, addrHash.Bytes()...)
}

func AccountIndexStateKey(hash common.Hash) []byte {
	return append(AccountIndexStatePrefix, hash.Bytes()...)
}

func indexItemKey(prefix []byte, addr common.Address, refNum uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, prefix)
	binary.Write(buf, binary.BigEndian, addr.Bytes())
	binary.Write(buf, binary.BigEndian, refNum)
	return buf.Bytes()
}

func IndexItemRef(blockNumber uint64, index uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[:4], uint32(blockNumber))
	binary.BigEndian.PutUint32(buf[4:], uint32(index))
	return buf
}

func IndexItemRefNum(blockNumber uint64, index uint64) uint64 {
	buf := IndexItemRef(blockNumber, index)
	return binary.BigEndian.Uint64(buf)
}

func AccountSentTxKey(addr common.Address, refNum uint64) []byte {
	return indexItemKey(AccountSentTxPrefix, addr, refNum)
}

func AccountInternalTxKey(addr common.Address, refNum uint64) []byte {
	return indexItemKey(AccountInternalTxPrefix, addr, refNum)
}

func AccountTokenTxKey(addr common.Address, refNum uint64) []byte {
	return indexItemKey(AccountTokenTxPrefix, addr, refNum)
}

func TokenHolderAddrKey(tknAddr common.Address, refNum uint64) []byte {
	return indexItemKey(TokenHolderAddrPrefix, tknAddr, refNum)
}
