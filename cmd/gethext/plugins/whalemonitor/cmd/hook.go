package main

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
)

func (p *WhaleMonitorPlugin) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	p.log.Warn("OnTxStart")
}

func (p *WhaleMonitorPlugin) OnCallEnter(ctx *reexec.Context, frame *reexec.CallFrame) {
	_, tx := ctx.Transaction()
	p.log.Warn("OnCallEnter", "tx", tx.Hash().Hex())
}

func (p *WhaleMonitorPlugin) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
}

func (p *WhaleMonitorPlugin) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, resetGas uint64) {
}
