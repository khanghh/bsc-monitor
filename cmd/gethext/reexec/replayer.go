//
// Created on 2023/3/13 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package reexec

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

type ChainReplayer struct {
	stateCache    state.Database   // Isolated memory state cache for chain re-execution
	blockchain    *core.BlockChain // Ethereum blockchain provide blocks to be replayed
	processor     core.Processor   // State processor for replaying blockchain
	triesInMemory []common.Hash    // Keep track of which tries that still alive in memory
	maxReExec     uint64           // Max re-execution blocks to regenerate statedb
}

func (re *ChainReplayer) StateCache() state.Database {
	return re.stateCache
}

func (re *ChainReplayer) SetReExecBlocks(maxReExec uint64) {
	re.maxReExec = maxReExec
}

func (re *ChainReplayer) CapTrieDB(limit int) {
	if len(re.triesInMemory) > limit {
		capOffset := len(re.triesInMemory) - limit
		toEvict := re.triesInMemory[0:capOffset]
		re.triesInMemory = re.triesInMemory[capOffset:]
		for _, root := range toEvict {
			re.stateCache.TrieDB().Dereference(root)
		}
	}
}

func (re *ChainReplayer) Reset() {
	re.triesInMemory = make([]common.Hash, 0)
	re.stateCache.Purge()
}

// StateAtBlock returns statedb after all transactions in block was executed
func (re *ChainReplayer) StateAtBlock(ctx context.Context, block *types.Block) (statedb *state.StateDB, err error) {
	statedb, err = state.New(block.Root(), re.stateCache, nil)
	if err == nil {
		return statedb, nil
	}
	// State was available at historical point, regenerate
	// retrieve nearest historical state snapshot
	current := block
	for i := uint64(0); i < re.maxReExec; i++ {
		if current.NumberU64() == 0 {
			return nil, errors.New("genesis state is missing")
		}
		parent := re.blockchain.GetBlock(current.ParentHash(), current.NumberU64()-1)
		if parent == nil {
			return nil, fmt.Errorf("missing block %#x %d", current.ParentHash(), current.NumberU64()-1)
		}
		current = parent
		statedb, err = state.New(parent.Root(), re.stateCache, nil)
		if err == nil {
			break
		}
	}
	if err != nil {
		switch err.(type) {
		case *trie.MissingNodeError:
			return nil, fmt.Errorf("snapshot state unavailable (range=[%d,%d])", current.NumberU64(), block.NumberU64())
		default:
			return nil, err
		}
	}

	// start generating chain state at the given block from the snapshotted point
	var (
		start  = time.Now()
		logged time.Time
	)
	blockNum := block.NumberU64()
	for current.NumberU64() < blockNum {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if time.Since(logged) > 8*time.Second {
			log.Info("Regenerating historical state", "current", current.NumberU64()+1, "target", blockNum, "remaining", blockNum-current.NumberU64()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
		next := current.NumberU64() + 1
		if current = re.blockchain.GetBlockByNumber(next); current == nil {
			return nil, fmt.Errorf("block #%d not found", next)
		}
		start := time.Now()
		statedb, _, _, _, err = re.processor.Process(current, statedb, vm.Config{})
		if err != nil {
			return nil, err
		}
		elapsed := time.Since(start)
		if elapsed > time.Second {
			log.Warn(fmt.Sprintf("Regenerating historical state at block %d took %v", current.NumberU64(), elapsed))
		}
		statedb.SetExpectedStateRoot(current.Root())
		statedb.Finalise(re.blockchain.Config().IsEIP158(current.Number()))
		statedb.AccountsIntermediateRoot()
		root, _, err := statedb.Commit(nil)
		if err != nil {
			return nil, fmt.Errorf("commit state failed: %v", err)
		}
		re.triesInMemory = append(re.triesInMemory, root)
	}
	nodes, imgs := re.stateCache.TrieDB().Size()
	log.Info("Historical state regenerated", "block", current.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	return statedb, nil
}

// StateAtTransaction returns the state before transaction was executed
func (re *ChainReplayer) StateAtTransaction(ctx context.Context, block *types.Block, txIndex uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	// Short circuit if it's genesis block.
	if block.NumberU64() == 0 {
		return nil, vm.BlockContext{}, nil, errors.New("no transaction in genesis")
	}
	// Create the parent state database
	parent := re.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.BlockContext{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	// Get statedb of parent block to apply transactions
	statedb, err := re.StateAtBlock(ctx, parent)
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}
	// if block has no transaction return the state
	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, vm.BlockContext{}, statedb, nil
	}
	// Recompute transactions up to the target index.
	signer := types.MakeSigner(re.blockchain.Config(), block.Number())
	for idx, tx := range block.Transactions() {
		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer, block.BaseFee())
		txContext := core.NewEVMTxContext(msg)
		context := core.NewEVMBlockContext(block.Header(), re.blockchain, nil)
		if uint64(idx) == txIndex {
			return msg, context, statedb, nil
		}
		// Not yet the searched for transaction, execute on top of the current state
		vmenv := vm.NewEVM(context, txContext, statedb, re.blockchain.Config(), vm.Config{})
		if posa, ok := re.blockchain.Engine().(consensus.PoSA); ok && msg.From() == context.Coinbase &&
			posa.IsSystemContract(msg.To()) && msg.GasPrice().Cmp(big.NewInt(0)) == 0 {
			balance := statedb.GetBalance(consensus.SystemAddress)
			if balance.Cmp(common.Big0) > 0 {
				statedb.SetBalance(consensus.SystemAddress, big.NewInt(0))
				statedb.AddBalance(context.Coinbase, balance)
			}
		}
		statedb.Prepare(tx.Hash(), idx)
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
		// Ensure any modifications are committed to the state
		// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}
	return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
}

// ReplayBlock re-execute all transactions in provided block, if base state not provided, base state will be generated instead
func (re *ChainReplayer) ReplayBlock(ctx context.Context, block *types.Block, base *state.StateDB, hook ReExecHook) (*state.StateDB, error) {
	if block.NumberU64() == 0 {
		return nil, errors.New("cannot replay genesis block")
	}
	var err error
	if base == nil {
		parent := re.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
		if parent == nil {
			return nil, fmt.Errorf("missing parent block %#x %d", block.ParentHash(), block.NumberU64()-1)
		}
		base, err = re.StateAtBlock(ctx, parent)
		if err != nil {
			return nil, fmt.Errorf("missing base state: %v", err)
		}
	}
	signer := types.MakeSigner(re.blockchain.Config(), block.Number())
	tracer := NewCallTracerWithHook(block, signer, hook)
	statedb, _, _, _, err := re.processor.Process(block, base, vm.Config{Debug: true, Tracer: tracer})
	if err != nil {
		return nil, err
	}
	statedb.SetExpectedStateRoot(block.Root())
	statedb.Finalise(re.blockchain.Config().IsEIP158(block.Number()))
	statedb.AccountsIntermediateRoot()
	// commit to cache the state to database
	root, _, err := statedb.Commit(nil)
	if err != nil {
		return nil, fmt.Errorf("commit state failed: %v", err)
	}
	re.triesInMemory = append(re.triesInMemory, root)
	return statedb, nil
}

// ReplayTransaction re-execute transaction at the provided index in a block
func (re *ChainReplayer) ReplayTransaction(ctx context.Context, block *types.Block, txIndex uint64, hook ReExecHook) (*state.StateDB, error) {
	transactions := block.Transactions()
	if txIndex >= uint64(len(transactions)) {
		return nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
	}
	tx := transactions[txIndex]
	msg, blkCtx, statedb, err := re.StateAtTransaction(ctx, block, txIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieving state at tx index %d in block %d: %v", txIndex, block.NumberU64(), err)
	}
	txCtx := core.NewEVMTxContext(msg)
	signer := types.MakeSigner(re.blockchain.Config(), block.Number())
	tracer := NewCallTracerWithHook(block, signer, hook)
	vmenv := vm.NewEVM(blkCtx, txCtx, statedb, re.blockchain.Config(), vm.Config{Debug: true, Tracer: tracer})
	if posa, ok := re.blockchain.Engine().(consensus.PoSA); ok && msg.From() == blkCtx.Coinbase &&
		posa.IsSystemContract(msg.To()) && msg.GasPrice().Cmp(big.NewInt(0)) == 0 {
		balance := statedb.GetBalance(consensus.SystemAddress)
		if balance.Cmp(common.Big0) > 0 {
			statedb.SetBalance(consensus.SystemAddress, big.NewInt(0))
			statedb.AddBalance(blkCtx.Coinbase, balance)
		}
	}
	statedb.Prepare(tx.Hash(), int(txIndex))
	if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
		return nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
	}
	statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	return statedb, nil
}

func NewChainReplayer(db state.Database, bc *core.BlockChain) *ChainReplayer {
	return &ChainReplayer{
		stateCache: db,
		blockchain: bc,
		processor:  NewReplayProcessor(bc.Config(), bc, bc.Engine()),
		maxReExec:  math.MaxUint64,
	}
}
