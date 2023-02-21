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

const (
	hexFormat = "%06x"
)

var (
	StateRootKey      = []byte("StateRoot")      // SnapshotRootKey tracks the hash of the last snapshot.
	LastIndexBlockKey = []byte("LastIndexBlock") // LastIndexBlock tracks the hash of the last indexed block.
	TotalAccountsKey  = []byte("TotalAccounts")
	TotalContractsKey = []byte("TotalContracts")

	AccountInfoPrefix  = []byte("a") // AccountInfoPrefix + address -> account info
	AccountStatePrefix = []byte("s") // AccountStatePrefix + hash(StateAccount) -> account ext state
	SentTxPrefix       = []byte("t") // SentTxPrefix + address + nonce => transaction hash
	ReceivedTxPrefix   = []byte("r") // ReceiveTxPrefix + address + index => transaction hash
	TokenTxPrefix      = []byte("x") // TokenTxPrefix + address + index => transaction hash
)

func accountInfoKey(addr common.Address) []byte {
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	return append(AccountInfoPrefix, addrHash.Bytes()...)
}

func accountStateKey(root common.Hash) []byte {
	return append(AccountStatePrefix, root.Bytes()...)
}

func tableElementKey(prefix []byte, addr common.Address, index uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, prefix)
	binary.Write(buf, binary.BigEndian, addr.Bytes())
	binary.Write(buf, binary.BigEndian, index)
	return buf.Bytes()
}

func sentTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(SentTxPrefix, addr, nonce)
}

func receivedTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(ReceivedTxPrefix, addr, nonce)
}

func tokenTxKey(addr common.Address, nonce uint64) []byte {
	return tableElementKey(TokenTxPrefix, addr, nonce)
}
