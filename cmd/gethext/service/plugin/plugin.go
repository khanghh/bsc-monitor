//
// Created on 2022/12/26 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package plugin

import (
	"context"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/cmd/gethext/model"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type Plugin interface {
	OnEnable(ctx *PluginCtx) error
	OnDisable(ctx *PluginCtx) error
}

type NodeBackend interface {
	RegisterAPIs(api []rpc.API) error
	RegisterHandler(name, path string, handler http.Handler) error
	GetClient() *rpc.Client
}

type EthBackend interface {
	SyncProgress() ethereum.SyncProgress
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	FeeHistory(ctx context.Context, blockCount int, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (*big.Int, [][]*big.Int, []*big.Int, []float64, error)
	Chain() *core.BlockChain
	ChainDb() ethdb.Database
	AccountManager() *accounts.Manager
	ExtRPCEnabled() bool
	RPCGasCap() uint64
	RPCEVMTimeout() time.Duration
	RPCTxFeeCap() float64
	UnprotectedAllowed() bool

	// Blockchain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
	HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error)
	CurrentHeader() *types.Header
	CurrentBlock() *types.Block
	BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)
	PendingBlockAndReceipts() (*types.Block, types.Receipts)
	GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error)
	GetTd(ctx context.Context, hash common.Hash) *big.Int
	GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error)
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

	// Transaction pool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
	GetPoolTransactions() (types.Transactions, error)
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions)
	SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription

	// Filter API
	BloomStatus() (uint64, uint64)
	GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
	SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription
	SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription
	ChainConfig() *params.ChainConfig
	Engine() consensus.Engine
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive, preferDisk bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error)
}

type MonitorBackend interface {
	RegisterProcessor(name string, proc monitor.Processor)
	UnregisterProcessor(name string)
}

type ChainIndexer interface {
	GetAccount(addr common.Address) (*model.AccountInfo, error)
	GetAccountAt(root common.Hash, addr common.Address) (*model.AccountInfo, error)
}

type TaskManager interface {
	AddTask(name string, task task.Task) error
	RunTask(name string, task task.Task) error
	GetTask(name string) (task.Task, error)
	KillTask(name string) error
}

// sharedCtx exposes common useful modules for the plugin to
// implement its own business
type sharedCtx struct {
	Node    *node.Node
	Eth     EthBackend
	Monitor MonitorBackend
	Indexer ChainIndexer
	TaskMgr TaskManager
	Storage map[string]interface{}
	mtx     sync.RWMutex
}

func (ctx *sharedCtx) Set(key string, val interface{}) {
	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()
	if ctx.Storage == nil {
		ctx.Storage = make(map[string]interface{})
	}
	ctx.Storage[key] = val
}

func (ctx *sharedCtx) Get(key string) (interface{}, bool) {
	ctx.mtx.RLock()
	defer ctx.mtx.RUnlock()
	if ctx.Storage == nil {
		return nil, false
	}
	val, ok := ctx.Storage[key]
	return val, ok
}

// PluginCtx provides access to internal services for a plugin, each plugin
// has it own context
type PluginCtx struct {
	*sharedCtx
	Log        log.Logger
	LoadConfig func(cfg interface{}) error
	SaveConfig func(cfg interface{}) error
}
