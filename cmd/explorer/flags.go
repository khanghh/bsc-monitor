package main

import (
	"gopkg.in/urfave/cli.v1"
)

const (
	MiscCategory = "MISC"
)

var (
	configFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
	rpcUrlFlag = &cli.StringFlag{
		Name:  "rpcurl",
		Usage: "Ethereum node RPC url, prefer a websocket RPC url",
		Value: "ws://localhost:8546",
	}
	genesisFlag = &cli.StringFlag{
		Name:  "genesis",
		Usage: "Path to the genesis json file. If not specified default mainnet genesis are used",
	}
)
