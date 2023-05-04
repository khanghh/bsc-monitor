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
		Enabled: false,
	}
	DefaultIndexerConfig = IndexerConfig{
		Cache:   16, // Cache memory in MB for chain replayer
		Enabled: false,
	}
)

type MonitorConfig struct {
	Enabled bool
}

func (cfg *MonitorConfig) Sanitize() error {
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
