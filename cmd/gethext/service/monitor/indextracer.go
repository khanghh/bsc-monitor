package monitor

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type StateAccessor struct {
	database   state.Database
	blockchain *core.BlockChain
}

func NewStateAccessor(db state.Database, bc *core.BlockChain) (*StateAccessor, error) {
	return &StateAccessor{
		database:   db,
		blockchain: bc,
	}, nil
}

func (idx *StateAccessor) StateAtBlock(block *types.Block, statedb *state.StateDB) (*state.StateDB, error) {
	processor := idx.blockchain.Processor()
	// *state.StateDB, types.Receipts, []*types.Log, uint64, error
	statedb, _, _, _, err := processor.Process(block, statedb, vm.Config{})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (idx *StateAccessor) applyTransaction(statedb *state.StateDB, gp *core.GasPool, tx *types.Transaction, vmConfig *vm.Config) (*state.StateDB, *types.Receipt, error) {
	return nil, nil, nil
}
