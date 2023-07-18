package reexec

import (
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type Context struct {
	block   *types.Block   // The block that chain replayer is executing
	signer  types.Signer   // Signer used for transaction signature handling
	state   *state.StateDB // State at the point of replaying the block for the current transaction
	results []TxResult     // Results from executing the transactions within the block

	txIndex     int         // Index of the transaction currently being executed within the block
	txCallStack []CallFrame // Call stack illustrating the execution flow of the current transaction
}

// Block returns the current block being executed. If replaying pending transactions, it returns the head block
func (c *Context) Block() *types.Block {
	return c.block
}

func (c *Context) Signer() types.Signer {
	return c.signer
}

// Transaction returns the currently executing transaction and its index in the block.
// If it is a pending transaction, returns -1 for the index
func (c *Context) Transaction() (int, *types.Transaction) {
	return c.txIndex, c.block.Transactions()[c.txIndex]
}

func (c *Context) State() *state.StateDB {
	return c.state
}

func (c *Context) Results() []TxResult {
	return c.results
}
