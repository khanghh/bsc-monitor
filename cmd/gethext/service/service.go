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
	"github.com/ethereum/go-ethereum/cmd/gethext/service/reexec"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
)

const (
	monitorNamespace      = "/eth/db/monitor"
	monitorDatabaseName   = "monitor"
	monitorDatabaseHandle = 512
	monitorDatabaseCache  = 1024
)

type MonitorServiceOptions struct {
	MonitorConfig *monitor.Config
	ReExecConfig  *reexec.Config
	PluginDir     string
}

type MonitorService struct {
	db            ethdb.Database
	chainMonitor  *monitor.ChainMonitor
	pluginManager *plugin.PluginManager
	taskManager   *reexec.TaskManager

	quitCh   chan struct{}
	quitLock sync.Mutex
}

func (s *MonitorService) Start() error {
	log.Info("Starting chain monitor service.")
	if err := s.pluginManager.LoadPlugins(); err != nil {
		log.Error("Could not load plugins", "error", err)
		return err
	}
	if err := s.chainMonitor.Start(); err != nil {
		log.Error("Could not start chain monitor", "error", err)
		return err
	}
	return nil
}

func (s *MonitorService) Stop() {
	s.quitLock.Lock()
	select {
	case <-s.quitCh:
	default:
		s.taskManager.Stop()
		s.pluginManager.Stop()
		s.chainMonitor.Stop()
		close(s.quitCh)
	}
	s.quitLock.Unlock()
	log.Info("Chain monitor service stopped.")
}

func NewMonitorService(opts *MonitorServiceOptions, node *node.Node, eth *eth.Ethereum) (*MonitorService, error) {
	db, err := node.OpenDatabaseWithFreezer(monitorDatabaseName, monitorDatabaseCache, monitorDatabaseHandle, "",
		monitorNamespace, false, false, false, false, true)
	if err != nil {
		return nil, err
	}
	chainMonitor, err := monitor.NewChainMonitor(opts.MonitorConfig, db, eth)
	if err != nil {
		return nil, err
	}
	taskManager, err := reexec.NewTaskManager(opts.ReExecConfig)
	if err != nil {
		return nil, err
	}
	pluginManager, err := plugin.NewPluginManager(opts.PluginDir, node, eth.APIBackend, chainMonitor, taskManager)
	if err != nil {
		return nil, err
	}
	instance := &MonitorService{
		chainMonitor:  chainMonitor,
		pluginManager: pluginManager,
		taskManager:   taskManager,
		quitCh:        make(chan struct{}),
	}
	return instance, nil
}
