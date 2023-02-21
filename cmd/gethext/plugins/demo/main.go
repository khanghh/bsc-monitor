package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

type DemoConfig struct {
	Field1 string
	Field2 int
}

type demoPlugin struct {
	config DemoConfig
}

func (p *demoPlugin) execute(ctx *plugin.PluginCtx) {
	runTimer := time.After(5 * time.Second)
	countDown := 5
	for {
		fmt.Printf("Run afeter %d seconds\n", countDown)
		select {
		case <-time.After(1 * time.Second):
			countDown -= 1
		case <-runTimer:
			start := time.Now()
			block, err := ctx.Eth.BlockByNumber(context.Background(), 7_099_870)
			if err != nil {
				log.Error("BlockByNumber", "error", err)
			}
			_, err = ctx.Eth.StateAtBlock(context.Background(), block, 100000, nil, false, true)
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

func (p *demoPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	cfg := DemoConfig{}
	if err := ctx.LoadConfig(&cfg); err != nil {
		return err
	}

	snap := ctx.Eth.Chain().Snapshots().Snapshot(ctx.Eth.CurrentBlock().Root())
	hasher := crypto.NewKeccakState()
	acc := common.HexToAddress("0x8894E0a0c962CB723c1976a4421c95949bE2D4E3")
	accHash := crypto.HashData(hasher, acc.Bytes())
	accData, err := snap.AccountRLP(accHash)
	if err != nil {
		ctx.Log.Error("Could not read account data", "error", err)
	}
	fmt.Println("accData size:", len(accData))
	// api := ethapi.NewPublicBlockChainAPI(ctx.Eth)
	// fmt.Println(cfg)

	// ctx.Log.Info("Demo plugin enabled!")
	// task, err := ctx.ReExec.RunTask(reexec.ReExecOptions{
	// 	StartBlock: 5000000,
	// 	EndBlock:   5100000,
	// 	Processors: []reexec.Processor{p},
	// })
	// if err != nil {
	// 	ctx.Log.Error("failed to run task", "error", err)
	// 	return nil
	// }
	// task.Wait()
	return nil
}

func (p *demoPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &demoPlugin{}
}
