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

var defaultConfig = &Config{
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

func (cfg *Config) Sanitize() (*Config, error) {
	if len(cfg.RPCUrl) == 0 {
		return nil, errors.New("rpc url must be provided")
	}
	if len(cfg.GenesisFile) == 0 {
		return nil, errors.New("genesis file must be provided")
	}
	var err error
	if cfg.Genesis, err = loadGenesis(cfg.GenesisFile); err != nil {
		return nil, err
	}
	// Cap the totalCache allowance and tune the garbage collector
	totalCache := cfg.CacheMemory
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
		cfg.DatabaseCache = 40 * cfg.CacheMemory / 100
		cfg.TrieCleanCache = 15 * cfg.CacheMemory / 100
		cfg.TrieDirtyCache = 25 * cfg.CacheMemory / 100
		cfg.SnapshotCache = cfg.CacheMemory - cfg.DatabaseCache - cfg.TrieCleanCache - cfg.TrieDirtyCache
	}

	// sanitize database options
	if cfg.DatabaseHandles == 0 {
		cfg.DatabaseHandles = utils.MakeDatabaseHandles()
	}
	if cfg.DatabaseCache == 0 {
		cfg.DatabaseCache = defaultConfig.DatabaseCache
	}

	// sanitize EVM options
	if cfg.RPCGasCap == 0 {
		cfg.RPCGasCap = defaultConfig.RPCGasCap
	}
	if cfg.RPCEVMTimeout == 0 {
		cfg.RPCEVMTimeout = defaultConfig.RPCEVMTimeout
	}

	// sanitize trie cache config
	if cfg.TrieCleanCache == 0 {
		cfg.TrieCleanCache = defaultConfig.TrieCleanCache
	}
	if len(cfg.TrieCleanCacheJournal) == 0 {
		cfg.TrieCleanCacheJournal = defaultConfig.TrieCleanCacheJournal
	}
	if cfg.TrieCleanCacheRejournal == 0 {
		cfg.TrieCleanCacheRejournal = defaultConfig.TrieCleanCacheRejournal
	}
	if cfg.TrieDirtyCache == 0 {
		cfg.TrieDirtyCache = defaultConfig.TrieDirtyCache
	}
	if cfg.TrieTimeout == 0 {
		cfg.TrieTimeout = defaultConfig.TrieTimeout
	}
	if cfg.SnapshotCache == 0 {
		cfg.SnapshotCache = defaultConfig.SnapshotCache
	}
	if cfg.TriesInMemory == 0 {
		cfg.TriesInMemory = defaultConfig.TriesInMemory
	}
	if cfg.TriesVerifyMode == 0 {
		cfg.TriesVerifyMode = defaultConfig.TriesVerifyMode
	}

	return cfg, nil
}
