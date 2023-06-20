package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"
)

//go:embed genesis.json
var bscMainnetGenesis string

// makeGenesis loads the specified genesis file or returns the default genesis config.
func makeGenesis(ctx *cli.Context) *core.Genesis {
	genesisPath := ctx.String(genesisFlag.Name)
	var reader io.Reader

	if genesisPath == "" {
		log.Info("Use default BSC mainnet genesis config")
		reader = strings.NewReader(bscMainnetGenesis)
	} else {
		file, err := os.Open(genesisPath)
		if err != nil {
			utils.Fatalf("Failed to read genesis file: %v", err)
		}
		defer file.Close()
		log.Info("Use custom genesis file", "genesis", genesisPath)
		reader = file
	}

	genesis := new(core.Genesis)
	if err := json.NewDecoder(reader).Decode(genesis); err != nil {
		utils.Fatalf("Invalid genesis file: %v", err)
	}
	return genesis
}
