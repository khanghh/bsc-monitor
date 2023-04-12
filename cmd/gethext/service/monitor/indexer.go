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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

const (
	maxRetryIndexAttempt = 5
	safeBlockDistance    = 7
)

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain
	replayer   *reexec.ChainReplayer

	lastBlock *types.Block
	indexData []*blockIndexData

	status  uint32
	pauseCh chan bool
	termCh  chan struct{}
	quitCh  chan struct{}
}

// processBlock re-executes every transactions in block and extracts neccessary info into indexdb
func (idx *ChainIndexer) processBlock(block *types.Block, statedb *state.StateDB) (*state.StateDB, *blockIndexData, error) {
	data := newBlockIndexData(idx.indexdb, block)
	if idx.lastBlock.Root() == block.Root() {
		return statedb, data, nil
	}
	statedb, err := idx.replayer.ReplayBlock(block, statedb, newBlockParser(data))
	if err != nil {
		return nil, nil, err
	}
	return statedb, data, nil
}

func (idx *ChainIndexer) cleanUpAndStop() {
	idx.commitIndexData()
	close(idx.termCh)
	atomic.SwapUint32(&idx.status, uint32(task.StatusStopped))
	log.Info("Chain indexer stopped")
}

func (idx *ChainIndexer) commitIndexData() {
	if len(idx.indexData) == 0 {
		return
	}
	start := time.Now()
	batch := idx.diskdb.NewBatch()
	first := idx.indexData[0].block.NumberU64()
	last := idx.indexData[len(idx.indexData)-1].block.NumberU64()
	numAccount := 0
	for _, blockData := range idx.indexData {
		numAccount += len(blockData.dirtyAccounts)
		blockData.Commit(batch, true)
	}
	extdb.WriteLastIndexBlock(idx.diskdb, idx.lastBlock.Hash())
	if err := batch.Write(); err != nil {
		log.Crit("Faield to commit index data", "blocks", len(idx.indexData), "segment", []uint64{first, last}, "error", err)
	}
	numBlock := len(idx.indexData)
	elapsed := time.Since(start)
	dirty := common.StorageSize(batch.ValueSize())
	idx.indexData = make([]*blockIndexData, 0)
	log.Info("Persisted indexing data", "blocks", numBlock, "segment", []uint64{first, last}, "accounts", numAccount, "dirty", dirty, "elapsed", elapsed)
}

func (idx *ChainIndexer) indexingLoop() {
	defer idx.cleanUpAndStop()
	idx.indexData = make([]*blockIndexData, 0)
	proctime := time.Duration(0)
	var (
		statedb *state.StateDB
		data    *blockIndexData
		err     error
		retry   int
	)
	for {
		select {
		case <-idx.quitCh:
			return
		default:
		}
		block := idx.blockchain.GetBlockByNumber(idx.lastBlock.NumberU64() + 1)
		if block == nil {
			// block is not available, wait a second
			time.Sleep(time.Second)
			continue
		}
		log.Debug("Indexing block", "number", block.NumberU64())
		if statedb == nil {
			statedb, err = idx.replayer.StateAtBlock(idx.lastBlock)
			if err != nil {
				log.Error("Could not get historical state", "number", idx.lastBlock.NumberU64(), "root", idx.lastBlock.Root())
				return
			}
		}
		start := time.Now()
		statedb, data, err = idx.processBlock(block, statedb)
		if err != nil {
			// retry indexing block
			if retry += 1; retry < maxRetryIndexAttempt {
				log.Warn(fmt.Sprintf("Indexer could not process block, retry indexing (%d)", retry), "number", block.NumberU64(), "error", err)
				continue
			}
			log.Error("Indexer failed to process block", "number", block.NumberU64(), "error", err)
			return
		}
		proctime += time.Since(start)
		retry = 0
		idx.lastBlock = block
		idx.indexData = append(idx.indexData, data)
		if proctime > 10*time.Second {
			idx.commitIndexData()
			proctime = 0
		}
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			log.Warn(fmt.Sprintf("Indexing block %d tooks %v", block.NumberU64(), elapsed))
		}
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

	lastBlockHash := extdb.ReadLastIndexBlock(idx.diskdb)
	if lastBlockHash == nilHash {
		idx.lastBlock = idx.blockchain.GetBlockByNumber(0)
	} else {
		idx.lastBlock = idx.blockchain.GetBlockByHash(lastBlockHash)
	}

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

func (idx *ChainIndexer) Stop() {
	if atomic.LoadUint32(&idx.status) != uint32(task.StatusRunning) {
		return
	}
	close(idx.quitCh)
	idx.Wait()
}

func NewChainIndexer(diskdb ethdb.Database, stateCache state.Database, bc *core.BlockChain) (*ChainIndexer, error) {
	return &ChainIndexer{
		diskdb:     diskdb,
		indexdb:    NewIndexDB(diskdb, stateCache),
		blockchain: bc,
		replayer:   reexec.NewChainReplayer(stateCache, bc, 200000),
		pauseCh:    make(chan bool),
		termCh:     make(chan struct{}),
		quitCh:     make(chan struct{}),
	}, nil
}
