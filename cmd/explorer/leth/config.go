package leth

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	gopsutil "github.com/shirou/gopsutil/mem"
)

var DefaultConfig = Config{
	RPCUrl: "http://localhost:8545",
	TxPool: core.DefaultTxPoolConfig,

	TotalCache:              4096,
	TrieCleanCacheJournal:   "triecache",
	TrieCleanCacheRejournal: 60 * time.Minute,
	TrieTimeout:             5 * time.Minute,
	TriesInMemory:           128,
	TriesVerifyMode:         core.LocalVerify,
	Preimages:               false,

	RPCGasCap:     50000000,
	RPCEVMTimeout: 5 * time.Second,
	EVMConfig:     vm.Config{},
}

type Config struct {
	RPCUrl  string
	Genesis *core.Genesis `toml:"-"`

	TxPool     core.TxPoolConfig
	TotalCache int `toml:",omitempty"` // Total memory in MB to distribute among DB cache and trie cache, snapshot cache

	// Database options
	SkipBcVersionCheck bool   `toml:"-"`
	DatabaseHandles    int    `toml:"-"`
	DatabaseCache      int    `toml:",omitempty"`
	DatabaseFreezer    string `toml:",omitempty"`

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
	Preimages       bool            `toml:"-"`

	// EVM options
	RPCGasCap     uint64        `toml:",ommitempty"`
	RPCEVMTimeout time.Duration `toml:",ommitempty"`
	EVMConfig     vm.Config     `toml:"-"`
}

func (config *Config) Sanitize() error {
	if len(config.RPCUrl) == 0 {
		return errors.New("rpc url must be provided")
	}
	// Cap the totalCache allowance and tune the garbage collector
	totalCache := config.TotalCache
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

	if totalCache > 0 {
		config.DatabaseCache = 40 * config.TotalCache / 100
		config.TrieCleanCache = 15 * config.TotalCache / 100
		config.TrieDirtyCache = 25 * config.TotalCache / 100
		config.SnapshotCache = config.TotalCache - config.DatabaseCache - config.TrieCleanCache - config.TrieDirtyCache
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

	totalCache = config.DatabaseCache + config.TrieCleanCache + config.TrieDirtyCache + config.SnapshotCache
	log.Info("Cache memory allocations",
		"TotalCache", common.StorageSize(totalCache*1024*1024),
		"DatabaseCache", common.StorageSize(config.DatabaseCache*1024*1024),
		"TrieCleanCache", common.StorageSize(config.TrieCleanCache*1024*1024),
		"TrieDirtyCache", common.StorageSize(config.TrieDirtyCache*1024*1024),
		"SnapshotCache", common.StorageSize(config.SnapshotCache*1024*1024),
	)

	return nil
}
