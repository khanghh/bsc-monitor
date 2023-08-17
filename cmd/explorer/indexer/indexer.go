package indexer

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
)

type BlockChain interface {
	core.ChainContext
	consensus.ChainReader
}

type ChainIndexer struct {
	config *Config
	db     ethdb.Database
	bc     BlockChain
}

func (idx *ChainIndexer) StateProcessor() core.Processor {
	return nil
}

func (idx *ChainIndexer) PreprocessBlock(block *types.Block, state *state.StateDB, vmConfig vm.Config) {
}

func (idx *ChainIndexer) PostprocessBlock(state *state.StateDB, receipt types.Receipts, logs []*types.Log, gasUsed uint64, err error) {
}

func (idx *ChainIndexer) Start() error {
	return nil
}

func (idx *ChainIndexer) Stop() error {
	return nil
}

func New(config *Config, indexdb ethdb.Database, bc BlockChain) (*ChainIndexer, error) {
	return &ChainIndexer{
		config: config,
		db:     indexdb,
		bc:     bc,
	}, nil
}
