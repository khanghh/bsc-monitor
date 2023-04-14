//
// Created on 2022/12/23 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package monitor

import (
	"sync"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type Processor interface {
	ProcessTx(state *state.StateDB, tx *types.Transaction, block *types.Block) error
}

// ChainMonitor calls registed processors to process every pending
// transactions recieved in txpool
type ChainMonitor struct {
	config *Config
	eth    *eth.Ethereum

	processors    map[string]Processor
	newTxsSubs    event.Subscription
	newTxsEventCh chan core.NewTxsEvent
	queue         chan *types.Transaction

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

func (m *ChainMonitor) processQueueLoop() {
	throttler := NewLimitWaitGroup(m.config.ProcessSlot)
	defer func() {
		throttler.Wait()
		m.wg.Done()
	}()
	processTx := func(throttler *LimitWaitGroup, tx *types.Transaction) {
		defer throttler.Done()
		state, err := m.eth.BlockChain().State()
		if err != nil {
			log.Error("Failed to get chain state", "error", err)
			return
		}
		wg := sync.WaitGroup{}
		for procName, proc := range m.Processors() {
			wg.Add(1)
			go func(procName string, proc Processor) {
				defer wg.Done()
				err := proc.ProcessTx(state, tx, nil)
				log.Warn("Process pending transaction error", "processor", procName, "error", err)
			}(procName, proc)
		}
		wg.Wait()
	}
	for {
		select {
		case tx := <-m.queue:
			throttler.Add()
			go processTx(throttler, tx)
		case <-m.quitCh:
			return
		}
	}
}

func (m *ChainMonitor) enqueueTxsLoop() {
	defer m.wg.Done()
	m.queue = make(chan *types.Transaction, m.config.ProcessQueue)
	for {
		select {
		case event := <-m.newTxsEventCh:
			enqueued := 0
			for _, tx := range event.Txs {
				select {
				case m.queue <- tx:
					enqueued++
				default:
				}
			}
			log.Debug("Enqueue new pending txs to monitor", "count", enqueued, "received", len(event.Txs), "queued", len(m.queue))
		case <-m.newTxsSubs.Err():
			return
		}
	}
}

func (m *ChainMonitor) Start() error {
	m.wg.Add(1)
	m.newTxsEventCh = make(chan core.NewTxsEvent)
	m.newTxsSubs = m.eth.TxPool().SubscribeNewTxsEvent(m.newTxsEventCh)
	go m.enqueueTxsLoop()

	m.wg.Add(1)
	go m.processQueueLoop()
	return nil
}

func (m *ChainMonitor) Stop() error {
	close(m.quitCh)
	m.newTxsSubs.Unsubscribe()
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

func NewChainMonitor(cfg *Config, db ethdb.Database, ethereum *eth.Ethereum) (*ChainMonitor, error) {
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
