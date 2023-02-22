package monitor

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/gethext/model"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
)

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain

	newBlockSubs     event.Subscription
	newBlocksEventCh chan core.NewMinedBlockEvent
	root             common.Hash

	status uint32
	mtx    sync.Mutex
	quitCh chan struct{}
}

func (idx *ChainIndexer) Status() task.TaskStatus {
	return task.TaskStatus(idx.status)
}

func (idx *ChainIndexer) GetAccount(addr common.Address) (*model.Account, error) {
	return idx.GetAccountAt(idx.blockchain.CurrentBlock().Root(), addr)
}

// TODO: implement transaction decoder
func (s *ChainIndexer) ProcessBlock(block *types.Block) error {
	return nil
}

// GetAccountAt returns account and its state at specific state root
func (s *ChainIndexer) GetAccountAt(root common.Hash, addr common.Address) (*model.Account, error) {
	return nil, nil
}

func (idx *ChainIndexer) Run() {
	// Check the current task status before running
	status := atomic.LoadUint32(&idx.status)
	if status != uint32(task.StatusPending) && status != uint32(task.StatusPaused) {
		return
	}
	// Try to update the status from pending or paused to preparing
	if atomic.CompareAndSwapUint32(&idx.status, status, uint32(task.StatusRunning)) {
		return
	}

	lastBlock := idx.blockchain.GetBlockByHash(extdb.ReadLastIndexBlock(idx.diskdb))
	idx.root = lastBlock.Root()
}

func (idx *ChainIndexer) Wait() {
	<-idx.quitCh
}

func (idx *ChainIndexer) Pause() {
	// Try to update the status from running to paused
	if atomic.CompareAndSwapUint32(&idx.status, uint32(task.StatusRunning), uint32(task.StatusPaused)) {
		return
	}
}

func (idx *ChainIndexer) Resume() {
	if atomic.CompareAndSwapUint32(&idx.status, uint32(task.StatusPaused), uint32(task.StatusRunning)) {
		return
	}
}

func (idx *ChainIndexer) Abort() {
}

func (idx *ChainIndexer) Stop() {
	idx.Wait()
	atomic.SwapUint32(&idx.status, uint32(task.StatusStopped))
}

func NewChainIndexer(diskdb ethdb.Database, bc *core.BlockChain) (*ChainIndexer, error) {
	indexdb := NewIndexDB(diskdb, bc.StateCache().TrieDB())
	return &ChainIndexer{
		diskdb:     diskdb,
		indexdb:    indexdb,
		blockchain: bc,
	}, nil
}
