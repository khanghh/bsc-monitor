//
// Created on 2022/12/23 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package monitor

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
	maxTriesInMemory = 127
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
	config     *MonitorConfig
	blockchain *core.BlockChain
	replayer   *reexec.ChainReplayer

	processors   map[string]Processor
	chainHeadSub event.Subscription
	chainHeadCh  chan core.ChainHeadEvent

	wg     sync.WaitGroup
	mtx    sync.Mutex
	cancel context.CancelFunc
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

func (m *ChainMonitor) processBlock(ctx context.Context, block *types.Block) {
	defer func() {
		if err := recover(); err != nil {
			log.Error(fmt.Sprintf("ChainMonitor process block panic:\n%#v", err))
		}
	}()
	hook := newMonitorHook(block)
	statedb, err := m.replayer.ReplayBlock(ctx, block, nil, hook)
	if err != nil {
		return
	}
	for _, proc := range m.processors {
		proc.ProcessBlock(statedb, block, hook.txs)
	}
}

func (m *ChainMonitor) eventLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	defer func() {
		m.wg.Done()
	}()
	for {
		select {
		case event := <-m.chainHeadCh:
			parentBlock := m.blockchain.GetBlockByHash(event.Block.ParentHash())
			if parentBlock == nil || !m.blockchain.HasState(parentBlock.Root()) {
				continue
			}
			log.Info("ChainMonitor processing block", "number", event.Block.NumberU64())
			m.processBlock(ctx, event.Block)
			m.replayer.CapTrieDB(maxTriesInMemory)
		case <-m.quitCh:
			return
		}
	}
}

func (m *ChainMonitor) Start() error {
	processors := make([]string, 0, len(m.processors))
	for procName := range m.processors {
		processors = append(processors, procName)
	}
	log.Info("Start monitoring blockchain", "processors", processors)
	m.chainHeadCh = make(chan core.ChainHeadEvent)
	m.chainHeadSub = m.blockchain.SubscribeChainHeadEvent(m.chainHeadCh)
	m.wg.Add(1)
	go m.eventLoop()
	return nil
}

func (m *ChainMonitor) Stop() error {
	close(m.quitCh)
	if m.chainHeadSub != nil {
		m.chainHeadSub.Unsubscribe()
	}
	if m.cancel != nil {
		m.cancel()
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

func NewChainMonitor(cfg *MonitorConfig, db ethdb.Database, blockchain *core.BlockChain) (*ChainMonitor, error) {
	if err := cfg.Sanitize(); err != nil {
		return nil, err
	}
	replayer := reexec.NewChainReplayer(blockchain.StateCache(), blockchain)
	return &ChainMonitor{
		config:     cfg,
		blockchain: blockchain,
		replayer:   replayer,
		quitCh:     make(chan struct{}),
		processors: make(map[string]Processor),
	}, nil
}
