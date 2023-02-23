package monitor

import (
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/gethext/model"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain

	lastBlock *types.Block
	blockCh   chan *types.Block
	stateRoot common.Hash

	status  uint32
	pauseCh chan bool
	termCh  chan struct{}
	quitCh  chan struct{}
}

func (idx *ChainIndexer) Database() *IndexDB {
	return idx.indexdb
}

func (idx *ChainIndexer) Status() task.TaskStatus {
	return task.TaskStatus(idx.status)
}

func (idx *ChainIndexer) GetAccount(addr common.Address) (*model.Account, error) {
	return idx.GetAccountAt(idx.blockchain.CurrentBlock().Root(), addr)
}

// processBlock generate statedb at the given block, re-execute every transaction and extract neccessary info into indexdb
func (idx *ChainIndexer) processBlock(block *types.Block) error {
	accStates := make(map[common.Address]interface{})
	getAccState := func(addr common.Address) *AccountIndexState {
		accState, exist := accStates[addr]
		if !exist {
			accState := new(AccountIndexState)
			idx.indexdb.AccountExtState(idx.stateRoot, addr, accState)
			accStates[addr] = accState
		}
		return accState.(*AccountIndexState)
	}
	batch := idx.diskdb.NewBatch()
	signer := types.MakeSigner(idx.blockchain.Config(), block.Number())
	txs := block.Transactions()
	for _, tx := range txs {
		msg, _ := tx.AsMessage(signer, block.BaseFee())
		sender := msg.From()
		if _, err := idx.indexdb.AccountInfo(sender); err != nil {
			accInfo := &model.AccountInfo{
				Address: sender,
				FirstTx: tx.Hash(),
			}
			enc, _ := rlp.EncodeToBytes(accInfo)
			extdb.WriteAccountInfo(batch, sender, enc)
		}
		extdb.WriteAccountSentTx(batch, sender, tx.Hash(), tx.Nonce())
		senderState := getAccState(sender)
		senderState.SentTxCount += 1
	}
	idx.indexdb.UpdateExtState(block.Root(), accStates)
	return batch.Write()
}

func (idx *ChainIndexer) indexingLoop() {
	waitResume := func() {
		for paused := range idx.pauseCh {
			if !paused {
				return
			}
		}
	}
	go func() {
		for {
			select {
			case paused := <-idx.pauseCh:
				if !paused {
					waitResume()
				}
			case <-idx.quitCh:
				close(idx.blockCh)
				return
			default:
			}
			blockNum := idx.lastBlock.NumberU64() + 1
			if blockNum <= idx.blockchain.CurrentBlock().NumberU64() {
				block := idx.blockchain.GetBlockByNumber(blockNum)
				idx.blockCh <- block
				idx.lastBlock = block
			} else {
				time.Sleep(time.Second)
			}
		}
	}()
	defer close(idx.termCh)
	for block := range idx.blockCh {
		log.Debug("Indexing block", "number", block.Number())
		if err := idx.processBlock(block); err != nil {
			log.Error("Indexer could not process block", "number", block.NumberU64())
		}
	}
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
	var lastBlock *types.Block
	lastBlockHash := extdb.ReadLastIndexBlock(idx.diskdb)
	if lastBlockHash == nilHash {
		lastBlock = idx.blockchain.GetBlockByNumber(0)
	} else {
		lastBlock = idx.blockchain.GetBlockByHash(lastBlockHash)
	}

	// Try to update the status from pending or paused to preparing
	if atomic.CompareAndSwapUint32(&idx.status, status, uint32(task.StatusRunning)) {
		return
	}
	idx.lastBlock = lastBlock
	idx.stateRoot = lastBlock.Root()
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
