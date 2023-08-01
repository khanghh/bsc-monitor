package leth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (lc *LightChain) CurrentHeader() *types.Header {
	return lc.hc.CurrentHeader()
}

func (lc *LightChain) GetHighestVerifiedHeader() *types.Header {
	return lc.highestVerifiedHeader.Load().(*types.Header)
}

// GetJustifiedNumber returns the highest justified blockNumber on the branch including and before `header`.
func (lc *LightChain) GetJustifiedNumber(header *types.Header) uint64 {
	if p, ok := lc.engine.(consensus.PoSA); ok {
		justifiedBlockNumber, _, err := p.GetJustifiedNumberAndHash(lc, header)
		if err == nil {
			return justifiedBlockNumber
		}
	}
	// return 0 when err!=nil
	// so the input `header` will at a disadvantage during reorg
	return 0
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the lightchain's internal cache.
func (lc *LightChain) CurrentBlock() *types.Block {
	return lc.currentBlock.Load().(*types.Block)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (lc *LightChain) HaasHeader(hash common.Hash, number uint64) bool {
	return lc.hc.HasHeader(hash, number)
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (lc *LightChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return lc.hc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (lc *LightChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return lc.hc.GetHeaderByHash(hash)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (lc *LightChain) GetHeaderByNumber(number uint64) *types.Header {
	return lc.hc.GetHeaderByNumber(number)
}

// GetHeadersFrom returns a contiguous segment of headers, in rlp-form, going
// backwards from the given number.
func (lc *LightChain) GetHeadersFrom(number, count uint64) []rlp.RawValue {
	return lc.hc.GetHeadersFrom(number, count)
}

// GetBody retrieves a block body (transactions and uncles) from the database by
// hash, caching it if found.
func (lc *LightChain) GetBody(hash common.Hash) *types.Body {
	panic("not implemented")
}

// GetBodyRLP retrieves a block body in RLP encoding from the database by hash,
// caching it if found.
func (lc *LightChain) GetBodyRLP(hash common.Hash) rlp.RawValue {
	panic("not implemented")
}

// HasBlock checks if a block is fully present in the blockchain or not
func (lc *LightChain) HasBlock(hash common.Hash, number uint64) bool {
	return false
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (lc *LightChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	if block, ok := lc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	// Falling back to fetch block by hash
	block, err := lc.odr.GetBlockByHash(hash)
	if err != nil {
		return nil
	}
	lc.blockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (lc *LightChain) GetBlockByHash(hash common.Hash) *types.Block {
	number := lc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return lc.GetBlock(hash, *number)
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (lc *LightChain) GetBlockByNumber(number uint64) *types.Block {
	hash := lc.hc.GetCanonicalHash(number)
	if hash == (common.Hash{}) {
		return nil
	}
	return lc.GetBlock(hash, number)
}

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (lc *LightChain) GetBlocksFromHash(hash common.Hash, n int) []*types.Block {
	panic("not implemented")
}

// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
func (lc *LightChain) GetReceiptsByHash(hash common.Hash) types.Receipts {
	return nil
}

// GetUnclesInChain retrieves all the uncles from a given block backwards until
// a specific distance is reached.
func (lc *LightChain) GetUnclesInChain(block *types.Block, length int) []*types.Header {
	uncles := []*types.Header{}
	for i := 0; block != nil && i < length; i++ {
		uncles = append(uncles, block.Uncles()...)
		block = lc.GetBlock(block.ParentHash(), block.NumberU64()-1)
	}
	return uncles
}

// GetCanonicalHash returns the canonical hash for a given block number
func (lc *LightChain) GetCanonicalHash(number uint64) common.Hash {
	return lc.hc.GetCanonicalHash(number)
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (lc *LightChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	return lc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (lc *LightChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return lc.hc.GetTd(hash, number)
}

// HasState checks if state trie is fully present in the database or not.
func (lc *LightChain) HasState(hash common.Hash) bool {
	return false
}

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (lc *LightChain) HasBlockAndState(hash common.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := lc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return lc.HasState(block.Root())
}

// TrieNode retrieves a blob of data associated with a trie node
// either from ephemeral in-memory cache, or from persistent storage.
func (lc *LightChain) TrieNode(hash common.Hash) ([]byte, error) {
	return lc.stateCache.TrieDB().Node(hash)
}

// ContractCode retrieves a blob of data associated with a contract hash
// either from ephemeral in-memory cache, or from persistent storage.
func (lc *LightChain) ContractCode(hash common.Hash) ([]byte, error) {
	return lc.stateCache.ContractCode(common.Hash{}, hash)
}

// ContractCodeWithPrefix retrieves a blob of data associated with a contract
// hash either from ephemeral in-memory cache, or from persistent storage.
//
// If the code doesn't exist in the in-memory cache, check the storage with
// new code scheme.
func (lc *LightChain) ContractCodeWithPrefix(hash common.Hash) ([]byte, error) {
	type codeReader interface {
		ContractCodeWithPrefix(addrHash, codeHash common.Hash) ([]byte, error)
	}
	return lc.stateCache.(codeReader).ContractCodeWithPrefix(common.Hash{}, hash)
}

// State returns a new mutable state based on the current HEAD block.
func (lc *LightChain) State() (*state.StateDB, error) {
	return lc.StateAt(lc.CurrentBlock().Root())
}

// StateAt returns a new mutable state based on a particular point in time.
func (lc *LightChain) StateAt(root common.Hash) (*state.StateDB, error) {
	return nil, nil
}

// Config retrieves the chain's fork configuration.
func (lc *LightChain) Config() *params.ChainConfig { return lc.chainConfig }

// Engine retrieves the lightchain's consensus engine.
func (lc *LightChain) Engine() consensus.Engine { return lc.engine }

// GetVMConfig returns the block chain VM config.
func (lc *LightChain) GetVMConfig() *vm.Config {
	return &lc.vmConfig
}

// Snapshots returns the lightchain snapshot tree.
func (lc *LightChain) Snapshots() *snapshot.Tree {
	return lc.snaps
}

// Validator returns the current validator.
func (lc *LightChain) Validator() Validator {
	return lc.validator
}

// Processor returns the current processor.
func (lc *LightChain) Processor() Processor {
	return lc.processor
}

// StateCache returns the caching database underpinning the lightchain instance.
func (lc *LightChain) StateCache() state.Database {
	return lc.stateCache
}

// GasLimit returns the gas limit of the current HEAD block.
func (lc *LightChain) GasLimit() uint64 {
	return lc.CurrentBlock().GasLimit()
}

// Genesis retrieves the chain's genesis block.
func (lc *LightChain) Genesis() *types.Block {
	return lc.genesisBlock
}

// SubscribeRemovedLogsEvent registers a subscription of RemovedLogsEvent.
func (lc *LightChain) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return lc.scope.Track(lc.rmLogsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (lc *LightChain) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return lc.scope.Track(lc.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (lc *LightChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return lc.scope.Track(lc.chainHeadFeed.Subscribe(ch))
}

// SubscribeChainBlockEvent registers a subscription of ChainBlockEvent.
func (lc *LightChain) SubscribeChainBlockEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return lc.scope.Track(lc.chainBlockFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (lc *LightChain) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return lc.scope.Track(lc.chainSideFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (lc *LightChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return lc.scope.Track(lc.logsFeed.Subscribe(ch))
}

// SubscribeBlockProcessingEvent registers a subscription of bool where true means
// block processing has started while false means it has stopped.
func (lc *LightChain) SubscribeBlockProcessingEvent(ch chan<- bool) event.Subscription {
	return lc.scope.Track(lc.blockProcFeed.Subscribe(ch))
}

// SubscribeFinalizedHeaderEvent registers a subscription of FinalizedHeaderEvent.
func (lc *LightChain) SubscribeFinalizedHeaderEvent(ch chan<- core.FinalizedHeaderEvent) event.Subscription {
	return lc.scope.Track(lc.finalizedHeaderFeed.Subscribe(ch))
}
