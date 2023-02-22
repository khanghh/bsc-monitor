//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

var nilHash = common.Hash{}

type AccountInfo struct {
	Name    string
	Address common.Address
	Tags    []string
	FirstTx common.Hash
}

type ContractInfo struct {
	Type string
	ABI  []byte
}

type AccountChangeSet struct {
	SentTxs     []common.Hash
	ReceivedTxs []common.Hash
	TokenTxs    []common.Hash
	Holders     []common.Address
}

type AccountIndexState struct {
	SentTxCount     uint64
	ReceivedTxCount uint64
	TokenTxCount    uint64
	HolderCount     uint64
}

func (s *AccountIndexState) MarshalRLP() []byte {
	data, _ := rlp.EncodeToBytes(s)
	return data
}

func (s *AccountIndexState) UnmarshalRLP(data []byte) error {
	return rlp.DecodeBytes(data, s)
}
