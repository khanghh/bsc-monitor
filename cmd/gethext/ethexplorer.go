//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package main

import (
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/task"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	extNamespace      = "/eth/db/ethexplorer"
	extDatabaseName   = "ethexplorer"
	extDatabaseHandle = 512
	extDatabaseCache  = 1024
	indexerTaskName   = "indexer"
)

type EthExplorerConfig struct {
	ConfigFile string
	Monitor    *monitor.MonitorConfig
	Indexer    *monitor.IndexerConfig
	PluginsDir string
}

type EthExplorer struct {
	cfg           *EthExplorerConfig
	chainMonitor  *monitor.ChainMonitor
	chainIndexer  *monitor.ChainIndexer
	pluginManager *plugin.PluginManager
	taskManager   *task.TaskManager

	quitCh   chan struct{}
	quitLock sync.Mutex
}

func (s *EthExplorer) Start() error {
	log.Info("Starting chain explorer service")
	if err := s.pluginManager.LoadPlugins(); err != nil {
		log.Error("Could not load plugins", "error", err)
		return err
	}
	if s.cfg.Monitor.Enabled {
		if err := s.chainMonitor.Start(); err != nil {
			log.Error("Could not start chain monitor", "error", err)
			return err
		}
	}
	if s.cfg.Indexer.Enabled {
		if err := s.chainIndexer.Start(); err != nil {
			log.Error("Could not start chain indexer", "error", err)
			return err
		}
	}
	return nil
}

func (s *EthExplorer) Stop() error {
	log.Info("Stopping chain explorer service...")
	s.quitLock.Lock()
	select {
	case <-s.quitCh:
	default:
		s.pluginManager.Stop()
		s.chainMonitor.Stop()
		s.chainIndexer.Stop()
		s.taskManager.Stop()
		close(s.quitCh)
	}
	s.quitLock.Unlock()
	log.Info("Chain monitor service stopped")
	return nil
}

func (s *EthExplorer) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "idx",
			Version:   "1.0",
			Service:   monitor.NewIndexerAPI(s.chainIndexer),
			Public:    true,
		},
	}
}

func NewExplorerService(opts *EthExplorerConfig, node *node.Node, eth *eth.Ethereum) (*EthExplorer, error) {
	diskdb, err := node.OpenDatabaseWithFreezer(extDatabaseName, extDatabaseCache, extDatabaseHandle, "",
		extNamespace, false, false, false, false, true)
	if err != nil {
		return nil, err
	}
	chainMonitor, err := monitor.NewChainMonitor(opts.Monitor, diskdb, eth)
	if err != nil {
		return nil, err
	}

	stateCache := state.NewDatabaseWithConfigAndCache(eth.ChainDb(), &trie.Config{Cache: 16})
	chainIndexer, err := monitor.NewChainIndexer(diskdb, stateCache, eth.BlockChain())
	if err != nil {
		return nil, err
	}

	taskManager, err := task.NewTaskManager()
	if err != nil {
		return nil, err
	}

	pluginManager, err := plugin.NewPluginManager(opts.PluginsDir, node, eth.APIBackend, chainMonitor, taskManager)
	if err != nil {
		return nil, err
	}

	instance := &EthExplorer{
		cfg:           opts,
		chainMonitor:  chainMonitor,
		chainIndexer:  chainIndexer,
		pluginManager: pluginManager,
		taskManager:   taskManager,
		quitCh:        make(chan struct{}),
	}
	return instance, nil
}
