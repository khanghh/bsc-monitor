package main

import (
	"github.com/ethereum/go-ethereum/cmd/explorer/indexer"
	"github.com/ethereum/go-ethereum/cmd/explorer/leth"
	"github.com/ethereum/go-ethereum/cmd/explorer/service"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
)

const (
	lethDbNamespace    = "db/leth/chaindata"    // namespace of leth database used for metrics collection
	lethChainDataDir   = "chaindata"            // leth chain data directory
	indexerDbNamespace = "db/indexer/indexdata" // namespace of chain indexer for metrics collection
	indexerDataDir     = "indexdata"            // index data directory
)

func newServiceStack(config *service.Config) *service.ServiceStack {
	stack, err := service.NewServiceStack(config)
	if err != nil {
		utils.Fatalf("Could not create service stack. %v", err)
	}
	return stack
}

func makeLightEthereum(stack *service.ServiceStack, config *leth.Config, lcOpts ...leth.LightChainOption) (*leth.LightEthereum, error) {
	// Ensure configuration values are valid
	if err := config.Sanitize(); err != nil {
		return nil, err
	}

	chaindb, err := stack.OpenDatabase(lethChainDataDir, config.DatabaseCache, config.DatabaseHandles, lethDbNamespace, false)
	if err != nil {
		log.Error("Could not open database", "dbpath", stack.ResolvePath(lethChainDataDir), "error", err)
		return nil, err
	}
	leth, err := leth.New(config, chaindb, lcOpts...)
	if err != nil {
		log.Error("Failed to initialize Light Ethereum backend", "error", err)
		return nil, err
	}
	return leth, nil
}

func makeChainIndexer(stack *service.ServiceStack, config *indexer.Config, leth *leth.LightEthereum) (*indexer.ChainIndexer, error) {
	indexdb, err := stack.OpenDatabase(indexerDataDir, config.DatabaseCache, config.DatabaseHandles, indexerDbNamespace, false)
	if err != nil {
		log.Error("Could not open database", "dbpath", stack.ResolvePath(indexerDataDir), "error", err)
		return nil, err
	}
	chain := leth.LightChain()
	indexer, err := indexer.New(config, indexdb, chain)
	if err != nil {
		return nil, err
	}
	chain.SetProcessorHooks(indexer.PreprocessBlock, indexer.PostprocessBlock)
	return indexer, nil
}
