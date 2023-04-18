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
		Cache:   16, // Cache memory in MB for chain replayer
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
	Cache   int
	Enabled bool
}

func (cfg *IndexerConfig) Sanitize() error {
	if cfg.Cache <= 0 {
		log.Warn("Sanitizing indexer cache size", "provided", cfg.Cache, "updated", DefaultIndexerConfig.Cache)
	}
	return nil
}
