package main

import (
	"fmt"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/khanghh/goja-nodejs/require"
)

const (
	pluginName = "JETH"
)

var (
	logger = plugin.NewLogger(pluginName)
)

type JETH struct {
	ctx      *plugin.PluginCtx    // JETH plugin context
	rootDir  string               // Root directory where JavaScript files are located
	runCount uint64               // The counter used to uniquely identify a runner instance
	runners  map[uint64]*JsRunner // All javascript runner that have been added
	registry *require.Registry    // Native module registry to register golang module into goja javascript runtime
}

func (p *JETH) createRunner() *JsRunner {
	runnerId := atomic.AddUint64(&p.runCount, 1)
	runnerName := fmt.Sprintf("jeth%d-%d", runnerId, time.Now().Unix())
	return newRunner(runnerName, p.registry)
}

// Run execute JavaScript code
func (p *JETH) Execute(code string) (*JsRunner, error) {
	return nil, nil
}

// Run load and execute JavaScript code from file
func (p *JETH) Run(filename string) (*JsRunner, error) {
	filepath := path.Join(p.rootDir, filename)
	code, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	runner := p.createRunner()
	runner.CompileAndRun(filepath, string(code))
	return runner, nil
}

func (p *JETH) Stop() error {
	wg := sync.WaitGroup{}
	for _, runner := range p.runners {
		wg.Add(1)
		go func(runner *JsRunner) {
			defer wg.Done()
			if err := runner.Stop(); err != nil {
				logger.Error("Runner stopped with error", "error", err)
			}
		}(runner)
	}
	wg.Wait()
	return nil
}

func (p *JETH) OnEnable(ctx *plugin.PluginCtx) error {
	if err := makeDirIfNotExist(p.rootDir); err != nil {
		logger.Error("Could not create JETH root directory", err)
		return err
	}
	p.registry = require.NewRegistryWithLoader(rootDirFileLoader(p.rootDir))
	return nil
}

func (p *JETH) OnDisable(ctx *plugin.PluginCtx) error {
	return p.Stop()
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &JETH{
		ctx:     ctx,
		runners: make(map[uint64]*JsRunner),
		rootDir: ctx.DataDir,
	}
}
