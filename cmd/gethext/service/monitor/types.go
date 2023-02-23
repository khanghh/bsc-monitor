// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/common"
)

var nilHash = common.Hash{}

type AccountChangeSet struct {
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
