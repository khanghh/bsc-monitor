package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/ethereum/go-ethereum/cmd/explorer/leth"
	"github.com/ethereum/go-ethereum/cmd/explorer/service"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"gopkg.in/urfave/cli.v1"
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
	app.Usage = "Binance Smart Chain explorer service"
	app.Version = fmt.Sprintf("%s - %s ", gitCommit, gitDate)
	app.Flags = []cli.Flag{
		configFileFlag,
		rpcUrlFlag,
		genesisFlag,
		utils.DataDirFlag,
		utils.CacheFlag,
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

// makeAppConfig reads the provide TOML configuration file, if config file is
// not sepcified default config is used.
//
// Returns a sanitized appConfig instance to be used by the application.
func makeAppConfig(ctx *cli.Context) *appConfig {
	configFile := ctx.String(configFileFlag.Name)
	config := appConfig{
		LEth:    leth.DefaultConfig,
		Service: service.DefaultConfig,
		Metrics: metrics.DefaultConfig,
	}
	if configFile != "" {
		if err := loadTOMLConfig(configFile, &config); err != nil {
			utils.Fatalf("Could not load config file %s: %v", configFile, err)
		}
	}
	config.LEth.Genesis = makeGenesis(ctx)

	dataDir := ctx.String(utils.DataDirFlag.Name)
	if dataDir != "" {
		config.Service.DataDir = dataDir
	}

	return &config
}

// runServiceStack initialize http/ws rpcservice and start all lifecycle registered in the stack
func runServiceStack(stack *service.ServiceStack) error {
	if err := stack.Run(); err != nil {
		utils.Fatalf("Error starting protocol stack: %v", err)
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

	stack, err := service.NewServiceStack(&config.Service)
	if err != nil {
		return err
	}

	_, err = registerLightEthereum(stack, &config.LEth)
	if err != nil {
		return err
	}

	return runServiceStack(stack)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
