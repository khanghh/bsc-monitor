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
	LastIndexStateKey = []byte("LastIndexState") // LastIndexState tracks the hash of the last indexed state.
	LastIndexBlockKey = []byte("LastIndexBlock") // LastIndexBlock tracks the hash of the last indexed block.
	TotalAccountsKey  = []byte("TotalAccounts")
	TotalContractsKey = []byte("TotalContracts")

	AccountInfoPrefix       = []byte("a") // AccountInfoPrefix + address -> account info
	ContractInfoPrefix      = []byte("c") // ContractInfoPrefix + address -> contract info
	AccountStatePrefix      = []byte("s") // AccountStatePrefix + hash(StateAccount) -> account ext state
	AccountSentTxPrefix     = []byte("t") // AccountSentTxPrefix + address + nonce -> transaction hash
	AccountInternalTxPrefix = []byte("i") // AccountInternalTxPrefix + address + index -> transaction hash
	AccountTokenTxPrefix    = []byte("x") // AccountTokenTxPrefix + address + index -> transaction hash
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
	return append(ContractInfoPrefix, addrHash.Bytes()...)
}

func AccountExtStateKey(root common.Hash) []byte {
	return append(AccountStatePrefix, root.Bytes()...)
}

func tableElementKey(prefix []byte, addr common.Address, index uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, prefix)
	binary.Write(buf, binary.BigEndian, addr.Bytes())
	binary.Write(buf, binary.BigEndian, index)
	return buf.Bytes()
}

func AccountSentTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(AccountSentTxPrefix, addr, nonce)
}

func AccountInternalTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(AccountInternalTxPrefix, addr, nonce)
}

func AccountTokenTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(AccountTokenTxPrefix, addr, nonce)
}
