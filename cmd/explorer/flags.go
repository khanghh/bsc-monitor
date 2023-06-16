package main

import (
	"github.com/urfave/cli/v2"
)

const (
	MiscCategory = "MISC"
)

var (
	rpcUrlFlag = &cli.StringFlag{
		Name:  "rpcurl",
		Usage: "Ethereum node RPC url",
		Value: "ws://localhost:8546",
	}
	genesisFlag = &cli.StringFlag{
		Name:  "genesis",
		Usage: "Genesis file",
		Value: "genesis.json",
	}
	dataDirFlag = &cli.StringFlag{
		Name:  "datadir",
		Usage: "chain data directory",
		Value: "chaindata",
	}
	verbosityFlag = &cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		Value: 3,
	}
)

func init() {
	cli.HelpFlag.(*cli.BoolFlag).Category = MiscCategory
	cli.VersionFlag.(*cli.BoolFlag).Category = MiscCategory
}
