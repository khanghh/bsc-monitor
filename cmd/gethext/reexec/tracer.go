//
// Created on 2023/3/13 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package reexec

import (
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type CallFrame struct {
	Type    vm.OpCode      `json:"type"`
	From    common.Address `json:"from"`
	To      common.Address `json:"to,omitempty"`
	Value   *big.Int       `json:"value,omitempty"`
	Gas     uint64         `json:"gas"`
	GasUsed uint64         `json:"gasUsed"`
	Input   []byte         `json:"input"`
	Output  []byte         `json:"output,omitempty"`
	Error   error          `json:"error,omitempty"`
	Calls   []CallFrame    `json:"calls,omitempty"`
}

type callTracer struct {
	env       *vm.EVM
	callstack []CallFrame
	opts      *callTracerOptions
	interrupt uint32 // Atomic flag to signal execution interruption
	reason    error  // Textual reason for the interruption
}

type callTracerOptions struct {
	OnlyTopCall bool `json:"onlyTopCall"` // If true, call tracer won't collect any subcalls
}

// newCallTracer returns a native go tracer which tracks
// call frames of a tx, and implements vm.EVMLogger.
func newCallTracer(opts *callTracerOptions) *callTracer {
	if opts == nil {
		opts = &callTracerOptions{}
	}
	return &callTracer{
		opts:      opts,
		callstack: make([]CallFrame, 1),
	}
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (t *callTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	t.callstack = make([]CallFrame, 1)
	t.callstack[0] = CallFrame{
		Type:  vm.CALL,
		From:  from,
		To:    to,
		Input: input,
		Gas:   gas,
		Value: value,
	}
	if create {
		t.callstack[0].Type = vm.CREATE
	}
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *callTracer) CaptureEnd(output []byte, gasUsed uint64, err error) {
	t.callstack[0].GasUsed = gasUsed
	if err != nil {
		t.callstack[0].Error = err
		if err.Error() == "execution reverted" && len(output) > 0 {
			t.callstack[0].Output = output
		}
	} else {
		t.callstack[0].Output = output
	}
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (t *callTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (t *callTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, _ *vm.ScopeContext, depth int, err error) {
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *callTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if t.opts.OnlyTopCall {
		return
	}
	// Skip if tracing was interrupted
	if atomic.LoadUint32(&t.interrupt) > 0 {
		t.env.Cancel()
		return
	}

	call := CallFrame{
		Type:  typ,
		From:  from,
		To:    to,
		Input: common.CopyBytes(input),
		Gas:   gas,
		Value: value,
	}
	t.callstack = append(t.callstack, call)
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (t *callTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	if t.opts.OnlyTopCall {
		return
	}
	size := len(t.callstack)
	if size <= 1 {
		return
	}
	// pop call
	call := t.callstack[size-1]
	t.callstack = t.callstack[:size-1]
	size -= 1

	call.GasUsed = gasUsed
	if err == nil {
		call.Output = output
	} else {
		call.Error = err
		// if call.Type == "CREATE" || call.Type == "CREATE2" {
		// 	call.To = ""
		// }
	}
	t.callstack[size-1].Calls = append(t.callstack[size-1].Calls, call)
}

func (t *callTracer) CaptureTxStart(gasLimit uint64) {
}

func (t *callTracer) CaptureTxEnd(restGas uint64) {
}

// GetResult returns the json-encoded nested list of call traces, and any
// error arising from the encoding or forceful termination (via `Stop`).
func (t *callTracer) GetResult() (json.RawMessage, error) {
	if len(t.callstack) != 1 {
		return nil, errors.New("incorrect number of top-level calls")
	}
	res, err := json.Marshal(t.callstack[0])
	if err != nil {
		return nil, err
	}
	return json.RawMessage(res), t.reason
}

func (t *callTracer) GetCallStack() []CallFrame {
	return t.callstack
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *callTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}

func bytesToHex(s []byte) string {
	return "0x" + common.Bytes2Hex(s)
}

func bigToHex(n *big.Int) string {
	if n == nil {
		return ""
	}
	return "0x" + n.Text(16)
}

func uintToHex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func addrToHex(a common.Address) string {
	return strings.ToLower(a.Hex())
}
