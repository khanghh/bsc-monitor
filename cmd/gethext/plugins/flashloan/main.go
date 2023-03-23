package main

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

type DemoConfig struct {
	Field1 string
	Field2 int
}

type demoPlugin struct {
	config DemoConfig
}

type callHook struct {
}

func (h *callHook) OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {
	log.Warn(fmt.Sprintf("OnTxStart %#x, index: %d", ctx.Transaction.Hash(), ctx.TxIndex))
}

func (h *callHook) OnTxEnd(ctx *reexec.TxContext, resetGas uint64) {
	log.Warn(fmt.Sprintf("OnTxEnd %#x", ctx.Transaction.Hash()))
}

func (h *callHook) OnCallEnter(ctx *reexec.CallCtx) {
	method := [4]byte{}
	if len(ctx.Input) >= 4 {
		copy(method[:], ctx.Input[0:4])
	}
	log.Warn(fmt.Sprintf("OnCallEnter %#x", method))
}

func (h *callHook) OnCallExit(ctx *reexec.CallCtx) {
	method := [4]byte{}
	if len(ctx.Input) >= 4 {
		copy(method[:], ctx.Input[0:4])
	}
	log.Warn(fmt.Sprintf("OnCallExit %#x", method))
	if ctx.Error != nil {
		log.Error(fmt.Sprintf("OnCallExit reverted. tx: %#v", ctx.Transaction.Hash()))
	}
}

func (p *demoPlugin) execute(ctx *plugin.PluginCtx) {
	bc := ctx.Eth.Chain()
	db := state.NewDatabaseWithConfigAndCache(ctx.Eth.ChainDb(), &trie.Config{Cache: 16})
	replayer := reexec.NewChainReplayer(db, bc, 100000)
	hook := &callHook{}
	start := time.Now()
	var (
		err     error
		statedb *state.StateDB
	)
	for i := 93015; i <= 163454; i++ {
		block := bc.GetBlockByNumber(uint64(i))
		statedb, err = replayer.ReplayBlock(block, statedb, hook)
		if err != nil {
			panic(err)
		}
	}
	// statedb, err := replayer.StateAtBlock(block)
	tr, err := statedb.Trie()
	if err != nil {
		panic(err)
	}
	fmt.Println("root:", tr.Hash())
	fmt.Printf("took: %v\n", time.Since(start))
}

func (p *demoPlugin) runCountDown(ctx *plugin.PluginCtx) {
	countDown := 5
	runTimer := time.After(time.Duration(countDown) * time.Second)
	for {
		fmt.Printf("Run afeter %d seconds\n", countDown)
		select {
		case <-time.After(1 * time.Second):
			countDown -= 1
		case <-runTimer:
			p.execute(ctx)
			return
		}
	}
}

func (p *demoPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	ctx.Log.Info("Demo plugin enabled!")
	go p.runCountDown(ctx)
	return nil
}

func (p *demoPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &demoPlugin{}
}
