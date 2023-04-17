//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/log"
)

var (
	DefaultMonitorConfig = MonitorConfig{
		ProcessQueue: 10000,
		ProcessSlot:  1000,
		Enabled:      false,
	}
	DefaultIndexerConfig = IndexerConfig{
		ABIDir:  "abis",
		Enabled: false,
	}
)

type MonitorConfig struct {
	ProcessQueue int
	ProcessSlot  int
	Enabled      bool
}

func (cfg *MonitorConfig) Sanitize() error {
	if cfg.ProcessQueue < 1 {
		log.Warn("Sanitizing monitor process txs queue", "provided", cfg.ProcessQueue, "updated", DefaultMonitorConfig.ProcessQueue)
		cfg.ProcessQueue = DefaultMonitorConfig.ProcessQueue
	}
	if cfg.ProcessSlot < 1 {
		log.Warn("Sanitizing monitor process txs slot", "provided", cfg.ProcessSlot, "updated", DefaultMonitorConfig.ProcessSlot)
		cfg.ProcessSlot = DefaultMonitorConfig.ProcessSlot
	}
	return nil
}

type IndexerConfig struct {
	ABIDir  string
	Enabled bool
}

func (cfg *IndexerConfig) Sanitize() error {
	if len(cfg.ABIDir) == 0 {
		log.Warn("Sanitizing indexer contract ABIs directory path", "provided", cfg.ABIDir, "updated", DefaultIndexerConfig.ABIDir)
	}
	return nil
}
