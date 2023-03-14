package monitor

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/cmd/gethext/service/task"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

type ChainIndexer struct {
	diskdb     ethdb.Database
	indexdb    *IndexDB
	blockchain *core.BlockChain

	lastBlock    *types.Block
	commitBlock  *types.Block
	blockCh      chan *types.Block
	pendingState *stateObject

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

func (idx *ChainIndexer) newTrieDiffNodeIterator(oldRoot, newRoot common.Hash) (*trie.Iterator, error) {
	oldTrie, err := idx.indexdb.trieCache.OpenTrie(oldRoot)
	if err != nil {
		return nil, err
	}
	newTrie, err := idx.indexdb.trieCache.OpenTrie(newRoot)
	if err != nil {
		return nil, err
	}
	diff, _ := trie.NewDifferenceIterator(oldTrie.NodeIterator([]byte{}), newTrie.NodeIterator([]byte{}))
	iter := trie.NewIterator(diff)
	return iter, nil
}

func (idx *ChainIndexer) commitChange(stateObj *stateObject, block *types.Block) (*stateObject, error) {
	if err := stateObj.Commit(block.Root()); err != nil {
		return nil, err
	}
	extdb.WriteLastIndexBlock(idx.diskdb, block.Hash())
	extdb.WriteIndexStateRoot(idx.diskdb, block.Root())
	newState := newStateObject(idx.indexdb, block.Root())
	newState.accounts = stateObj.dirtyAccounts
	for addr, oldChange := range stateObj.dirtyChange {
		newState.accStates[addr] = oldChange.IndexState
	}
	return newState, nil
}

// processBlock re-executes every transactions in block and extracts neccessary info into indexdb
func (idx *ChainIndexer) processBlock(block *types.Block) error {
	if block.Root() == idx.lastBlock.Root() {
		log.Debug("Ignore block, no need indexing", "block", block.NumberU64(), "root", block.Root())
		return nil
	}
	stateObj := idx.pendingState
	var sender, receiver *AccountDetail
	txs := block.Transactions()
	receipts := idx.blockchain.GetReceiptsByHash(block.Hash())
	signer := types.MakeSigner(idx.blockchain.Config(), block.Number())
	for txIndex, tx := range txs {
		msg, _ := tx.AsMessage(signer, block.BaseFee())
		if tx.Nonce() == 0 {
			sender = stateObj.SetAccountDetail(msg.From(), &AccountInfo{FirstTx: tx.Hash()}, nil)
			log.Warn(fmt.Sprintf("Add new account %s", sender.Address), "tx", tx.Hash().Hex())
		}
		stateObj.AccountIndex(msg.From()).AddSentTx(tx.Hash())
		if tx.To() == nil {
			receipt := receipts[txIndex]
			if receipt.ContractAddress != nilAddress {
				receiver = stateObj.SetAccountDetail(
					receipt.ContractAddress,
					&AccountInfo{FirstTx: tx.Hash()},
					&ContractInfo{Creator: msg.From()},
				)
				stateObj.AccountIndex(receiver.Address).AddInternalTx(tx.Hash())
				log.Warn(fmt.Sprintf("Add new contract %s", receiver.Address), "tx", tx.Hash().Hex())
			}
		}
	}
	// TODO: iterate over modified nodes of new trie to index full contracts
	if _, err := idx.indexdb.trieCache.OpenTrie(block.Root()); err != nil {
		log.Debug("Missing trie node, continue indexing on next state", "missing", block.Root())
	} else if len(idx.pendingState.DirtyAccounts()) > 0 {
		start := time.Now()
		log.Info("Commiting index data", "number", block.NumberU64(), "root", block.Root())
		newState, err := idx.commitChange(stateObj, block)
		if err != nil {
			log.Error("Could not commit indexing state", "root", block.Root(), "error", err)
			return err
		}
		elapsed := time.Since(start)
		log.Info("Persisted indexing data", "accounts", len(stateObj.DirtyAccounts()), "number", block.NumberU64(), "root", block.Root(), "elapsed", elapsed, "dirty")
		idx.commitBlock = block
		idx.pendingState = newState
	}
	return nil
}

func (idx *ChainIndexer) cleanUpAndStop() {
	if idx.commitBlock != nil && idx.pendingState != nil {
		extdb.WriteLastIndexBlock(idx.diskdb, idx.commitBlock.Hash())
		extdb.WriteIndexStateRoot(idx.diskdb, idx.pendingState.origin)
	}
	close(idx.termCh)
	atomic.SwapUint32(&idx.status, uint32(task.StatusStopped))
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
				if block != nil {
					idx.blockCh <- block
					continue
				}
			}
			time.Sleep(time.Second)
		}
	}()
	defer idx.cleanUpAndStop()
	proctime := time.Duration(0)
	for block := range idx.blockCh {
		start := time.Now()
		log.Debug("Indexing block", "number", block.Number())
		if err := idx.processBlock(block); err != nil {
			log.Error("Indexer could not process block", "number", block.NumberU64())
			return
		}
		proctime += time.Since(start)
		idx.lastBlock = block
		if proctime > 10*time.Second {
			proctime = 0
			log.Info("Indexing progress", "number", idx.lastBlock.NumberU64(), "accounts", len(idx.pendingState.DirtyAccounts()))
		}
	}
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
	if !atomic.CompareAndSwapUint32(&idx.status, status, uint32(task.StatusRunning)) {
		return
	}
	idx.lastBlock = lastBlock
	idx.pendingState = newStateObject(idx.indexdb, idx.lastBlock.Root())
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

func NewChainIndexer(diskdb ethdb.Database, bc *core.BlockChain) (*ChainIndexer, error) {
	indexdb := NewIndexDB(diskdb, bc.StateCache())
	return &ChainIndexer{
		diskdb:     diskdb,
		indexdb:    indexdb,
		blockchain: bc,
		blockCh:    make(chan *types.Block),
		pauseCh:    make(chan bool),
		termCh:     make(chan struct{}),
		quitCh:     make(chan struct{}),
	}, nil
}
