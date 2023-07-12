package main

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
)

type TokenTransferMonitor handler

func (p *TokenTransferMonitor) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	log.Warn("TokenTransferMonitor.OnTxStart")
}

func (p *TokenTransferMonitor) OnCallEnter(ctx *reexec.Context, frame *reexec.CallFrame) {
}

func (p *TokenTransferMonitor) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
}

func (p *TokenTransferMonitor) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, resetGas uint64) {
}
