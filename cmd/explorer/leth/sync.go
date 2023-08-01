package leth

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

const (
	maxBlockFetch = 100 // Maximum amount of blocks to be fetched per retrieval request
)

var (
	errBusy         = errors.New("busy")
	errInvalidChain = errors.New("retrieved hash chain is invalid")
)

type ChainSyncer struct {
	chain *LightChain
	odr   OdrBackend

	// Statistics
	syncStatsChainOrigin uint64       // Origin block number where syncing started at
	syncStatsChainHeight uint64       // Highest block number known when syncing started
	syncStatsLock        sync.RWMutex // Lock protecting the sync stats fields

	running int32

	cancelLock sync.Mutex
	cancelCh   chan struct{}
	doneCh     chan struct{}
}

func (d *ChainSyncer) Progress() ethereum.SyncProgress {
	// Lock the current stats and return the progress
	d.syncStatsLock.RLock()
	defer d.syncStatsLock.RUnlock()
	current := d.chain.CurrentHeader().Number.Uint64()

	return ethereum.SyncProgress{
		StartingBlock: d.syncStatsChainOrigin,
		CurrentBlock:  current,
		HighestBlock:  d.syncStatsChainHeight,
	}
}

func (d *ChainSyncer) importChain(results []*types.Block) error {
	// Check for any early termination requests
	if len(results) == 0 {
		return nil
	}
	// Retrieve the a batch of results to import
	first, last := results[0], results[len(results)-1]
	log.Debug("Inserting downloaded chain", "items", len(results),
		"firstnum", first.Number, "firsthash", first.Hash(),
		"lastnum", last.Number, "lasthash", last.Hash(),
	)
	// Downloaded blocks are always regarded as trusted after the
	// transition. Because the downloaded chain is guided by the
	// consensus-layer.
	if index, err := d.chain.InsertChain(results); err != nil {
		if index < len(results) {
			log.Debug("Downloaded item processing failed", "number", results[index].NumberU64(), "hash", results[index].Hash(), "err", err)
		} else {
			// The InsertChain method in blockchain.go will sometimes return an out-of-bounds index,
			// when it needs to preprocess blocks to import a sidechain.
			// The importer will put together a new list of blocks to import, which is a superset
			// of the blocks delivered from the downloader, and the indexing will be off.
			log.Debug("Downloaded item processing failed on sidechain import", "index", index, "err", err)
		}
		if errors.Is(err, core.ErrAncestorHasNotBeenVerified) {
			return err
		}
		return fmt.Errorf("%w: %v", errInvalidChain, err)
	}
	return nil
}

func (s *ChainSyncer) syncLoop() {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}
	defer func() {
		close(s.doneCh)
		atomic.StoreInt32(&s.running, 0)
		log.Error("Block synchronization stopped")
	}()

	var (
		err          error
		remoteHeight uint64
		segment      types.Blocks
	)
	for ; ; time.Sleep(1 * time.Second) {
		select {
		case <-s.cancelCh:
			return
		default:
		}
		headBlock := s.chain.CurrentBlock()
		startNum := headBlock.NumberU64() + 1
		log.Info("Fetching chain segment from remote", "head", startNum)
		segment, remoteHeight, err = s.odr.GetChainSegment(startNum, maxBlockFetch)
		if err != nil {
			log.Error("Could not fetch chain segment", "head", headBlock.NumberU64(), "count", maxBlockFetch, "error", err)
			continue
		}
		if len(segment) > 0 {
			if segment[0].ParentHash() != headBlock.Hash() {
				log.Error("Remote return invalid chain segment, possible chain reorged", "head", headBlock.NumberU64(), "remoteHash", segment[0].Hash().Hex())
				return
			}
			s.syncStatsChainHeight = remoteHeight
			log.Info("Importing new chain segment", "blocks", len(segment))
			if err := s.importChain(segment); err != nil {
				log.Error("Could not import chain segment", "error", err)
				continue
			}
		}
	}
}

func (s *ChainSyncer) Start() error {
	if atomic.LoadInt32(&s.running) == 1 {
		return errBusy
	}
	log.Info("Block synchronization started")
	s.cancelCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	headBlock := s.chain.CurrentBlock()
	s.syncStatsChainOrigin = headBlock.NumberU64()
	go s.syncLoop()
	return nil
}

func (s *ChainSyncer) Stop() {
	s.cancelLock.Lock()
	defer s.cancelLock.Unlock()
	if s.cancelCh != nil {
		select {
		case <-s.cancelCh:
			return
		default:
			close(s.cancelCh)
		}
	}
	<-s.doneCh
}

func NewChainSyncer(odr OdrBackend, chain *LightChain) *ChainSyncer {
	return &ChainSyncer{
		odr:   odr,
		chain: chain,
	}
}
