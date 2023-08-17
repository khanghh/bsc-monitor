package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ethereum/go-ethereum/cmd/explorer/service"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
	// The app that holds all commands and flags.
	app *cli.App
)

func init() {
	app = cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Usage = "Binance Smart Chain explorer service"
	app.Version = fmt.Sprintf("%s - %s ", gitCommit, gitDate)
	app.Flags = []cli.Flag{
		configFileFlag,
		rpcUrlFlag,
		genesisFlag,
		utils.DataDirFlag,
	}
	app.Flags = append(app.Flags, debug.Flags...)

	app.Action = run
	app.Before = func(ctx *cli.Context) error {
		return debug.Setup(ctx)
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		prompt.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

// runServiceStack initialize http/ws rpcservice and start all lifecycle registered in the stack
func runServiceStack(stack *service.ServiceStack) error {
	if err := stack.Run(); err != nil {
		utils.Fatalf("Error starting service stack: %v", err)
	}
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		shutdown := func() {
			log.Info("Got interrupt, shutting down...")
			go stack.Stop()
			for i := 10; i > 0; i-- {
				<-sigCh
				if i > 1 {
					log.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
				}
			}
			debug.Exit() // ensure trace and CPU profile data is flushed.
			debug.LoudPanic("boom")
		}

		<-sigCh
		shutdown()
	}()
	return stack.Wait()
}

func run(ctx *cli.Context) error {
	config := makeAppConfig(ctx)
	applyMetricConfig(ctx, &config.Metrics)

	stack, err := service.NewServiceStack(&config.Service)
	if err != nil {
		return err
	}

	leth, err := makeLightEthereum(stack, &config.LEth)
	if err != nil {
		return err
	}

	indexer, err := makeChainIndexer(stack, &config.Indexer, leth)
	if err != nil {
		return err
	}

	stack.RegisterLifeCycle(leth)
	stack.RegisterAPIs(leth.APIs())
	stack.RegisterLifeCycle(indexer)
	return runServiceStack(stack)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
