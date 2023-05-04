//
// Created on 2022/12/23 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package monitor

import (
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type Processor interface {
	ProcessBlock(state *state.StateDB, block *types.Block, txResults []*reexec.TxContext) error
}

type monitorHook struct {
	block *types.Block
	txs   []*reexec.TxContext
}

func (h *monitorHook) OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {}

func (h *monitorHook) OnTxEnd(ctx *reexec.TxContext, resetGas uint64) {
	h.txs[int(ctx.TxIndex)] = ctx
}

func (h *monitorHook) OnCallEnter(ctx *reexec.CallCtx) {}

func (h *monitorHook) OnCallExit(ctx *reexec.CallCtx) {}

func newMonitorHook(block *types.Block) *monitorHook {
	return &monitorHook{
		block: block,
		txs:   make([]*reexec.TxContext, block.Transactions().Len()),
	}
}

// ChainMonitor calls registed processors to process every pending
// transactions recieved in txpool
type ChainMonitor struct {
	config   *MonitorConfig
	eth      *eth.Ethereum
	replayer *reexec.ChainReplayer

	processors   map[string]Processor
	chainHeadSub event.Subscription
	chainHeadCh  chan core.ChainHeadEvent

	wg     sync.WaitGroup
	mtx    sync.Mutex
	quitCh chan struct{}
}

func (m *ChainMonitor) Processors() map[string]Processor {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	ret := make(map[string]Processor)
	for procName, proc := range m.processors {
		ret[procName] = proc
	}
	return ret
}

func (m *ChainMonitor) processBlock(block *types.Block) {
	hook := newMonitorHook(block)
	statedb, err := m.replayer.ReplayBlock(block, nil, hook)
	if err != nil {
		return
	}
	for _, proc := range m.processors {
		proc.ProcessBlock(statedb, block, hook.txs)
	}
}

func (m *ChainMonitor) eventLoop() {
	defer func() {
		m.wg.Done()
	}()
	for {
		select {
		case event := <-m.chainHeadCh:
			m.processBlock(event.Block)
		case <-m.quitCh:
			return
		}
	}
}

func (m *ChainMonitor) Start() error {
	m.chainHeadCh = make(chan core.ChainHeadEvent)
	m.chainHeadSub = m.eth.BlockChain().SubscribeChainHeadEvent(m.chainHeadCh)
	go m.eventLoop()
	return nil
}

func (m *ChainMonitor) Stop() error {
	close(m.quitCh)
	if m.chainHeadSub != nil {
		m.chainHeadSub.Unsubscribe()
	}
	m.wg.Wait()
	log.Info("ChainMonitor stopped")
	return nil
}

func (m *ChainMonitor) RegisterProcessor(name string, proc Processor) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.processors[name] = proc
}

func (m *ChainMonitor) UnregisterProcessor(name string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.processors, name)
}

func NewChainMonitor(cfg *MonitorConfig, db ethdb.Database, ethereum *eth.Ethereum) (*ChainMonitor, error) {
	if err := cfg.Sanitize(); err != nil {
		return nil, err
	}
	return &ChainMonitor{
		config:     cfg,
		eth:        ethereum,
		quitCh:     make(chan struct{}),
		processors: make(map[string]Processor),
	}, nil
}
