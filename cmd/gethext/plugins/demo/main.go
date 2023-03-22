package main

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
)

type DemoConfig struct {
	Field1 string
	Field2 int
}

type demoPlugin struct {
	config DemoConfig
}

func OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {
}

func OnTxEnd(ctx *reexec.TxContext, resetGas uint64) {
}

func OnCallEnter(ctx *reexec.CallCtx) {
}

func OnCallExit(ctx *reexec.CallCtx) {

}

func (p *demoPlugin) execute(ctx *plugin.PluginCtx) {
	bc := ctx.Eth.Chain()
	db := state.NewDatabaseWithConfigAndCache(ctx.Eth.ChainDb(), &trie.Config{Cache: 16})
	replayer := reexec.NewChainReplayer(db, bc, 100000)
	block := bc.GetBlockByNumber(163454)
	statedb, err := replayer.StateAtBlock(block)
	if err != nil {
		panic(err)
	}
	tr, err := statedb.Trie()
	if err != nil {
		panic(err)
	}
	fmt.Println("root:", tr.Hash())

	// statedb, err := replayer.ReplayTransaction(block, 1, nil)
	// if err != nil {
	// 	panic(err)
	// }
	// sender := common.HexToAddress("0x7ae2f5b9e386cd1b50a4550696d957cb4900f03a")
	// balance := statedb.GetBalance(sender)
	// if balance == nil {
	// 	panic("no state")
	// }
	// fmt.Println("sender balance: ", balance.Uint64())
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
