//
// Created on 2023/3/15 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package reexec

import (
	"github.com/ethereum/go-ethereum/common"
)

type TxCallCtx struct {
	TxIndex        uint64
	TxHash         common.Hash
	BlockHash      common.Hash
	TxGasLimit     uint64 // GasLimit of the current transaction
	BlockGasLimit  uint64 // GasRemain is the remaining gas of the block
	BlockGasRemain uint64 // GasRemain is the remaining gas of the block
}

type CallCtx struct {
	Transaction *TxCallCtx
	GasLimit    uint64
	GasRemain   uint64
	CallStack   []CallFrame
	From        common.Address
	To          common.Address
	Method      [4]byte
}

type ReExecHook interface {
	OnTxStart(ctx *TxCallCtx, gasLimit uint64)
	OnTxEnd(ctx *TxCallCtx, resetGas uint64)
	OnCallEnter(ctx *CallCtx)
	OnCallExit(ctx *CallCtx)
}
