//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain
	replayer   *reexec.ChainReplayer

	lastBlock   *types.Block
	statedb     *state.StateDB
	pendingData *blockIndex

	status  uint32
	pauseCh chan bool
	termCh  chan struct{}
	quitCh  chan struct{}
}

// processBlock re-executes every transactions in block and extracts neccessary info into indexdb
func (idx *ChainIndexer) processBlock(block *types.Block) error {
	time.Sleep(1 * time.Second)
	statedb, err := idx.replayer.ReplayBlock(block, idx.statedb, nil)
	if err != nil {
		return err
	}
	fmt.Println(statedb.IntermediateRoot(true))
	return nil
}

func (idx *ChainIndexer) indexingLoop() {
	defer func() {
		close(idx.termCh)
		atomic.SwapUint32(&idx.status, uint32(task.StatusStopped))
	}()
	waitResume := func() {
		for paused := range idx.pauseCh {
			if !paused {
				return
			}
		}
	}
	proctime := time.Duration(0)
	for {
		select {
		case paused := <-idx.pauseCh:
			if !paused {
				waitResume()
			}
		case <-idx.quitCh:
			return
		default:
		}
		block := idx.blockchain.GetBlockByNumber(idx.lastBlock.NumberU64() + 1)
		if block != nil {
			log.Debug("Indexing block", "number", block.Number())
			start := time.Now()
			if err := idx.processBlock(block); err != nil {
				log.Error("Indexer could not process block", "number", block.NumberU64())
				return
			}
			idx.lastBlock = block
			proctime += time.Since(start)
			if proctime > 10*time.Second {
				proctime = 0
				log.Info("Indexing progress", "number", idx.lastBlock.NumberU64(), "accounts", len(idx.pendingData.DirtyAccounts()))
			}
			continue
		}
		time.Sleep(time.Second)
	}
}

func (idx *ChainIndexer) Status() task.TaskStatus {
	return task.TaskStatus(idx.status)
}

func (idx *ChainIndexer) Run() {
	// Check the current task status before running
	status := atomic.LoadUint32(&idx.status)
	if status != uint32(task.StatusPending) && status != uint32(task.StatusPaused) {
		return
	}
	var lastBlock *types.Block
	lastBlockHash := extdb.ReadLastIndexBlock(idx.diskdb)
	if lastBlockHash == nilHash {
		lastBlock = idx.blockchain.GetBlockByNumber(0)
	} else {
		lastBlock = idx.blockchain.GetBlockByHash(lastBlockHash)
	}
	statedb, err := idx.replayer.StateAtBlock(lastBlock)
	if err != nil {
		log.Error("Could not get historical state", "number", lastBlock.NumberU64(), "root", lastBlock.Root())
		return
	}
	idx.statedb = statedb
	idx.lastBlock = lastBlock
	idx.pendingData = newBlockIndex(idx.indexdb, lastBlock)
	if err != nil {
		log.Error("Could not initialize indexing data", "number", lastBlock.NumberU64(), "root", lastBlock.Root())
		return
	}
	if !atomic.CompareAndSwapUint32(&idx.status, status, uint32(task.StatusRunning)) {
		return
	}
	log.Info("Start indexing blockchain", "number", idx.lastBlock.Number(), "root", idx.lastBlock.Root())
	go idx.indexingLoop()
}

func (idx *ChainIndexer) Wait() {
	<-idx.termCh
}

func (idx *ChainIndexer) Pause() {
	// Try to update the status from running to paused
	if atomic.CompareAndSwapUint32(&idx.status, uint32(task.StatusRunning), uint32(task.StatusPaused)) {
		idx.pauseCh <- true
	}
}

func (idx *ChainIndexer) Resume() {
	if atomic.CompareAndSwapUint32(&idx.status, uint32(task.StatusPaused), uint32(task.StatusRunning)) {
		idx.pauseCh <- false
	}
}

func (idx *ChainIndexer) Abort() {
}

func (idx *ChainIndexer) Stop() {
	close(idx.quitCh)
	idx.Wait()
}

func NewChainIndexer(diskdb ethdb.Database, stateCache state.Database, bc *core.BlockChain) (*ChainIndexer, error) {
	return &ChainIndexer{
		diskdb:     diskdb,
		indexdb:    NewIndexDB(diskdb, stateCache),
		blockchain: bc,
		replayer:   reexec.NewChainReplayer(stateCache, bc, 10000),
		pauseCh:    make(chan bool),
		termCh:     make(chan struct{}),
		quitCh:     make(chan struct{}),
	}, nil
}
