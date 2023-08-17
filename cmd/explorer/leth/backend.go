package leth

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/parlia"
	"github.com/ethereum/go-ethereum/core"
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
	config *Config

	chainDb ethdb.Database
	chain   *LightChain
	txpool  *core.TxPool
	engine  consensus.Engine

	odr        *RpcOdr
	syncer     *ChainSyncer
	apiBackend *LEthAPIBackend

	shutdownTracker *shutdowncheck.ShutdownTracker
}

func (s *LightEthereum) BlockChain() *core.BlockChain { return nil }
func (s *LightEthereum) LightChain() *LightChain      { return s.chain }
func (s *LightEthereum) TxPool() *core.TxPool         { return s.txpool }
func (s *LightEthereum) Engine() consensus.Engine     { return s.engine }
func (s *LightEthereum) ChainDb() ethdb.Database      { return s.chainDb }
func (s *LightEthereum) IsListening() bool            { return true }
func (s *LightEthereum) Syncer() *ChainSyncer         { return s.syncer }

func (s *LightEthereum) APIs() []rpc.API {
	// Append any APIs exposed explicitly by the consensus engine
	apis := s.engine.APIs(s.BlockChain())
	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   ethapi.NewPublicEthereumAPI(s.apiBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   ethapi.NewPublicBlockChainAPI(s.apiBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPublicDebugAPI(s.apiBackend),
			Public:    true,
		},
		{
			Namespace: "debug",
			Version:   "1.0",
			Service:   ethapi.NewPrivateDebugAPI(s.apiBackend),
		},
	}...)
}

func (s *LightEthereum) Start() error {
	s.shutdownTracker.Start()
	return s.syncer.Start()
}

func (s *LightEthereum) Stop() error {
	s.syncer.Stop()
	s.chain.Stop()
	s.engine.Close()

	// Clean shutdown marker as the last thing before closing db
	s.shutdownTracker.Stop()
	return nil
}

func New(config *Config, chainDb ethdb.Database, lcOpts ...LightChainOption) (leth *LightEthereum, err error) {
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, nil, nil, nil)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}

	leth = &LightEthereum{
		config:          config,
		chainDb:         chainDb,
		shutdownTracker: shutdowncheck.NewShutdownTracker(chainDb),
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
	leth.odr, err = NewRpcOdr(chainDb, config.RPCUrl)
	if err != nil {
		return nil, err
	}
	leth.apiBackend = &LEthAPIBackend{leth}
	leth.engine = parlia.New(chainConfig, chainDb, &EthCallAPI{leth.apiBackend}, genesisHash)
	leth.chain, err = NewLightChain(leth.odr, chainDb, cacheConfig, chainConfig, leth.engine, config.EVMConfig, lcOpts...)
	if err != nil {
		return nil, err
	}
	leth.syncer = NewChainSyncer(leth.odr, leth.chain)
	log.Info("Initialised block chain configuration", "config", chainConfig)
	leth.shutdownTracker.MarkStartup()
	return leth, nil
}
