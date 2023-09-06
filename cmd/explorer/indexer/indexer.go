package indexer

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

type BlockChain interface {
	core.ChainContext
	consensus.ChainReader
	StateCache() state.Database
}

type ChainIndexer struct {
	config  *Config
	indexdb *IndexDB // Persisted database to store the indexed data
	bc      BlockChain

	// block processing variables
	parent common.Hash
	data   *indexData // The index data collected from the current block

	status uint32
	quitCh chan struct{}
}

// PreprocessBlock adds hooks to each transaction execution to extract necessary information of the transaction.
// block represents the block being processed
// state is the current state of the blockchain before block processing
// vmConfig is the EVM configuration to be used execute transactions in block
func (idx *ChainIndexer) PreprocessBlock(block *types.Block, state *state.StateDB, vmConfig *vm.Config) {
	// ignore the empty block
	if block.Transactions().Len() == 0 {
		return
	}
	hook := newTxHook(block)
	idx.data = hook.data
	idx.parent = state.StateIntermediateRoot()
	signer := types.MakeSigner(idx.bc.Config(), block.Number())
	vmConfig.Tracer = reexec.NewCallTracerWithHook(block, signer, state, hook)
}

func (idx *ChainIndexer) PostprocessBlock(block *types.Block, state *state.StateDB, receipt types.Receipts, logs []*types.Log, gasUsed uint64, err error) {
	if block.Root() != state.StateIntermediateRoot() {
		return
	}
	idx.indexdb.update(idx.parent, block.Root(), idx.data)
	log.Debug("Chain indexer processed block", "block", block.NumberU64(), "txns", block.Transactions().Len())
}

func (idx *ChainIndexer) Start() error {
	return nil
}

func (idx *ChainIndexer) Stop() error {
	return nil
}

func New(config *Config, db ethdb.Database, bc BlockChain) (*ChainIndexer, error) {
	indexdb, err := NewIndexDB(db, bc.StateCache().TrieDB(), bc.CurrentHeader().Root, config.DatabaseCache)
	if err != nil {
		return nil, err
	}
	return &ChainIndexer{
		config:  config,
		indexdb: indexdb,
		bc:      bc,
	}, nil
}
