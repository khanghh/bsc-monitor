package main

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

// handler serves as a base type for monitor processors.
type handler struct {
	*plugin.PluginCtx
	config *Config
	client *rpc.Client
	feed   event.Feed
}

func (p *handler) SubscribeWhaleEvent(ch chan whalemonitor.WhaleEvent) event.Subscription {
	return p.EventScope.Track(p.feed.Subscribe(ch))
}

func (t *handler) getERC20Info(addr common.Address) (*whalemonitor.ERC20Token, error) {
	client := ethclient.NewClient(t.client)
	erc20, err := contracts.NewERC20(addr, client)
	if err != nil {
		return nil, err
	}
	name, err := erc20.Name(&bind.CallOpts{})
	if err != nil {
		name = "Unknown"
	}
	symbol, err := erc20.Symbol(&bind.CallOpts{})
	if err != nil {
		symbol = "Unknown"
	}
	decimals, err := erc20.Decimals(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	return &whalemonitor.ERC20Token{
		Address:  addr,
		Name:     name,
		Symbol:   symbol,
		Decimals: uint64(decimals),
	}, nil
}
