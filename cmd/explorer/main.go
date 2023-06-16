package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/cmd/explorer/leth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
	// The app that holds all commands and flags.
	app          *cli.App
	whenShutdown sync.Once
)

func init() {
	app = cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Usage = "Ethereum blockchain monitor service"
	app.Version = fmt.Sprintf("%s - %s ", gitCommit, gitDate)
	app.Flags = []cli.Flag{
		rpcUrlFlag,
		genesisFlag,
		dataDirFlag,
		verbosityFlag,
	}
	app.Action = run
}

func initLogger(verbosity int) {
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
	glogger.Verbosity(log.Lvl(verbosity))
	log.Root().SetHandler(glogger)
}

// loadGenesis will load the given JSON format genesis file
func loadGenesis(genesisPath string) *core.Genesis {
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis file")
	}
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}
	return genesis
}

func run(ctx *cli.Context) error {
	initLogger(ctx.Int(verbosityFlag.Name))
	rpcUrl := ctx.String(rpcUrlFlag.Name)
	dataDir := ctx.String(dataDirFlag.Name)
	genesis := loadGenesis(ctx.String(genesisFlag.Name))
	lethConfig := &leth.Config{
		RPCUrl:          rpcUrl,
		Genesis:         genesis,
		DataDir:         dataDir,
		DatabaseCache:   512,
		DatabaseHandles: utils.MakeDatabaseHandles(),
		RPCGasCap:       50000000,
		RPCEVMTimeout:   5 * time.Second,
	}
	leth, err := leth.NewLightEthereum(lethConfig)
	if err != nil {
		return err
	}
	if err := leth.Start(); err != nil {
		leth.Stop()
		return err
	}
	// stop monitoring if receive interupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("Got interrupt, shutting down...")
	leth.Stop()
	return nil
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
