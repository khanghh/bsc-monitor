//
// Created on 2022/12/20 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package leth

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/parlia"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/shutdowncheck"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

// LightEthereum is a lightweight Ethereum backend that runs with an RPC url.
// It does not implement the P2P protocol and prunes the chain state during block importing.
type LightEthereum struct {
	config          *Config
	chainDb         ethdb.Database
	apiBackend      ethapi.Backend
	blockchain      *core.BlockChain
	txpool          *core.TxPool
	engine          consensus.Engine
	hanlder         *handler
	shutdownTracker *shutdowncheck.ShutdownTracker
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
			Service:   ethapi.NewPublicEthereumAPI(leth.apiBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   ethapi.NewPublicBlockChainAPI(leth.apiBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPublicDebugAPI(leth.apiBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPrivateDebugAPI(leth.apiBackend),
		},
		{
			Namespace: "admin",
			Service:   NewAdminAPI(leth),
		},
	}...)
}

func (leth *LightEthereum) Start() error {
	leth.shutdownTracker = shutdowncheck.NewShutdownTracker(leth.chainDb)
	leth.shutdownTracker.MarkStartup()
	leth.shutdownTracker.Start()
	return nil
}

func (leth *LightEthereum) Stop() error {
	if err := leth.hanlder.Stop(); err != nil {
		return err
	}
	leth.blockchain.Stop()
	leth.engine.Close()

	// Clean shutdown marker as the last thing before closing db
	leth.shutdownTracker.Stop()
	return nil
}

func New(config *Config, chainDb ethdb.Database) (leth *LightEthereum, err error) {
	// Ensure configuration values are valid
	if err := config.Sanitize(); err != nil {
		return nil, err
	}

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, nil, nil, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	if err := checkBlockChainVersion(chainDb); err != nil {
		return nil, err
	}

	leth = &LightEthereum{
		config:  config,
		chainDb: chainDb,
	}
	var (
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit:     config.TrieCleanCache,
			TrieCleanJournal:   config.TrieCleanCacheJournal,
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
	bcOps := []core.BlockChainOption{
		core.EnableLightProcessor,
		core.EnablePipelineCommit,
	}
	leth.apiBackend = &EthAPIBackend{leth}
	leth.engine = parlia.New(chainConfig, chainDb, leth.apiBackend, genesisHash)
	leth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, chainConfig, leth.engine, config.EVMConfig, nil, nil, bcOps...)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		leth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	log.Info("Initialised chain configuration", "config", chainConfig)
	leth.txpool = core.NewTxPool(leth.config.TxPool, chainConfig, leth.blockchain)
	leth.hanlder = newHandler(NewRpcConnector(leth.config.RPCUrl), leth.blockchain, leth.txpool)
	return leth, nil
}
