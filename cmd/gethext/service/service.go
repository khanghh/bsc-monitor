//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package service

import (
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/service/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	monitorNamespace      = "/eth/db/monitor"
	monitorDatabaseName   = "monitor"
	monitorDatabaseHandle = 512
	monitorDatabaseCache  = 1024
	indexerTaskName       = "indexer"
)

type MonitorServiceOptions struct {
	MonitorConfig *monitor.Config
	ReExecConfig  *task.Config
	PluginDir     string
	NoIndexing    bool
}

type MonitorService struct {
	opts          *MonitorServiceOptions
	chainMonitor  *monitor.ChainMonitor
	chainIndexer  *monitor.ChainIndexer
	pluginManager *plugin.PluginManager
	taskManager   *task.TaskManager

	quitCh   chan struct{}
	quitLock sync.Mutex
}

func (s *MonitorService) Start() error {
	log.Info("Starting chain monitor service")
	if err := s.pluginManager.LoadPlugins(); err != nil {
		log.Error("Could not load plugins", "error", err)
		return err
	}
	if err := s.chainMonitor.Start(); err != nil {
		log.Error("Could not start chain monitor", "error", err)
		return err
	}
	if !s.opts.NoIndexing {
		if err := s.chainIndexer.Start(); err != nil {
			log.Error("Could not start chain indexer", "error", err)
			return err
		}
	}
	return nil
}

func (s *MonitorService) Stop() error {
	log.Info("Stopping monitor service...")
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

func (s *MonitorService) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "idx",
			Version:   "1.0",
			Service:   monitor.NewIndexerAPI(s.chainIndexer),
			Public:    true,
		},
	}
}

func NewMonitorService(opts *MonitorServiceOptions, node *node.Node, eth *eth.Ethereum) (*MonitorService, error) {
	diskdb, err := node.OpenDatabaseWithFreezer(monitorDatabaseName, monitorDatabaseCache, monitorDatabaseHandle, "",
		monitorNamespace, false, false, false, false, true)
	if err != nil {
		return nil, err
	}
	chainMonitor, err := monitor.NewChainMonitor(opts.MonitorConfig, diskdb, eth)
	if err != nil {
		return nil, err
	}

	stateCache := state.NewDatabaseWithConfigAndCache(eth.ChainDb(), &trie.Config{Cache: 1024})
	chainIndexer, err := monitor.NewChainIndexer(diskdb, stateCache, eth.BlockChain())
	if err != nil {
		return nil, err
	}

	taskManager, err := task.NewTaskManager(opts.ReExecConfig)
	if err != nil {
		return nil, err
	}

	pluginManager, err := plugin.NewPluginManager(opts.PluginDir, node, eth.APIBackend, chainMonitor, taskManager)
	if err != nil {
		return nil, err
	}

	instance := &MonitorService{
		opts:          opts,
		chainMonitor:  chainMonitor,
		chainIndexer:  chainIndexer,
		pluginManager: pluginManager,
		taskManager:   taskManager,
		quitCh:        make(chan struct{}),
	}
	return instance, nil
}