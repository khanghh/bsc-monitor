package monitor

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
)

type monitorHook struct {
	processors []Processor
}

func (h *monitorHook) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	for _, proc := range h.processors {
		proc.OnTxStart(ctx, gasLimit)
	}
}

func (h *monitorHook) OnCallEnter(ctx *reexec.Context, call *reexec.CallFrame) {
	for _, proc := range h.processors {
		proc.OnCallEnter(ctx, call)
	}
}

func (h *monitorHook) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
	for _, proc := range h.processors {
		proc.OnCallExit(ctx, call)
	}
}

func (h *monitorHook) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, resetGas uint64) {
	for _, proc := range h.processors {
		proc.OnTxEnd(ctx, ret, resetGas)
	}
}
