package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/reexec"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type demoPlugin struct {
	ctx *plugin.PluginCtx
}

func (p *demoPlugin) execute() {
	runTimer := time.After(5 * time.Second)
	countDown := 5
	for {
		fmt.Printf("Run afeter %d seconds\n", countDown)
		select {
		case <-time.After(1 * time.Second):
			countDown -= 1
		case <-runTimer:
			start := time.Now()
			block, err := p.ctx.Eth.BlockByNumber(context.Background(), 7_099_870)
			if err != nil {
				log.Error("BlockByNumber", "error", err)
			}
			_, err = p.ctx.Eth.StateAtBlock(context.Background(), block, 100000, nil, false, true)
			if err != nil {
				log.Error("StateAndHeaderByNumber", "error", err)
			}
			elapsed := time.Since(start)
			log.Info("StateAndHeaderByNumber", "elapse", elapsed)
			return
		}
	}
}

func (p *demoPlugin) ProcessState(state *state.StateDB, block *types.Block, txIndex int) error {
	fmt.Println("block", block.NumberU64())
	fmt.Println("txIndex", txIndex)
	fmt.Println("stateHash", state.StateIntermediateRoot())
	return nil
}

func (p *demoPlugin) OnEnable() error {
	p.ctx.Log.Info("Demo plugin enabled!")
	task, err := p.ctx.ReExec.RunTask(reexec.ReExecOptions{
		StartBlock: 5000000,
		EndBlock:   5100000,
		Processors: []reexec.Processor{p},
	})
	if err != nil {
		p.ctx.Log.Error("failed to run task", "error", err)
		return nil
	}
	task.Wait()
	return nil
}

func (p *demoPlugin) OnDisable() error {
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &demoPlugin{ctx}
}
