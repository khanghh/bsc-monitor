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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
	maxTriesInMemory = 127
)

type Processor = reexec.TransactionHook

// ChainMonitor calls registered processors to process every pending transactions received in txpool
type ChainMonitor struct {
	config     *Config
	blockchain *core.BlockChain
	replayer   *reexec.ChainReplayer

	processors   map[Processor]bool
	chainHeadSub event.Subscription
	chainHeadCh  chan core.ChainHeadEvent

	wg     sync.WaitGroup
	mtx    sync.Mutex
	cancel context.CancelFunc
	quitCh chan struct{}
}

func (m *ChainMonitor) getProcessors() []Processor {
	ret := make([]Processor, 0)
	for proc, enabled := range m.processors {
		if enabled {
			ret = append(ret, proc)
		}
	}
	return ret
}

func (m *ChainMonitor) processBlock(ctx context.Context, block *types.Block) {
	defer func() {
		if err := recover(); err != nil {
			log.Error(fmt.Sprintf("ChainMonitor process block panic:\n%#v", err))
		}
	}()

	hook := &monitorHook{m.getProcessors()}
	_, err := m.replayer.ReplayBlock(ctx, block, nil, hook)
	if err != nil {
		return
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
	log.Info("Start monitoring blockchain")
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

func (m *ChainMonitor) AddProcessor(proc Processor) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.processors[proc] = true
}

func (m *ChainMonitor) RemoveProcessor(proc Processor) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.processors, proc)
}

func NewChainMonitor(cfg *Config, db ethdb.Database, bc *core.BlockChain) (*ChainMonitor, error) {
	if err := cfg.Sanitize(); err != nil {
		return nil, err
	}
	// MaxTrieInMemory is set to 128, ensuring the state trie stays in the blockchain's state cache (state.Database).
	// For monitoring, we only re-execute the lastest block, so we can use the blockchain's state cache directly.
	replayer := reexec.NewChainReplayer(bc.StateCache(), bc)
	return &ChainMonitor{
		config:     cfg,
		blockchain: bc,
		replayer:   replayer,
		quitCh:     make(chan struct{}),
		processors: make(map[Processor]bool),
	}, nil
}
