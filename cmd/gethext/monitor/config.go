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
	DefaultConfig = Config{
		ProcessQueue: 10000,
		ProcessSlot:  1000,
	}
)

type Config struct {
	ProcessQueue int
	ProcessSlot  int
	ABIDir       string
}

func (cfg *Config) Sanitize() error {
	if cfg.ProcessQueue < 1 {
		log.Warn("Sanitizing monitor process txs queue", "provided", cfg.ProcessQueue, "updated", DefaultConfig.ProcessQueue)
		cfg.ProcessQueue = DefaultConfig.ProcessQueue
	}
	if cfg.ProcessSlot < 1 {
		log.Warn("Sanitizing monitor process txs slot", "provided", cfg.ProcessSlot, "updated", DefaultConfig.ProcessSlot)
		cfg.ProcessSlot = DefaultConfig.ProcessSlot
	}
	return nil
}
