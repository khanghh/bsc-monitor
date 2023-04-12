//
// Created on 2023/3/13 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package reexec

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/systemcontracts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

type ChainReplayer struct {
	db            state.Database   // Isolated memory state cache for chain re-execution
	bc            *core.BlockChain // Ethereum blockchain provide blocks to be replayed
	triesInMemory []common.Hash    // Keep track of which tries that still alive in memory
	maxReExec     uint64           // Max re-execution blocks to regenerate statedb
}

func (re *ChainReplayer) StateCache() state.Database {
	return re.db
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
			re.db.TrieDB().Dereference(root)
		}
	}
}

func (re *ChainReplayer) Reset() {
	re.triesInMemory = make([]common.Hash, 0)
	re.db.Purge()
}

// cacheSystemContracts caches account trie and storage trie of system contracts to live state cache.
// Parlia use live database to call system contracts, when replay transactions old state has been dereferenced,
// we must copy them to the live database `re.bc.StateCache()`
func (re *ChainReplayer) cacheSystemContracts(root common.Hash, statedb *state.StateDB) {
	contractAddr := common.HexToAddress(systemcontracts.ValidatorContract)
	addrHash := crypto.Keccak256Hash(contractAddr[:])
	str := statedb.StorageTrie(contractAddr)
	if _, err := re.bc.StateCache().OpenStorageTrie(addrHash, str.Hash()); err == nil {
		return
	}
	log.Debug("Caching system contracts state for replaying", "root", root)
	tr, _ := statedb.Trie()
	re.bc.StateCache().CacheAccount(root, tr)
	re.bc.StateCache().CacheStorage(addrHash, str.Hash(), str)
}

// StateAtBlock returns statedb after all transactions in block was executed
func (re *ChainReplayer) StateAtBlock(block *types.Block) (statedb *state.StateDB, err error) {
	statedb, err = state.New(block.Root(), re.db, nil)
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
		parent := re.bc.GetBlock(current.ParentHash(), current.NumberU64()-1)
		if parent == nil {
			return nil, fmt.Errorf("missing block %#x %d", current.ParentHash(), current.NumberU64()-1)
		}
		current = parent
		statedb, err = state.New(parent.Root(), re.db, nil)
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
		parent common.Hash // keep track of parent state root
	)
	blockNum := block.NumberU64()
	for current.NumberU64() < blockNum {
		if time.Since(logged) > 8*time.Second {
			log.Info("Regenerating historical state", "current", current.NumberU64()+1, "target", blockNum, "remaining", blockNum-current.NumberU64()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
		next := current.NumberU64() + 1
		if current = re.bc.GetBlockByNumber(next); current == nil {
			return nil, fmt.Errorf("block #%d not found", next)
		}
		// Replayer use its own state.Database to cache tries and isolated with the live one. This cause the parlia engine
		// failed to load state of the system contracts at epoch block, we need to cache their generated states into the live database
		if current.NumberU64()%200 == 0 {
			re.cacheSystemContracts(parent, statedb)
		}
		statedb, _, _, _, err = re.bc.Processor().Process(current, statedb, vm.Config{})
		if err != nil {
			return nil, fmt.Errorf("processing block %d failed: %v", current.NumberU64(), err)
		}
		statedb.SetExpectedStateRoot(current.Root())
		statedb.Finalise(re.bc.Config().IsEIP158(current.Number()))
		statedb.AccountsIntermediateRoot()
		root, _, err := statedb.Commit(nil)
		if err != nil {
			return nil, fmt.Errorf("commit state at block %d failed: %v", current.NumberU64(), err)
		}
		re.triesInMemory = append(re.triesInMemory, root)
		parent = root
	}
	nodes, imgs := re.db.TrieDB().Size()
	log.Info("Historical state regenerated", "block", current.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	return statedb, nil
}

// StateAtTransaction returns the state before transaction was executed
func (re *ChainReplayer) StateAtTransaction(block *types.Block, txIndex uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	// Short circuit if it's genesis block.
	if block.NumberU64() == 0 {
		return nil, vm.BlockContext{}, nil, errors.New("no transaction in genesis")
	}
	// Create the parent state database
	parent := re.bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.BlockContext{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	// Get statedb of parent block to apply transactions
	statedb, err := re.StateAtBlock(parent)
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}
	// if block has no transaction return the state
	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, vm.BlockContext{}, statedb, nil
	}
	// Recompute transactions up to the target index.
	signer := types.MakeSigner(re.bc.Config(), block.Number())
	for idx, tx := range block.Transactions() {
		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer, block.BaseFee())
		txContext := core.NewEVMTxContext(msg)
		context := core.NewEVMBlockContext(block.Header(), re.bc, nil)
		if uint64(idx) == txIndex {
			return msg, context, statedb, nil
		}
		// Not yet the searched for transaction, execute on top of the current state
		vmenv := vm.NewEVM(context, txContext, statedb, re.bc.Config(), vm.Config{})
		if posa, ok := re.bc.Engine().(consensus.PoSA); ok && msg.From() == context.Coinbase &&
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
func (re *ChainReplayer) ReplayBlock(block *types.Block, base *state.StateDB, hook ReExecHook) (*state.StateDB, error) {
	if block.NumberU64() == 0 {
		return nil, errors.New("cannot replay genesis block")
	}
	var err error
	if base == nil {
		parent := re.bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
		if parent == nil {
			return nil, fmt.Errorf("missing parent block %#x %d", block.ParentHash(), block.NumberU64()-1)
		}
		base, err = re.StateAtBlock(parent)
		if err != nil {
			return nil, fmt.Errorf("missing base state: %v", err)
		}
	}
	if block.NumberU64()%200 == 0 {
		re.cacheSystemContracts(base.StateIntermediateRoot(), base)
	}
	signer := types.MakeSigner(re.bc.Config(), block.Number())
	tracer := NewCallTracerWithHook(block, signer, hook)
	statedb, _, _, _, err := re.bc.Processor().Process(block, base, vm.Config{Debug: true, Tracer: tracer})
	if err != nil {
		return nil, fmt.Errorf("replay block %d failed: %v", block.NumberU64(), err)
	}
	statedb.SetExpectedStateRoot(block.Root())
	statedb.Finalise(re.bc.Config().IsEIP158(block.Number()))
	statedb.AccountsIntermediateRoot()
	// commit to cache the state to database
	root, _, err := statedb.Commit(nil)
	if err != nil {
		return nil, fmt.Errorf("commit state for block %d failed: %v", block.NumberU64(), err)
	}
	re.triesInMemory = append(re.triesInMemory, root)
	return statedb, nil
}

// ReplayTransaction re-execute transaction at the provided index in a block
func (re *ChainReplayer) ReplayTransaction(block *types.Block, txIndex uint64, hook ReExecHook) (*state.StateDB, error) {
	transactions := block.Transactions()
	if txIndex >= uint64(len(transactions)) {
		return nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
	}
	tx := transactions[txIndex]
	msg, blkCtx, statedb, err := re.StateAtTransaction(block, txIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieving state at tx index %d in block %d: %v", txIndex, block.NumberU64(), err)
	}
	txCtx := core.NewEVMTxContext(msg)
	vmenv := vm.NewEVM(blkCtx, txCtx, statedb, re.bc.Config(), vm.Config{})
	if posa, ok := re.bc.Engine().(consensus.PoSA); ok && msg.From() == blkCtx.Coinbase &&
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
		db:        db,
		bc:        bc,
		maxReExec: math.MaxUint64,
	}
}
