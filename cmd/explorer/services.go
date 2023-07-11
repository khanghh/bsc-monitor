package main

import (
	"github.com/ethereum/go-ethereum/cmd/explorer/leth"
	"github.com/ethereum/go-ethereum/cmd/explorer/service"
	"github.com/ethereum/go-ethereum/log"
)

const (
	lethDbNamespace  = "db/leth/chaindata" // namespace of leth database used for metrics collection
	lethChainDataDir = "chaindata"         // leth chain data directory
)

func registerLightEthereum(stack *service.ServiceStack, config *leth.Config) (*leth.LightEthereum, error) {
	chaindb, err := stack.OpenDatabase(lethChainDataDir, config.DatabaseCache, config.DatabaseHandles, lethDbNamespace, false)
	if err != nil {
		log.Error("Could not open database", "dbpath", stack.ResolvePath(lethChainDataDir), "error", err)
		return nil, err
	}
	leth, err := leth.New(config, chaindb)
	if err != nil {
		log.Error("Failed to initialize Light Ethereum backend", "error", err)
	}
	stack.RegisterLifeCycle(leth)
	stack.RegisterAPIs(leth.APIs())
	return leth, nil
}
