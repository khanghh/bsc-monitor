package reexec

import (
	"encoding/json"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

type CallTracerWithHook struct {
	*Context
	handler  *callTracer
	hook     TransactionHook
	txResult *TxResult // The execution result of the current transaction
}

func (t *CallTracerWithHook) CaptureTxStart(gasLimit uint64) {
	t.handler.CaptureTxStart(gasLimit)
	t.txCallStack = t.handler.callstack
	t.txResult = &t.results[t.txIndex]
	t.hook.OnTxStart(t.Context, gasLimit)
}

func (t *CallTracerWithHook) CaptureTxEnd(restGas uint64) {
	t.handler.CaptureTxEnd(restGas)
	if int(t.txIndex+1) < t.block.Transactions().Len() {
		t.txIndex += 1
	}
	t.hook.OnTxEnd(t.Context, t.txResult, restGas)
}

func (t *CallTracerWithHook) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.handler.CaptureStart(env, from, to, create, input, gas, value)
}

func (t *CallTracerWithHook) CaptureEnd(output []byte, gasUsed uint64, err error) {
	t.handler.CaptureEnd(output, gasUsed, err)
	if err != nil {
		t.txResult.Reverted = true
	}
}

func (t *CallTracerWithHook) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	t.handler.CaptureEnter(typ, from, to, input, gas, value)
	if atomic.LoadUint32(&t.handler.interrupt) > 0 {
		return
	}
	frame := t.handler.callstack[len(t.handler.callstack)-1]
	t.hook.OnCallEnter(t.Context, &frame)
}

func (t *CallTracerWithHook) CaptureExit(output []byte, gasUsed uint64, err error) {
	frame := t.handler.callstack[len(t.handler.callstack)-1]
	t.handler.CaptureExit(output, gasUsed, err)
	t.hook.OnCallExit(t.Context, &frame)
}

func (t *CallTracerWithHook) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	t.handler.CaptureState(pc, op, gas, cost, scope, rData, depth, err)
}

func (t *CallTracerWithHook) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
	t.handler.CaptureFault(pc, op, gas, cost, scope, depth, err)
}

func (t *CallTracerWithHook) GetResult() (json.RawMessage, error) {
	return t.handler.GetResult()
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *CallTracerWithHook) Stop(err error) {
	t.handler.Stop(err)
}

func NewCallTracerWithHook(block *types.Block, signer types.Signer, state *state.StateDB, hook TransactionHook) tracers.Tracer {
	return &CallTracerWithHook{
		Context: &Context{
			block:   block,
			signer:  signer,
			state:   state,
			results: make([]TxResult, len(block.Transactions())),
		},
		handler: newCallTracer(nil),
		hook:    hook,
	}
}
