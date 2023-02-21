package monitor

import (
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

type ChainIndexer struct {
	diskdb ethdb.Database
	triedb *trie.Database
}

func (idx *ChainIndexer) ProcessState(state *state.StateDB, block *types.Block, txIndex int) error {
	return nil
}

func NewChainIndexer(diskdb ethdb.Database, triedb *trie.Database) *ChainIndexer {
	return &ChainIndexer{
		diskdb: diskdb,
		triedb: triedb,
	}
}
