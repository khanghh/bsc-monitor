// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	nilAddress = common.Address{}
	nilHash    = common.Hash{}
)

type AccountIndexData struct {
	SentTxs     []common.Hash
	InternalTxs []common.Hash
	TokenTxs    []common.Hash
	Holders     []common.Address
}

type AccountIndexState struct {
	SentTxCount     uint64
	InternalTxCount uint64
	TokenTxCount    uint64
	HolderCount     uint64
}

type accountIndex struct {
	IndexState *AccountIndexState
	ChangeSet  *AccountIndexData
}

func (c *accountIndex) AddSentTx(tx common.Hash) {
	c.ChangeSet.SentTxs = append(c.ChangeSet.SentTxs, tx)
	c.IndexState.SentTxCount += 1
}

func (c *accountIndex) AddInternalTx(tx common.Hash) {
	c.ChangeSet.InternalTxs = append(c.ChangeSet.InternalTxs, tx)
	c.IndexState.InternalTxCount += 1
}

func (c *accountIndex) AddTokenTx(tx common.Hash) {
	c.ChangeSet.TokenTxs = append(c.ChangeSet.TokenTxs, tx)
	c.IndexState.TokenTxCount += 1
}

func (c *accountIndex) AddHolder(addr common.Address) {
	c.ChangeSet.Holders = append(c.ChangeSet.Holders, addr)
	c.IndexState.HolderCount += 1
}

// AccountInfo holds basic information of an account
type AccountInfo struct {
	Name    string
	Tags    []string
	FirstTx common.Hash
}

// ContractInfo is additional data for an account, holds neccessary information about a contract
type ContractInfo struct {
	Type    []string
	ABI     []byte
	Creator common.Address
	Website string
}

type AccountDetail struct {
	Address common.Address
	*AccountInfo
	*ContractInfo
}
