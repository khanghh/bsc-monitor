package leth

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

// LEthAPIBackend implements internal/ethapi.Backend
type LEthAPIBackend struct {
	leth *LightEthereum
}

// General Ethereum API
func (b *LEthAPIBackend) SyncProgress() ethereum.SyncProgress {
	return b.leth.Syncer().Progress()
}

func (b *LEthAPIBackend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) FeeHistory(ctx context.Context, blockCount int, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (*big.Int, [][]*big.Int, []*big.Int, []float64, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) Chain() *core.BlockChain {
	return nil
}

func (b *LEthAPIBackend) ChainDb() ethdb.Database {
	panic("not implement")
}

func (b *LEthAPIBackend) AccountManager() *accounts.Manager {
	panic("not implement")
}

func (b *LEthAPIBackend) ExtRPCEnabled() bool {
	panic("not implement")
}

func (b *LEthAPIBackend) RPCGasCap() uint64 {
	return b.leth.config.RPCGasCap
}

func (b *LEthAPIBackend) RPCEVMTimeout() time.Duration {
	return b.leth.config.RPCEVMTimeout
}

func (b *LEthAPIBackend) RPCTxFeeCap() float64 {
	panic("not implement")
}

func (b *LEthAPIBackend) UnprotectedAllowed() bool {
	panic("not implement")
}

// Blockchain API
func (b *LEthAPIBackend) SetHead(number uint64) {
	// b.eth.handler.downloader.Cancel()
	b.leth.chain.SetHead(number)
}

func (b *LEthAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		return nil, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.leth.chain.CurrentBlock().Header(), nil
	}
	return b.leth.chain.GetHeaderByNumber(uint64(number)), nil
}

func (b *LEthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.leth.chain.GetHeaderByHash(hash), nil
}

func (b *LEthAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.leth.chain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.leth.chain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *LEthAPIBackend) CurrentHeader() *types.Header {
	return b.leth.chain.CurrentHeader()
}

func (b *LEthAPIBackend) CurrentBlock() *types.Block {
	return b.leth.chain.CurrentBlock()
}

func (b *LEthAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		return nil, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.leth.chain.CurrentBlock(), nil
	}
	return b.leth.chain.GetBlockByNumber(uint64(number)), nil
}

func (b *LEthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.leth.chain.GetBlockByHash(hash), nil
}

func (b *LEthAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.leth.chain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.leth.chain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.leth.chain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *LEthAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		return nil, nil, nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.leth.chain.StateAt(header.Root)
	return stateDb, header, err
}

func (b *LEthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.leth.chain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.leth.chain.StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *LEthAPIBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	panic("not implement")
}

func (b *LEthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	if header := b.leth.chain.GetHeaderByHash(hash); header != nil {
		return b.leth.chain.GetTd(hash, header.Number.Uint64())
	}
	return nil
}

func (b *LEthAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error) {
	vmError := func() error { return nil }
	if vmConfig == nil {
		vmConfig = b.leth.chain.GetVMConfig()
	}
	txContext := core.NewEVMTxContext(msg)
	context := core.NewEVMBlockContext(header, b.leth.chain, nil)
	return vm.NewEVM(context, txContext, state, b.leth.chain.Config(), *vmConfig), vmError, nil
}

func (b *LEthAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.leth.BlockChain().SubscribeChainEvent(ch)
}

func (b *LEthAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.leth.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *LEthAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.leth.BlockChain().SubscribeChainSideEvent(ch)
}

// Transaction pool API
func (b *LEthAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	panic("not implement")
}

func (b *LEthAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	panic("not implement")
}

func (b *LEthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) Stats() (pending int, queued int) {
	panic("not implement")
}

func (b *LEthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	panic("not implement")
}

func (b *LEthAPIBackend) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
	panic("not implement")
}

func (b *LEthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.leth.TxPool().SubscribeNewTxsEvent(ch)
}

// Filter API
func (b *LEthAPIBackend) BloomStatus() (uint64, uint64) {
	panic("not implement")
}

func (b *LEthAPIBackend) GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	panic("not implement")
}

func (b *LEthAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.leth.BlockChain().SubscribeLogsEvent(ch)
}

func (b *LEthAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	panic("not implement")
}

func (b *LEthAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.leth.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *LEthAPIBackend) SubscribeNewVoteEvent(chan<- core.NewVoteEvent) event.Subscription {
	panic("not implement")
}

func (b *LEthAPIBackend) SubscribeFinalizedHeaderEvent(ch chan<- core.FinalizedHeaderEvent) event.Subscription {
	panic("not implement")
}

func (b *LEthAPIBackend) ChainConfig() *params.ChainConfig {
	return b.leth.BlockChain().Config()
}

func (b *LEthAPIBackend) Engine() consensus.Engine {
	panic("not implement")
}

func (b *LEthAPIBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive, preferDisk bool) (*state.StateDB, error) {
	panic("not implement")
}

func (b *LEthAPIBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	panic("not implement")
}
