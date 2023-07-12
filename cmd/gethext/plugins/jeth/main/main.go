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
	mtx      sync.Mutex           // Runners list modification lock
}

func (p *JETH) createNewRunner() *JsRunner {
	runnerId := atomic.AddUint64(&p.runCount, 1)
	runnerName := fmt.Sprintf("jeth%d-%d", runnerId, time.Now().Unix())
	p.runners[runnerId] = newRunner(runnerName, p.registry)
	return p.runners[runnerId]
}

// Execute compiles and executes input JavaScript code
func (p *JETH) Execute(code string) (*JsRunner, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	runner := p.createNewRunner()
	filename := fmt.Sprintf("%s.js", runner.Name)
	_, err := runner.CompileAndRun(filename, code)
	return runner, err
}

// Run load and execute JavaScript code from a file
func (p *JETH) Run(filename string) (*JsRunner, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	filepath := path.Join(p.rootDir, filename)
	code, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	runner := p.createNewRunner()
	_, err = runner.CompileAndRun(filepath, string(code))
	return runner, err
}

// CleanUp removes non-running JavaScript runner from the list and return the removed count
func (p *JETH) CleanUp() int {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	count := 0
	for id, runner := range p.runners {
		if !runner.Running() {
			delete(p.runners, id)
			count++
		}
	}
	return count
}

// Kill stops the specified running JavaScript runner by its id
func (p *JETH) Kill(runnerId uint64) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if runner, ok := p.runners[runnerId]; ok {
		return runner.Stop()
	}
	return nil
}

func (p *JETH) Stop() error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	wg := sync.WaitGroup{}
	for _, runner := range p.runners {
		wg.Add(1)
		go func(runner *JsRunner) {
			defer wg.Done()
			runner.Stop()
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
