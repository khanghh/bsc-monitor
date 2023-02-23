package monitor

import (
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

func (idx *ChainIndexer) StateAtBlock(block *types.Block, statedb *state.StateDB) (*state.StateDB, error) {
	processor := idx.blockchain.Processor()
	// *state.StateDB, types.Receipts, []*types.Log, uint64, error
	statedb, _, _, _, err := processor.Process(block, statedb, vm.Config{})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
