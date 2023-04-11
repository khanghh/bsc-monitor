//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
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

const safeBlockDistance = 7

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain
	replayer   *reexec.ChainReplayer

	lastBlock *types.Block
	indexData []*blockIndex

	status  uint32
	pauseCh chan bool
	termCh  chan struct{}
	quitCh  chan struct{}
}

// processBlock re-executes every transactions in block and extracts neccessary info into indexdb
func (idx *ChainIndexer) processBlock(block *types.Block, statedb *state.StateDB) (*state.StateDB, *blockIndex, error) {
	data := newBlockIndex(idx.indexdb, block)
	statedb, err := idx.replayer.ReplayBlock(block, statedb, newBlockParser(data))
	if err != nil {
		return nil, nil, err
	}
	return statedb, data, nil
}

func (idx *ChainIndexer) cleanUpAndStop() {
	if len(idx.indexData) > 0 {
		idx.commitIndexData(idx.indexData)
	}
	close(idx.termCh)
	atomic.SwapUint32(&idx.status, uint32(task.StatusStopped))
}

func (idx *ChainIndexer) commitIndexData(indexData []*blockIndex) {
	batch := idx.diskdb.NewBatch()
	first := indexData[0].block.NumberU64()
	last := indexData[len(indexData)-1].block.NumberU64()
	numAccount := 0
	for _, blockData := range indexData {
		numAccount += len(blockData.dirtyAccounts)
		blockData.Commit(batch, true)
	}
	extdb.WriteLastIndexBlock(idx.diskdb, idx.lastBlock.Hash())
	if err := batch.Write(); err != nil {
		log.Crit("Faield to commit index data", "range", []uint64{first, last}, "error", err)
	}
	idx.indexData = make([]*blockIndex, 0)
	log.Info("Persisted indexing data", "range", []uint64{first, last}, "accounts", numAccount)
}

func (idx *ChainIndexer) indexingLoop() {
	defer idx.cleanUpAndStop()
	waitResume := func() {
		for paused := range idx.pauseCh {
			if !paused {
				return
			}
		}
	}
	statedb, err := idx.replayer.StateAtBlock(idx.lastBlock)
	if err != nil {
		log.Error("Could not get historical state", "number", idx.lastBlock.NumberU64(), "root", idx.lastBlock.Root())
		return
	}
	idx.indexData = make([]*blockIndex, 0)
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
			start := time.Now()
			log.Debug("Indexing block", "number", block.Number())
			var blockData *blockIndex
			statedb, blockData, err = idx.processBlock(block, statedb)
			if err != nil {
				log.Error("Indexer could not process block", "number", block.NumberU64(), "error", err)
				// retry process block
				continue
			}
			idx.lastBlock = block
			idx.indexData = append(idx.indexData, blockData)
			proctime += time.Since(start)
			if proctime > 10*time.Second {
				idx.commitIndexData(idx.indexData)
				proctime = 0
			}
			continue
		}
		time.Sleep(time.Second)
	}
}

func (idx *ChainIndexer) Status() task.TaskStatus {
	return task.TaskStatus(idx.status)
}

func (idx *ChainIndexer) Start() error {
	// Check the current task status before running
	status := atomic.LoadUint32(&idx.status)
	if status != uint32(task.StatusPending) && status != uint32(task.StatusPaused) {
		return nil
	}
	var lastBlock *types.Block
	lastBlockHash := extdb.ReadLastIndexBlock(idx.diskdb)
	if lastBlockHash == nilHash {
		lastBlock = idx.blockchain.GetBlockByNumber(0)
	} else {
		lastBlock = idx.blockchain.GetBlockByHash(lastBlockHash)
	}

	idx.lastBlock = lastBlock
	if !atomic.CompareAndSwapUint32(&idx.status, status, uint32(task.StatusRunning)) {
		return nil
	}
	log.Info("Start indexing blockchain", "number", idx.lastBlock.Number(), "root", idx.lastBlock.Root())
	go idx.indexingLoop()
	return nil
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

func (idx *ChainIndexer) Stop() {
	if atomic.LoadUint32(&idx.status) != uint32(task.StatusRunning) {
		return
	}
	close(idx.quitCh)
	idx.Wait()
	log.Info("ChainIndexer stopped")
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
