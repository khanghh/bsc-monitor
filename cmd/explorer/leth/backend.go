//
// Created on 2022/12/20 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package leth

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/parlia"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/shutdowncheck"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	chainDataDir     = "chaindata"
	chaindbNamespace = "leth/db/block/"
)

var (
	defaultVmConfig = vm.Config{
		EnablePreimageRecording: false,
	}
)

// LightEthereum implements Ethereum blockchain service without p2p network
// usingprovided genesis and RPC url
type LightEthereum struct {
	config          *Config
	databases       map[*closeTrackingDB]struct{} // All open databases
	chainDb         ethdb.Database
	blockchain      *core.BlockChain
	txpool          *core.TxPool
	engine          consensus.Engine
	hanlder         *handler
	APIBackend      ethapi.Backend
	lock            sync.Mutex
	shutdownTracker *shutdowncheck.ShutdownTracker // Tracks if and when the node has shutdown ungracefully
}

func (leth *LightEthereum) Start() error {
	leth.shutdownTracker.Start()
	return leth.hanlder.Start()
}

func (leth *LightEthereum) Stop() error {
	if err := leth.hanlder.Stop(); err != nil {
		return err
	}
	leth.blockchain.Stop()
	leth.txpool.Stop()
	leth.engine.Close()

	// Clean shutdown marker as the last thing before closing db
	leth.shutdownTracker.Stop()

	errs := leth.closeDatabases()
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (leth *LightEthereum) checkBlockChainVersion(chainDb ethdb.Database) error {
	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}

	if bcVersion != nil && *bcVersion > core.BlockChainVersion {
		return fmt.Errorf("database version is v%d, Geth %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
	} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
		if bcVersion != nil { // only print warning on upgrade, not on init
			log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
		} else {
			log.Info("Initialize blockchain database version", "dbVer", core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	return nil
}

func (s *LightEthereum) BlockChain() *core.BlockChain { return s.blockchain }
func (s *LightEthereum) TxPool() *core.TxPool         { return s.txpool }
func (s *LightEthereum) Engine() consensus.Engine     { return s.engine }
func (s *LightEthereum) ChainDb() ethdb.Database      { return s.chainDb }
func (s *LightEthereum) IsListening() bool            { return true }

func (leth *LightEthereum) APIs() []rpc.API {
	// Append any APIs exposed explicitly by the consensus engine
	apis := leth.engine.APIs(leth.BlockChain())
	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   ethapi.NewPublicEthereumAPI(leth.APIBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   ethapi.NewPublicBlockChainAPI(leth.APIBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPublicDebugAPI(leth.APIBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPrivateDebugAPI(leth.APIBackend),
		},
		{
			Namespace: "admin",
			Service:   NewAdminAPI(leth),
		},
	}...)
}

func NewLightEthereum(config *Config) (*LightEthereum, error) {
	// Ensure configuration values are valid
	config, err := config.Sanitize()
	if err != nil {
		return nil, err
	}

	leth := LightEthereum{
		config:    config,
		databases: make(map[*closeTrackingDB]struct{}),
	}

	chainDb, err := leth.OpenDatabaseWithFreezer(chainDataDir, config.DatabaseCache, config.DatabaseHandles,
		config.DatabaseFreezer, chaindbNamespace, false, false, false, false, true)
	if err != nil {
		return nil, err
	}
	leth.chainDb = chainDb

	if err := leth.checkBlockChainVersion(chainDb); err != nil {
		return nil, err
	}
	leth.shutdownTracker = shutdowncheck.NewShutdownTracker(chainDb)
	leth.shutdownTracker.MarkStartup()

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, nil, nil, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		leth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	log.Info("Initialised chain configuration", "config", chainConfig)
	leth.APIBackend = &EthAPIBackend{leth: &leth}
	leth.engine = parlia.New(chainConfig, chainDb, leth.APIBackend, genesisHash)
	bcOps := []core.BlockChainOption{
		core.EnableLightProcessor,
		core.EnablePipelineCommit,
	}
	var (
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit:     config.TrieCleanCache,
			TrieCleanJournal:   leth.ResolvePath(config.TrieCleanCacheJournal),
			TrieCleanRejournal: config.TrieCleanCacheRejournal,
			TrieDirtyLimit:     config.TrieDirtyCache,
			TrieDirtyDisabled:  config.NoPruning,
			TrieTimeLimit:      config.TrieTimeout,
			NoTries:            config.TriesVerifyMode != core.LocalVerify,
			SnapshotLimit:      config.SnapshotCache,
			TriesInMemory:      config.TriesInMemory,
			Preimages:          config.Preimages,
		}
	)
	leth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, chainConfig, leth.engine, defaultVmConfig, nil, nil, bcOps...)
	if err != nil {
		return nil, err
	}
	leth.txpool = core.NewTxPool(leth.config.TxPool, chainConfig, leth.blockchain)
	leth.hanlder = newHandler(NewRpcConnector(leth.config.RPCUrl), leth.blockchain, leth.txpool)
	return &leth, nil
}
