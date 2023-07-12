//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package main

import (
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/task"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

const (
	extNamespace      = "/eth/db/ethexplorer"
	extDatabaseName   = "ethexplorer"
	extDatabaseHandle = 512
	extDatabaseCache  = 1024
	indexerTaskName   = "indexer"
	pluginsDataDir    = "plugins"
)

type EthExplorerConfig struct {
	InstanceDir string
	Plugins     *plugin.Config
	Monitor     *monitor.Config
}

func (c *EthExplorerConfig) sanitize() error {
	if len(c.Plugins.DataDir) == 0 {
		c.Plugins.DataDir = filepath.Join(c.InstanceDir, pluginsDataDir)
	}
	return nil
}

type EthExplorer struct {
	config        *EthExplorerConfig
	chainMonitor  *monitor.ChainMonitor
	pluginManager *plugin.PluginManager
	taskManager   *task.TaskManager

	quitCh   chan struct{}
	quitLock sync.Mutex
}

func (s *EthExplorer) Start() error {
	log.Info("Starting chain explorer service")
	if s.config.Monitor.Enabled {
		if err := s.chainMonitor.Start(); err != nil {
			log.Error("Could not start chain monitor", "error", err)
			return err
		}
	}
	if err := s.pluginManager.Start(); err != nil {
		log.Error("Could not start plugin manager", "error", err)
		return err
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
		s.taskManager.Stop()
		close(s.quitCh)
	}
	s.quitLock.Unlock()
	return nil
}

func NewExplorerService(cfg *EthExplorerConfig, node *node.Node, eth *eth.Ethereum) (*EthExplorer, error) {
	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	diskdb, err := node.OpenDatabase(extDatabaseName, extDatabaseCache, extDatabaseHandle, extNamespace, false)
	if err != nil {
		return nil, err
	}

	abiutils.InitDefaultParser(diskdb)
	chainMonitor, err := monitor.NewChainMonitor(cfg.Monitor, diskdb, eth.BlockChain())
	if err != nil {
		return nil, err
	}

	taskManager, err := task.NewTaskManager()
	if err != nil {
		return nil, err
	}

	pluginManager, err := plugin.NewPluginManager(cfg.Plugins, diskdb, node, eth.APIBackend, chainMonitor, taskManager)
	if err != nil {
		return nil, err
	}

	instance := &EthExplorer{
		config:        cfg,
		chainMonitor:  chainMonitor,
		pluginManager: pluginManager,
		taskManager:   taskManager,
		quitCh:        make(chan struct{}),
	}
	return instance, nil
}
