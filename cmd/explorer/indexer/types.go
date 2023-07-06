package indexer

import (
	"github.com/ethereum/go-ethereum/core/types"
)

type txContext = TxContext
type callFrame = CallFrame

// TxContext provides execution context for transaction
type TxContext struct {
	Block       *types.Block       // block in which the transaction was included
	Transaction *types.Transaction // the transaction in block
	TxIndex     uint64             // index of the transaction within the block
	Message     *types.Message     // message derived from the transaction
	Reverted    bool               // the state of transaction successful or reverted
	CallStack   []CallFrame
}

// CallCtx provides context information for a function call
type CallCtx struct {
	*txContext
	*callFrame
	Parent *CallCtx // pointer to the parent method context that executes this call
	Error  error
}

// ReExecHook provides a way to hook into the re-execution process
type ReExecHook interface {
	// OnTxStart is called when transaction execution starts
	OnTxStart(ctx *TxContext, gasLimit uint64)

	// OnTxEnd is called when transaction execution ends
	OnTxEnd(ctx *TxContext, resetGas uint64)

	// OnCallEnter is called when execution enters a method
	OnCallEnter(ctx *CallCtx)

	// OnCallExit is called when execution exits from a method
	OnCallExit(ctx *CallCtx)
}
