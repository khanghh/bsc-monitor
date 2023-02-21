//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type ExtSateRLP interface {
	MarshalRLP() []byte
	UnmarshalRLP(data []byte) error
}

type AccountInfo struct {
	Name    string
	Address common.Address
	Tags    []string
	ABI     []byte
	FirstTx common.Hash
}

type AccountState struct {
	*types.StateAccount
	ExtraData []byte
}

func (as *AccountState) Hash() common.Hash {
	enc, err := rlp.EncodeToBytes(as)
	if err != nil {
		return nilHash
	}
	return crypto.Keccak256Hash(enc)
}

type Account struct {
	AccountInfo
	AccountState
}
