package reexec

import (
	"encoding/json"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

type CallTracerWithHook struct {
	handler *callTracer
	block   *types.Block
	signer  types.Signer
	hook    ReExecHook

	// variables to track current transaction execution
	txIndex  uint64    // index of the current transaction
	txResult *TxResult // context of the current transaction
	ctxStack []CallCtx // call stack of the current transaction
}

func (t *CallTracerWithHook) CaptureTxStart(gasLimit uint64) {
	t.handler.CaptureTxStart(gasLimit)
	tx := t.block.Transactions()[t.txIndex]
	msg, _ := tx.AsMessage(t.signer, t.block.BaseFee())
	t.txResult = &TxResult{
		Block:       t.block,
		Transaction: tx,
		TxIndex:     t.txIndex,
		Message:     &msg,
	}
	t.hook.OnTxStart(t.txResult, gasLimit)
}

func (t *CallTracerWithHook) CaptureTxEnd(restGas uint64) {
	t.handler.CaptureTxEnd(restGas)
	t.txResult.CallStack = t.handler.callstack
	t.hook.OnTxEnd(t.txResult, restGas)
	t.txResult = nil
	t.txIndex += 1
}

func (t *CallTracerWithHook) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.handler.CaptureStart(env, from, to, create, input, gas, value)
	t.ctxStack = make([]CallCtx, 1)
	t.ctxStack[0] = CallCtx{
		txContext: t.txResult,
		callFrame: &t.handler.callstack[0],
	}
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
	callCtx := CallCtx{
		txContext: t.txResult,
		callFrame: &frame,
	}
	size := len(t.ctxStack)
	if size > 0 {
		callCtx.Parent = &t.ctxStack[size-1]
	}
	t.ctxStack = append(t.ctxStack, callCtx)
	t.hook.OnCallEnter(&callCtx)
}

func (t *CallTracerWithHook) CaptureExit(output []byte, gasUsed uint64, err error) {
	t.handler.CaptureExit(output, gasUsed, err)
	size := len(t.ctxStack)
	callCtx := t.ctxStack[size-1]
	t.ctxStack = t.ctxStack[:size-1]
	callCtx.Error = err
	t.hook.OnCallExit(&callCtx)
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

func NewCallTracerWithHook(block *types.Block, signer types.Signer, hook ReExecHook) tracers.Tracer {
	return &CallTracerWithHook{
		block:  block,
		signer: signer,
		handler: &callTracer{
			opts:      &callTracerOptions{},
			callstack: make([]callFrame, 1),
		},
		hook: hook,
	}
}
