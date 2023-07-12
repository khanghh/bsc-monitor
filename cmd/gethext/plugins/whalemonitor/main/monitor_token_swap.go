package main

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
)

type TokenSwapMonitor handler

func (p *TokenSwapMonitor) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	log.Warn("TokenSwapMonitor.OnTxStart")
}

func (p *TokenSwapMonitor) OnCallEnter(ctx *reexec.Context, frame *reexec.CallFrame) {
}

func (p *TokenSwapMonitor) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
}

func (p *TokenSwapMonitor) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, resetGas uint64) {
	if ret.Reverted {
		return
	}
}
