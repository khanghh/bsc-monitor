//
// Created on 2022/12/20 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package leth

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"time"

	godebug "runtime/debug"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	gopsutil "github.com/shirou/gopsutil/mem"
)

var DefaultConfig = Config{
	TxPool: core.DefaultTxPoolConfig,

	DatabaseCache:    512,
	PruneAncientData: false,

	CacheMemory:             4096,
	TrieCleanCache:          154,
	TrieCleanCacheJournal:   "triecache",
	TrieCleanCacheRejournal: 60 * time.Minute,
	TrieDirtyCache:          256,
	TrieTimeout:             5 * time.Minute,
	SnapshotCache:           102,
	TriesInMemory:           128,
	TriesVerifyMode:         core.LocalVerify,
	Preimages:               false,

	RPCGasCap:     50000000,
	RPCEVMTimeout: 5 * time.Second,
}

type Config struct {
	RPCUrl      string
	GenesisFile string
	Genesis     *core.Genesis `toml:"-"`
	DataDir     string        `toml:",ommitempty"`

	TxPool      core.TxPoolConfig
	CacheMemory int `toml:",omitempty"` // Megabytes of memory allocated to internal caching

	// Database options
	SkipBcVersionCheck bool   `toml:"-"`
	DatabaseHandles    int    `toml:"-"`
	DatabaseCache      int    `toml:",omitempty"`
	DatabaseFreezer    string `toml:",omitempty"`
	PruneAncientData   bool   `toml:",omitempty"`

	// trie cache options
	TrieCleanCache          int           `toml:",omitempty"`
	TrieCleanCacheJournal   string        `toml:",omitempty"`
	TrieCleanCacheRejournal time.Duration `toml:",omitempty"`
	TrieDirtyCache          int           `toml:",omitempty"`
	NoPruning               bool          `toml:",omitempty"`
	TrieTimeout             time.Duration `toml:",omitempty"`

	SnapshotCache   int             `toml:",omitempty"`
	TriesInMemory   uint64          `toml:",omitempty"`
	TriesVerifyMode core.VerifyMode `toml:",omitempty"`
	Preimages       bool            `toml:",omitempty"`

	// EVM options
	RPCGasCap     uint64        `toml:",ommitempty"`
	RPCEVMTimeout time.Duration `toml:",ommitempty"`
}

// loadGenesis will load the given JSON format genesis file
func loadGenesis(genesisPath string) (*core.Genesis, error) {
	file, err := os.Open(genesisPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		return nil, err
	}
	return genesis, nil
}

func (config *Config) Sanitize() (*Config, error) {
	if len(config.RPCUrl) == 0 {
		return nil, errors.New("rpc url must be provided")
	}
	if len(config.GenesisFile) == 0 {
		return nil, errors.New("genesis file must be provided")
	}
	var err error
	if config.Genesis, err = loadGenesis(config.GenesisFile); err != nil {
		return nil, err
	}
	// Cap the totalCache allowance and tune the garbage collector
	totalCache := config.CacheMemory
	mem, err := gopsutil.VirtualMemory()
	if err == nil {
		if 32<<(^uintptr(0)>>63) == 32 && mem.Total > 2*1024*1024*1024 {
			log.Warn("Lowering memory allowance on 32bit arch", "available", mem.Total/1024/1024, "addressable", 2*1024)
			mem.Total = 2 * 1024 * 1024 * 1024
		}
		allowance := int(mem.Total / 1024 / 1024 / 3)
		if totalCache > allowance {
			log.Warn("Sanitizing cache to Go's GC limits", "provided", totalCache, "updated", allowance)
			totalCache = allowance
		}
	}

	// Ensure Go's GC ignores the database cache for trigger percentage
	gogc := math.Max(20, math.Min(100, 100/(float64(totalCache)/1024)))
	log.Debug("Sanitizing Go's GC trigger", "percent", int(gogc))
	godebug.SetGCPercent(int(gogc))

	if totalCache > 0 {
		config.DatabaseCache = 40 * config.CacheMemory / 100
		config.TrieCleanCache = 15 * config.CacheMemory / 100
		config.TrieDirtyCache = 25 * config.CacheMemory / 100
		config.SnapshotCache = config.CacheMemory - config.DatabaseCache - config.TrieCleanCache - config.TrieDirtyCache
	}

	// sanitize database options
	if config.DatabaseHandles == 0 {
		config.DatabaseHandles = utils.MakeDatabaseHandles()
	}
	if config.DatabaseCache == 0 {
		config.DatabaseCache = DefaultConfig.DatabaseCache
	}

	// sanitize EVM options
	if config.RPCGasCap == 0 {
		config.RPCGasCap = DefaultConfig.RPCGasCap
	}
	if config.RPCEVMTimeout == 0 {
		config.RPCEVMTimeout = DefaultConfig.RPCEVMTimeout
	}

	// sanitize trie cache config
	if config.TrieCleanCache == 0 {
		config.TrieCleanCache = DefaultConfig.TrieCleanCache
	}
	if len(config.TrieCleanCacheJournal) == 0 {
		config.TrieCleanCacheJournal = DefaultConfig.TrieCleanCacheJournal
	}
	if config.TrieCleanCacheRejournal == 0 {
		config.TrieCleanCacheRejournal = DefaultConfig.TrieCleanCacheRejournal
	}
	if config.TrieDirtyCache == 0 {
		config.TrieDirtyCache = DefaultConfig.TrieDirtyCache
	}
	if config.TrieTimeout == 0 {
		config.TrieTimeout = DefaultConfig.TrieTimeout
	}
	if config.SnapshotCache == 0 {
		config.SnapshotCache = DefaultConfig.SnapshotCache
	}
	if config.TriesInMemory == 0 {
		config.TriesInMemory = DefaultConfig.TriesInMemory
	}
	if config.TriesVerifyMode == 0 {
		config.TriesVerifyMode = DefaultConfig.TriesVerifyMode
	}

	return config, nil
}
