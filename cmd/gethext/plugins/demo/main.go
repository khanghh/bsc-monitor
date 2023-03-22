package main

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	log.Warn(fmt.Sprintf("OnCallEnter %#x", hexutil.Bytes([]byte(ctx.Input))))
}

func (h *callHook) OnCallExit(ctx *reexec.CallCtx) {
	log.Warn(fmt.Sprintf("OnCallExit %#x", hexutil.Bytes([]byte(ctx.Input))))
}

func (p *demoPlugin) execute(ctx *plugin.PluginCtx) {
	bc := ctx.Eth.Chain()
	db := state.NewDatabaseWithConfigAndCache(ctx.Eth.ChainDb(), &trie.Config{Cache: 16})
	replayer := reexec.NewChainReplayer(db, bc, 100000)
	block := bc.GetBlockByNumber(75454)
	hook := &callHook{}
	statedb, err := replayer.ReplayBlock(block, nil, hook)
	if err != nil {
		panic(err)
	}
	tr, err := statedb.Trie()
	if err != nil {
		panic(err)
	}
	fmt.Println("root:", tr.Hash())
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
