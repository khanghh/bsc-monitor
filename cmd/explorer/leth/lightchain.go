package leth

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/syncx"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
)

var (
	errChainStopped = errors.New("blockchain is stopped")
)

// defaultCacheConfig are the default caching values if none are specified by the
// user (also used during testing).
var defaultCacheConfig = &core.CacheConfig{
	TrieCleanLimit: 256,
	TrieDirtyLimit: 256,
	TrieTimeLimit:  5 * time.Minute,
	SnapshotLimit:  256,
	TriesInMemory:  128,
	SnapshotWait:   true,
}

const (
	bodyCacheLimit         = 256
	blockCacheLimit        = 256
	diffLayerCacheLimit    = 1024
	diffLayerRLPCacheLimit = 256
	receiptsCacheLimit     = 10000
	txLookupCacheLimit     = 1024
	maxBadBlockLimit       = 16
	maxFutureBlocks        = 256
	maxTimeFutureBlocks    = 30
	maxBeyondBlocks        = 2048

	// BlockChainVersion ensures that an incompatible database forces a resync from scratch.
	//
	// Changelog:
	//
	// - Version 4
	//   The following incompatible database changes were added:
	//   * the `BlockNumber`, `TxHash`, `TxIndex`, `BlockHash` and `Index` fields of log are deleted
	//   * the `Bloom` field of receipt is deleted
	//   * the `BlockIndex` and `TxIndex` fields of txlookup are deleted
	// - Version 5
	//  The following incompatible database changes were added:
	//    * the `TxHash`, `GasCost`, and `ContractAddress` fields are no longer stored for a receipt
	//    * the `TxHash`, `GasCost`, and `ContractAddress` fields are computed by looking up the
	//      receipts' corresponding block
	// - Version 6
	//  The following incompatible database changes were added:
	//    * Transaction lookup information stores the corresponding block number instead of block hash
	// - Version 7
	//  The following incompatible database changes were added:
	//    * Use freezer as the ancient database to maintain all ancient data
	// - Version 8
	//  The following incompatible database changes were added:
	//    * New scheme for contract code in order to separate the codes and trie nodes
	BlockChainVersion uint64 = 8
)

type LightChain struct {
	// configurations
	chainConfig *params.ChainConfig // Chain & network configuration
	cacheConfig *core.CacheConfig   // Cache configuration for pruning
	vmConfig    vm.Config

	// data
	db    ethdb.Database
	odr   OdrBackend
	hc    *core.HeaderChain
	snaps *snapshot.Tree

	// states
	currentBlock          atomic.Value // Current head of the block chain
	highestVerifiedHeader atomic.Value
	genesisBlock          *types.Block

	// caches
	stateCache    state.Database // State database to reuse between imports (contains state cache)
	bodyCache     *lru.Cache     // Cache for the most recent block bodies
	bodyRLPCache  *lru.Cache     // Cache for the most recent block bodies in RLP encoded format
	blockCache    *lru.Cache     // Cache for the most recent entire blocks
	receiptsCache *lru.Cache     // Cache for the most recent receipts per block

	// block processing
	engine    consensus.Engine
	processor Processor
	validator Validator
	forker    *core.ForkChoice
	chainmu   *syncx.ClosableMutex

	// feeds
	rmLogsFeed          event.Feed
	chainFeed           event.Feed
	chainSideFeed       event.Feed
	chainHeadFeed       event.Feed
	chainBlockFeed      event.Feed
	logsFeed            event.Feed
	blockProcFeed       event.Feed
	finalizedHeaderFeed event.Feed
	scope               event.SubscriptionScope

	quitCh        chan struct{}  // blockchain quit channel
	wg            sync.WaitGroup // chain processing wait group for shutting down
	running       int32          // 0 if chain is running, 1 when stopped
	procInterrupt int32          // interrupt signaler for block processing
}

func (lc *LightChain) empty() bool {
	genesis := lc.genesisBlock.Hash()
	for _, hash := range []common.Hash{rawdb.ReadHeadBlockHash(lc.db), rawdb.ReadHeadHeaderHash(lc.db), rawdb.ReadHeadFastBlockHash(lc.db)} {
		if hash != genesis {
			return false
		}
	}
	return true
}

// insertStopped returns true after StopInsert has been called.
func (lc *LightChain) insertStopped() bool {
	return atomic.LoadInt32(&lc.procInterrupt) == 1
}

// skipBlock returns 'true', if the block being imported can be skipped over, meaning
// that the block does not need to be processed but can be considered already fully 'done'.
func (lc *LightChain) skipBlock(err error, it *insertIterator) bool {
	// We can only ever bypass processing if the only error returned by the validator
	// is ErrKnownBlock, which means all checks passed, but we already have the block
	// and state.
	if !errors.Is(err, ErrKnownBlock) {
		return false
	}
	// If we're not using snapshots, we can skip this, since we have both block
	// and (trie-) state
	if lc.snaps == nil {
		return true
	}
	var (
		header     = it.current() // header can't be nil
		parentRoot common.Hash
	)
	// If we also have the snapshot-state, we can skip the processing.
	if lc.snaps.Snapshot(header.Root) != nil {
		return true
	}
	// In this case, we have the trie-state but not snapshot-state. If the parent
	// snapshot-state exists, we need to process this in order to not get a gap
	// in the snapshot layers.
	// Resolve parent block
	if parent := it.previous(); parent != nil {
		parentRoot = parent.Root
	} else if parent = lc.GetHeaderByHash(header.ParentHash); parent != nil {
		parentRoot = parent.Root
	}
	if parentRoot == (common.Hash{}) {
		return false // Theoretically impossible case
	}
	// Parent is also missing snapshot: we can skip this. Otherwise process.
	if lc.snaps.Snapshot(parentRoot) == nil {
		return true
	}
	return false
}

func (lc *LightChain) updateHighestVerifiedHeader(header *types.Header) {
	if header == nil || header.Number == nil {
		return
	}
	currentHeader := lc.highestVerifiedHeader.Load().(*types.Header)
	if currentHeader == nil {
		lc.highestVerifiedHeader.Store(types.CopyHeader(header))
		return
	}

	newParentTD := lc.GetTd(header.ParentHash, header.Number.Uint64()-1)
	if newParentTD == nil {
		newParentTD = big.NewInt(0)
	}
	oldParentTD := lc.GetTd(currentHeader.ParentHash, currentHeader.Number.Uint64()-1)
	if oldParentTD == nil {
		oldParentTD = big.NewInt(0)
	}
	newTD := big.NewInt(0).Add(newParentTD, header.Difficulty)
	oldTD := big.NewInt(0).Add(oldParentTD, currentHeader.Difficulty)

	if newTD.Cmp(oldTD) > 0 {
		lc.highestVerifiedHeader.Store(types.CopyHeader(header))
		return
	}
}

// reportBadBlock logs a bad block error.
func (lc *LightChain) reportBadBlock(block *types.Block, receipts types.Receipts, err error) {
	rawdb.WriteBadBlock(lc.db, block)

	var receiptString string
	for i, receipt := range receipts {
		receiptString += fmt.Sprintf("\t %d: cumulative: %v gas: %v contract: %v status: %v tx: %v logs: %v bloom: %x state: %x\n",
			i, receipt.CumulativeGasUsed, receipt.GasUsed, receipt.ContractAddress.Hex(),
			receipt.Status, receipt.TxHash.Hex(), receipt.Logs, receipt.Bloom, receipt.PostState)
	}
	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Chain config: %v

Number: %v
Hash: 0x%x
%v

Error: %v
##############################
`, lc.chainConfig, block.Number(), block.Hash(), receiptString, err))
}

func (lc *LightChain) writeHeadWithState(block *types.Block, receipts []*types.Receipt, logs []*types.Log, state *state.StateDB) error {
	return nil
}

func (lc *LightChain) insertChain(chain types.Blocks, verifySeals, setHead bool) (int, error) {
	stats := insertStats{startTime: mclock.Now()}
	headers := make([]*types.Header, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.Header()
		seals[i] = verifySeals
	}
	abort, results := lc.engine.VerifyHeaders(lc, headers, seals)
	defer close(abort)
	it := newInsertIterator(chain, results, lc.validator)
	block, err := it.next()
	for ; block != nil && err == nil || errors.Is(err, ErrKnownBlock); block, err = it.next() {
		if lc.skipBlock(err, it) {
			start := time.Now()
			parent := it.previous()
			if parent == nil {
				parent = lc.GetHeader(block.ParentHash(), block.NumberU64()-1)
			}
			statedb, err := state.NewWithSharedPool(parent.Root, lc.stateCache, lc.snaps)
			if err != nil {
				return it.index, err
			}
			lc.updateHighestVerifiedHeader(block.Header())
			statedb.StartPrefetcher("chain")
			statedb.SetExpectedStateRoot(block.Root())
			statedb, receipts, logs, usedGas, err := lc.processor.Process(block, statedb, lc.vmConfig)
			if err != nil {
				lc.reportBadBlock(block, receipts, err)
				return it.index, err
			}
			lc.cacheReceipts(block.Hash(), receipts)
			lc.cacheBlock(block.Hash(), block)
			proctime := time.Since(start)
			if proctime > time.Second {
				log.Warn(fmt.Sprintf("Processing block took %dms", proctime.Milliseconds()), "block", block.NumberU64())
			}
			// Write the block to the chain and get the status.
			err = lc.writeHeadWithState(block, receipts, logs, statedb)
			if err != nil {
				return it.index, err
			}
			stats.processed++
			stats.usedGas += usedGas
			lc.chainBlockFeed.Send(core.ChainHeadEvent{block})
			dirty, _ := lc.stateCache.TrieDB().Size()
			stats.report(chain, it.index, dirty)
		}
	}
	return it.index, nil
}

func (lc *LightChain) cacheReceipts(hash common.Hash, receipts types.Receipts) {
	// TODO, This is a hot fix for the block hash of logs is `0x0000000000000000000000000000000000000000000000000000000000000000` for system tx
	// Please check details in https://github.com/binance-chain/bsc/issues/443
	// This is a temporary fix, the official fix should be a hard fork.
	const possibleSystemReceipts = 3 // One slash tx, two reward distribute txs.
	numOfReceipts := len(receipts)
	for i := numOfReceipts - 1; i >= 0 && i >= numOfReceipts-possibleSystemReceipts; i-- {
		for j := 0; j < len(receipts[i].Logs); j++ {
			receipts[i].Logs[j].BlockHash = hash
		}
	}
	lc.receiptsCache.Add(hash, receipts)
}

func (lc *LightChain) cacheBlock(hash common.Hash, block *types.Block) {
	lc.blockCache.Add(hash, block)
}

// writeHeadBlock injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *LightChain) writeHeadBlock(block *types.Block) {
	// Add the block to the canonical chain number scheme and mark as the head
	batch := bc.db.NewBatch()
	rawdb.WriteHeadHeaderHash(batch, block.Hash())
	rawdb.WriteHeadFastBlockHash(batch, block.Hash())
	rawdb.WriteCanonicalHash(batch, block.Hash(), block.NumberU64())
	rawdb.WriteTxLookupEntriesByBlock(batch, block)
	rawdb.WriteHeadBlockHash(batch, block.Hash())

	// Flush the whole batch into the disk, exit the node if failed
	if err := batch.Write(); err != nil {
		log.Crit("Failed to update chain indexes and markers", "err", err)
	}
	// Update all in-memory chain markers in the last step
	bc.hc.SetCurrentHeader(block.Header())
	bc.currentBlock.Store(block)
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *LightChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}

	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)
	headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64())
	blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	log.Info("Loaded most recent local header", "number", currentHeader.Number, "hash", currentHeader.Hash(), "td", headerTd, "age", common.PrettyAge(time.Unix(int64(currentHeader.Time), 0)))
	log.Info("Loaded most recent local full block", "number", currentBlock.Number(), "hash", currentBlock.Hash(), "td", blockTd, "age", common.PrettyAge(time.Unix(int64(currentBlock.Time()), 0)))
	return nil
}

func (bc *LightChain) setHeadBeyondRoot(head uint64, root common.Hash, repair bool) (uint64, error) {
	// Track the block number of the requested root hash
	var rootNumber uint64 // (no root == always 0)

	// Retrieve the last pivot block to short circuit rollbacks beyond it and the
	// current freezer limit to start nuking id underflown
	pivot := rawdb.ReadLastPivotNumber(bc.db)
	frozen, _ := bc.db.Ancients()

	updateFn := func(db ethdb.KeyValueWriter, header *types.Header) (uint64, bool) {
		// Rewind the blockchain, ensuring we don't end up with a stateless head
		// block. Note, depth equality is permitted to allow using SetHead as a
		// chain reparation mechanism without deleting any data!
		if currentBlock := bc.CurrentBlock(); currentBlock != nil && header.Number.Uint64() <= currentBlock.NumberU64() {
			newHeadBlock := bc.GetBlock(header.Hash(), header.Number.Uint64())
			lastBlockNum := header.Number.Uint64()
			if newHeadBlock == nil {
				log.Error("Gap in the chain, rewinding to genesis", "number", header.Number, "hash", header.Hash())
				newHeadBlock = bc.genesisBlock
			} else {
				// Block exists, keep rewinding until we find one with state,
				// keeping rewinding until we exceed the optional threshold
				// root hash
				beyondRoot := (root == common.Hash{}) // Flag whether we're beyond the requested root (no root, always true)
				enoughBeyondCount := false
				beyondCount := 0
				for {
					beyondCount++
					// If a root threshold was requested but not yet crossed, check
					if root != (common.Hash{}) && !beyondRoot && newHeadBlock.Root() == root {
						beyondRoot, rootNumber = true, newHeadBlock.NumberU64()
					}

					enoughBeyondCount = beyondCount > maxBeyondBlocks

					if _, err := state.New(newHeadBlock.Root(), bc.stateCache, bc.snaps); err != nil {
						log.Trace("Block state missing, rewinding further", "number", newHeadBlock.NumberU64(), "hash", newHeadBlock.Hash())
						if pivot == nil || newHeadBlock.NumberU64() > *pivot {
							parent := bc.GetBlock(newHeadBlock.ParentHash(), newHeadBlock.NumberU64()-1)
							if parent != nil {
								newHeadBlock = parent
								continue
							}
							log.Error("Missing block in the middle, aiming genesis", "number", newHeadBlock.NumberU64()-1, "hash", newHeadBlock.ParentHash())
							newHeadBlock = bc.genesisBlock
						} else {
							log.Trace("Rewind passed pivot, aiming genesis", "number", newHeadBlock.NumberU64(), "hash", newHeadBlock.Hash(), "pivot", *pivot)
							newHeadBlock = bc.genesisBlock
						}
					}
					if beyondRoot || (enoughBeyondCount && root != common.Hash{}) || newHeadBlock.NumberU64() == 0 {
						if enoughBeyondCount && (root != common.Hash{}) && rootNumber == 0 {
							for {
								lastBlockNum++
								block := bc.GetBlockByNumber(lastBlockNum)
								if block == nil {
									break
								}
								if block.Root() == root {
									rootNumber = block.NumberU64()
									break
								}
							}
						}
						log.Debug("Rewound to block with state", "number", newHeadBlock.NumberU64(), "hash", newHeadBlock.Hash())
						break
					}
					log.Debug("Skipping block with threshold state", "number", newHeadBlock.NumberU64(), "hash", newHeadBlock.Hash(), "root", newHeadBlock.Root())
					newHeadBlock = bc.GetBlock(newHeadBlock.ParentHash(), newHeadBlock.NumberU64()-1) // Keep rewinding
				}
			}
			rawdb.WriteHeadBlockHash(db, newHeadBlock.Hash())

			// Degrade the chain markers if they are explicitly reverted.
			// In theory we should update all in-memory markers in the
			// last step, however the direction of SetHead is from high
			// to low, so it's safe to update in-memory markers directly.
			bc.currentBlock.Store(newHeadBlock)
		}
		head := bc.CurrentBlock().NumberU64()

		// If setHead underflown the freezer threshold and the block processing
		// intent afterwards is full block importing, delete the chain segment
		// between the stateful-block and the sethead target.
		var wipe bool
		if head+1 < frozen {
			wipe = pivot == nil || head >= *pivot
		}
		return head, wipe // Only force wipe if full synced
	}
	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db ethdb.KeyValueWriter, hash common.Hash, num uint64) {
		// Ignore the error here since light client won't hit this path
		frozen, _ := bc.db.Ancients()
		if num+1 <= frozen {
			// Truncate all relative data(header, total difficulty, body, receipt
			// and canonical hash) from ancient store.
			if err := bc.db.TruncateAncients(num); err != nil {
				log.Crit("Failed to truncate ancient data", "number", num, "err", err)
			}
			// Remove the hash <-> number mapping from the active store.
			rawdb.DeleteHeaderNumber(db, hash)
		} else {
			// Remove relative body and receipts from the active store.
			// The header, total difficulty and canonical hash will be
			// removed in the hc.SetHead function.
			rawdb.DeleteBody(db, hash, num)
			rawdb.DeleteReceipts(db, hash, num)
		}
		// Todo(rjl493456442) txlookup, bloombits, etc
	}
	// If SetHead was only called as a chain reparation method, try to skip
	// touching the header chain altogether, unless the freezer is broken
	if repair {
		if target, force := updateFn(bc.db, bc.CurrentBlock().Header()); force {
			bc.hc.SetHead(target, updateFn, delFn)
		}
	} else {
		// Rewind the chain to the requested head and keep going backwards until a
		// block with a state is found or fast sync pivot is passed
		log.Warn("Rewinding blockchain", "target", head)
		bc.hc.SetHead(head, updateFn, delFn)
	}
	// Clear out any stale content from the caches
	bc.bodyCache.Purge()
	bc.bodyRLPCache.Purge()
	bc.receiptsCache.Purge()
	bc.blockCache.Purge()

	return rootNumber, bc.loadLastState()
}

func (bc *LightChain) SetHead(head uint64) error {
	return nil
}

func (lc *LightChain) InsertHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	if len(chain) == 0 {
		return 0, nil
	}
	start := time.Now()
	if i, err := lc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

	if !lc.chainmu.TryLock() {
		return 0, errChainStopped
	}
	defer lc.chainmu.Unlock()
	_, err := lc.hc.InsertHeaderChain(chain, start, lc.forker)
	return 0, err
}

// InsertChain attempts to process chain segment and insert its headers to the canonical
// header chain or, otherwise, create a fork. If an error is returned it will return
// the index number of the failing block as well an error describing what went wrong.
func (lc *LightChain) InsertChain(chain types.Blocks) (int, error) {
	if len(chain) == 0 {
		return 0, nil
	}
	lc.blockProcFeed.Send(true)
	defer lc.blockProcFeed.Send(false)

	// Do a sanity check that the provided chain is actually ordered and linked.
	for i := 1; i < len(chain); i++ {
		block, prev := chain[i], chain[i-1]
		if block.NumberU64() != prev.NumberU64()+1 || block.ParentHash() != prev.Hash() {
			log.Error("Non contiguous block insert",
				"number", block.Number(),
				"hash", block.Hash(),
				"parent", block.ParentHash(),
				"prevnumber", prev.Number(),
				"prevhash", prev.Hash(),
			)
			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x..], item %d is #%d [%x..] (parent [%x..])", i-1, prev.NumberU64(),
				prev.Hash().Bytes()[:4], i, block.NumberU64(), block.Hash().Bytes()[:4], block.ParentHash().Bytes()[:4])
		}
	}
	if !lc.chainmu.TryLock() {
		return 0, errChainStopped
	}
	defer lc.chainmu.Unlock()
	return lc.insertChain(chain, true, true)
}

func (bc *LightChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}

	// Unsubscribe all subscriptions registered from blockchain.
	bc.scope.Close()

	// Signal shutdown to all goroutines.
	close(bc.quitCh)
	// bc.StopInsert()

	// Now wait for all chain modifications to end and persistent goroutines to exit.
	//
	// Note: Close waits for the mutex to become available, i.e. any running chain
	// modification will have exited when Close returns. Since we also called StopInsert,
	// the mutex should become available quickly. It cannot be taken again after Close has
	// returned.
	bc.chainmu.Close()
	bc.wg.Wait()

	// Ensure that the entirety of the state snapshot is journalled to disk.
	snapBase, err := bc.snaps.Journal(bc.CurrentBlock().Root())
	if err != nil {
		log.Error("Failed to journal state snapshot", "err", err)
	}

	triedb := bc.stateCache.TrieDB()

	log.Info("Writing snapshot state to disk", "root", snapBase)
	if err := triedb.Commit(snapBase, true, nil); err != nil {
		log.Error("Failed to commit recent state trie", "err", err)
	} else {
		rawdb.WriteSafePointBlockNumber(bc.db, bc.CurrentBlock().NumberU64())
	}

	if size, _ := triedb.Size(); size != 0 {
		log.Error("Dangling trie nodes after full cleanup")
	}
	// Ensure all live cached entries be saved into disk, so that we can skip
	// cache warmup when node restarts.
	if bc.cacheConfig.TrieCleanJournal != "" {
		triedb := bc.stateCache.TrieDB()
		triedb.SaveCache(bc.cacheConfig.TrieCleanJournal)
	}
	log.Info("Blockchain stopped")
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *LightChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *LightChain) ResetWithGenesisBlock(genesis *types.Block) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	if !bc.chainmu.TryLock() {
		return errChainStopped
	}
	defer bc.chainmu.Unlock()

	// Prepare the genesis block and reinitialise the chain
	batch := bc.db.NewBatch()
	rawdb.WriteTd(batch, genesis.Hash(), genesis.NumberU64(), genesis.Difficulty())
	rawdb.WriteBlock(batch, genesis)
	if err := batch.Write(); err != nil {
		log.Crit("Failed to write genesis block", "err", err)
	}
	bc.writeHeadBlock(genesis)

	// Last update all in-memory chain markers
	bc.genesisBlock = genesis
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
	return nil
}

func NewLightChain(odr OdrBackend, db ethdb.Database, cacheConfig *core.CacheConfig, chainConfig *params.ChainConfig, engine consensus.Engine, vmConfig vm.Config) (*LightChain, error) {
	if cacheConfig == nil {
		cacheConfig = defaultCacheConfig
	}
	if cacheConfig.TriesInMemory != 128 {
		log.Warn("TriesInMemory isn't the default value(128), you need specify exact same TriesInMemory when prune data",
			"triesInMemory", cacheConfig.TriesInMemory)
	}
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	lc := &LightChain{
		chainConfig: chainConfig,
		cacheConfig: cacheConfig,
		db:          db,
		odr:         odr,
		stateCache: state.NewDatabaseWithConfigAndCache(db, &trie.Config{
			Cache:     cacheConfig.TrieCleanLimit,
			Journal:   cacheConfig.TrieCleanJournal,
			Preimages: cacheConfig.Preimages,
			NoTries:   cacheConfig.NoTries,
		}),
		bodyCache:     bodyCache,
		bodyRLPCache:  bodyRLPCache,
		receiptsCache: receiptsCache,
		blockCache:    blockCache,
		engine:        engine,
		vmConfig:      vmConfig,
		quitCh:        make(chan struct{}),
		chainmu:       syncx.NewClosableMutex(),
	}
	lc.forker = core.NewForkChoice(lc, nil)
	lc.validator = NewBlockValidator(chainConfig, lc, engine)
	lc.processor = NewLightProcessor(chainConfig, lc, engine)

	var err error
	lc.hc, err = core.NewHeaderChain(db, chainConfig, engine, lc.insertStopped)
	if err != nil {
		return nil, err
	}

	lc.genesisBlock = lc.GetBlockByNumber(0)
	if lc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	var nilBlock *types.Block
	lc.currentBlock.Store(nilBlock)

	var nilHeader *types.Header
	lc.highestVerifiedHeader.Store(nilHeader)
	if err := lc.loadLastState(); err != nil {
		return nil, err
	}

	// Make sure the state associated with the block is available
	head := lc.CurrentBlock()
	if _, err := state.New(head.Root(), lc.stateCache, lc.snaps); err != nil {
		// Head state is missing, before the state recovery, find out the
		// disk layer point of snapshot(if it's enabled). Make sure the
		// rewound point is lower than disk layer.
		var diskRoot common.Hash
		if lc.cacheConfig.SnapshotLimit > 0 {
			diskRoot = rawdb.ReadSnapshotRoot(lc.db)
		}
		if diskRoot != (common.Hash{}) {
			log.Warn("Head state missing rewind to nearest checkpoint", "number", head.Number(), "hash", head.Hash(), "snaproot", diskRoot)
			snapDisk, err := lc.setHeadBeyondRoot(head.NumberU64(), diskRoot, true)
			if err != nil {
				return nil, err
			}
			// Chain rewound, persist old snapshot number to indicate recovery procedure
			if snapDisk != 0 {
				rawdb.WriteSnapshotRecoveryNumber(lc.db, snapDisk)
			}
		}
	}
	if err := lc.engine.VerifyHeader(lc, lc.CurrentHeader(), true); err != nil {
		return nil, err
	}

	// Check the current state of the block hashes and make sure that we do not have any of the bad blocks in our chain
	for hash := range core.BadHashes {
		if header := lc.GetHeaderByHash(hash); header != nil {
			// get the canonical block corresponding to the offending header's number
			headerByNumber := lc.GetHeaderByNumber(header.Number.Uint64())
			// make sure the headerByNumber (if present) is in our current canonical chain
			if headerByNumber != nil && headerByNumber.Hash() == header.Hash() {
				log.Error("Found bad hash, rewinding chain", "number", header.Number, "hash", header.ParentHash)
				if err := lc.SetHead(header.Number.Uint64() - 1); err != nil {
					return nil, err
				}
				log.Error("Chain rewind was successful, resuming normal operation")
			}
		}
	}

	// Load any existing snapshot, regenerating it if loading failed
	if lc.cacheConfig.SnapshotLimit > 0 {
		// If the chain was rewound past the snapshot persistent layer (causing
		// a recovery block number to be persisted to disk), check if we're still
		// in recovery mode and in that case, don't invalidate the snapshot on a
		// head mismatch.
		var recover bool
		head := lc.CurrentBlock()
		if layer := rawdb.ReadSnapshotRecoveryNumber(lc.db); layer != nil && *layer > head.NumberU64() {
			log.Warn("Enabling snapshot recovery", "chainhead", head.NumberU64(), "diskbase", *layer)
			recover = true
		}
		lc.snaps, _ = snapshot.New(lc.db, lc.stateCache.TrieDB(), lc.cacheConfig.SnapshotLimit, int(lc.cacheConfig.TriesInMemory), head.Root(), !lc.cacheConfig.SnapshotWait, true, recover, lc.stateCache.NoTries())
	}
	rawdb.WriteSafePointBlockNumber(lc.db, lc.CurrentBlock().NumberU64())
	return lc, nil
}
