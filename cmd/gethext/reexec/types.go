package reexec

// TxResult provides execution context for transaction
type TxResult struct {
	TxIndex   uint64 // index of the transaction within the block
	Reverted  bool   // the state of transaction successful or reverted
	CallStack []CallFrame
}

// TransactionHook provides a way to hook into the transaction execution process
type TransactionHook interface {
	// OnTxStart is called when transaction execution starts
	OnTxStart(ctx *Context, gasLimit uint64)

	// OnCallEnter is called when execution enters a contract method
	OnCallEnter(ctx *Context, call *CallFrame)

	// OnCallExit is called when execution exits from a contract method
	OnCallExit(ctx *Context, call *CallFrame)

	// OnTxEnd is called when transaction execution ends
	OnTxEnd(ctx *Context, ret *TxResult, restGas uint64)
}
