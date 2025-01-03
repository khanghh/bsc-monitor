//
// Created on 2022/12/26 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package plugin

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/gethext/monitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/task"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type Plugin interface {
	OnEnable(ctx *PluginCtx) error
	OnDisable(ctx *PluginCtx) error
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
	SubscribeNewVoteEvent(chan<- core.NewVoteEvent) event.Subscription
	SubscribeFinalizedHeaderEvent(ch chan<- core.FinalizedHeaderEvent) event.Subscription

	ChainConfig() *params.ChainConfig
	Engine() consensus.Engine

	// State assessors
	StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive, preferDisk bool) (*state.StateDB, error)
	StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error)
}

type MonitorBackend interface {
	AddProcessor(proc monitor.Processor)
	RemoveProcessor(proc monitor.Processor)
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
	db      ethdb.Database
	config  *ConfigStore
	Node    *node.Node
	Eth     EthBackend
	Monitor MonitorBackend
	TaskMgr TaskManager
	Storage map[string]interface{}
	mtx     sync.RWMutex
}

func (ctx *sharedCtx) Database() ethdb.Database {
	return ctx.db
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

func (ctx *sharedCtx) LoadConfig(name string, cfg interface{}) error {
	return ctx.config.LoadConfig(name, cfg)
}

func (ctx *sharedCtx) OpenDatabase(prefix string) ethdb.Database {
	plPrefix := extdb.PluginDataPrefix(prefix)
	return rawdb.NewTable(ctx.db, string(plPrefix))
}

// PluginCtx provides access to internal services for a plugin, each plugin has it own context
type PluginCtx struct {
	*sharedCtx
	DataDir    string
	EventScope event.SubscriptionScope
}
