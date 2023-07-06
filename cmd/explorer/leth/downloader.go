package leth

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

const (
	downloadBulkSize = 100              // Maximum length of chain segment to download
	downloadTimeout  = 10 * time.Second // Maximum duration for one fetching blocks process
)

var (
	errBusy              = errors.New("busy")
	errCanceled          = errors.New("canceled")
	errConnectionFailure = errors.New("connection failure")
)

// Downloader download and import blocks from RPC node
type Downloader struct {
	connector     *RpcConnector
	checkpoint    uint64
	chain         *core.BlockChain
	synchronising int32
	quitCh        chan struct{}
	quitLock      sync.Mutex
}

func (d *Downloader) fetchSegment(fromBlock, toBlock uint64) (types.Blocks, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	var (
		retErr error
		idx    uint64
	)
	total := toBlock - fromBlock + 1
	blocks := make(types.Blocks, total)
	wg := sync.WaitGroup{}
	client := ethclient.NewClient(d.connector.Client())
	for idx = 0; idx < total; idx++ {
		wg.Add(1)
		go func(idx uint64) {
			defer wg.Done()
			block, err := client.BlockByNumber(ctx, big.NewInt(int64(fromBlock+idx)))
			if err != nil {
				retErr = err
			}
			blocks[idx] = block
		}(idx)
	}
	wg.Wait()
	return blocks, retErr
}

// fetchBlocks fetchs the chain segment from the specific start block
// it fetchs as many blocks as possible in a limited time duration
func (d *Downloader) fetchBlocks(startNum uint64, endNum uint64) (types.Blocks, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	blocks := types.Blocks{}
	currentNum := startNum
	for {
		toNum := currentNum + downloadBulkSize - 1
		if toNum > endNum {
			toNum = endNum
		}
		segment, err := d.fetchSegment(currentNum, toNum)
		if err != nil {
			return blocks, err
		}
		blocks = append(blocks, segment...)
		currentNum = toNum + 1
		select {
		case <-ctx.Done():
			return blocks, nil
		default:
		}
	}
}

// ImportBlock download and import known remote block into local blockchain
func (d *Downloader) ImportBlock(hash common.Hash) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	block, err := ethclient.NewClient(d.connector.Client()).BlockByHash(ctx, hash)
	if err != nil {
		return err
	}
	if _, err := d.chain.InsertChain(types.Blocks{block}); err != nil {
		return err
	}
	return nil
}

// ImportSegment download and import known remote blockchain segment into local blockchain
func (d *Downloader) ImportSegment(fromBlock, toBlock uint64) error {
	total := toBlock - fromBlock + 1
	blocks, err := d.fetchSegment(fromBlock, toBlock)
	if err != nil {
		log.Warn("Failed to fetch blocks, reconnect to rpc node", "blocks", total, "from", fromBlock, "to", toBlock, "error", err)
		return errConnectionFailure
	}
	if _, err := d.chain.InsertChain(blocks); err != nil {
		return err
	}
	return nil
}

func (d *Downloader) CheckPoint() uint64 {
	return d.checkpoint
}

// Synchronising returns whether the downloader is currently retrieving blocks.
func (d *Downloader) Synchronising() bool {
	return atomic.LoadInt32(&d.synchronising) > 0
}

// StartSync start download and import chain segment into local blockchain until it reach the checkpoint block number
func (d *Downloader) StartSync(checkpoint uint64) error {
	// Make sure only one goroutine is ever allowed past this point at once
	if !atomic.CompareAndSwapInt32(&d.synchronising, 0, 1) {
		return errBusy
	}
	defer atomic.StoreInt32(&d.synchronising, 0)
	log.Info("Block synchronization started", "current", d.chain.CurrentBlock().NumberU64(), "checkpoint", checkpoint)
	d.checkpoint = checkpoint
	d.quitCh = make(chan struct{})
	for {
		startNum := d.chain.CurrentBlock().NumberU64() + 1
		startTime := time.Now()
		segment, err := d.fetchBlocks(startNum, d.checkpoint)
		if err != nil {
			log.Warn("Failed to fetch blocks", "error", err)
			continue
		}
		elapsed := time.Since(startTime)
		blockRange := []uint64{segment[0].NumberU64(), segment[len(segment)-1].NumberU64()}
		log.Info("Downloaded new chain segment", "blocks", len(segment), "segment", blockRange, "elapsed", elapsed)
		if _, err := d.chain.InsertChain(segment); err != nil {
			return err
		}
		if d.chain.CurrentBlock().NumberU64() >= d.checkpoint {
			log.Info("Block synchronization finished")
			return nil
		}
		select {
		case <-d.quitCh:
			return errCanceled
		default:
		}
	}
}

func (d *Downloader) Stop() {
	d.quitLock.Lock()
	select {
	case <-d.quitCh:
	default:
		atomic.StoreInt32(&d.synchronising, 0)
		d.chain.StopInsert()
		close(d.quitCh)
		d.checkpoint = 0
	}
	d.quitLock.Unlock()
}

func NewDownloader(connector *RpcConnector, chain *core.BlockChain) *Downloader {
	return &Downloader{
		connector: connector,
		chain:     chain,
		quitCh:    make(chan struct{}),
	}
}
