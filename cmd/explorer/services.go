package main

import (
	"github.com/ethereum/go-ethereum/cmd/explorer/leth"
	"github.com/ethereum/go-ethereum/cmd/explorer/service"
)

const (
	lethDbNamespace  = "db/leth/chaindata" // namespace of leth database used for metrics collection
	lethChainDataDir = "chaindata"         // leth chain data directory
)

func registerLightEthereum(stack *service.ServiceStack, lethConfig *leth.Config) (*leth.LightEthereum, error) {
	// TODO(khanghh): register LightEthereum lifecycle
	return nil, nil
}
