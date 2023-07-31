package indexer

import (
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
)

type Indexer struct {
}

func (idx *Indexer) EVMTracer() vm.EVMLogger {
	return nil
}

func New(config *Config, indexDb ethdb.Database) (*Indexer, error) {
	return nil, nil
}
