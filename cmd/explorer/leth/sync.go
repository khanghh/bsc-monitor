//
// Created on 2022/12/20 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2022 Verichains Lab
//

package leth

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	ethEventNewHeads               = "newHeads"
	ethEventNewPendingTransactions = "newPendingTransactions"
)

type SyncProgress struct {
	StartBlock   uint64
	CurrentBlock uint64
	HighestBlock uint64
}

// chainSyncer sync the local blockchain with the provided RPC node,
// also sync new pending transactions to local txpool
type chainSyncer struct {
	connector  *RpcConnector
	blockchain *core.BlockChain
	txpool     *core.TxPool
	downloader *Downloader
	wg         sync.WaitGroup
	quitCh     chan struct{}
}

// processNewHeads listen for new head from remote and sync local blockchain to the latest block height
func (s *chainSyncer) processNewHeads(wg *sync.WaitGroup) {
	defer wg.Done()
	headCh := make(chan *types.Header)
	listenNewHeads := func(rpcClient *rpc.Client) error {
		sub, err := rpcClient.EthSubscribe(context.Background(), headCh, ethEventNewHeads)
		if err != nil {
			return err
		}
		select {
		case <-s.quitCh:
			return nil
		case err := <-sub.Err():
			return err
		}
	}
	pollNewHeads := func(rpcClient *rpc.Client) error {
		client := ethclient.NewClient(rpcClient)
		latestBlock, err := client.BlockNumber(context.Background())
		if err != nil {
			return err
		}
		blockNum := int64(latestBlock)
		for {
			select {
			case <-s.quitCh:
				return nil
			default:
			}
			curHead, err := client.HeaderByNumber(context.Background(), big.NewInt(blockNum))
			if err == ethereum.NotFound {
				continue
			} else if err != nil {
				return err
			}
			headCh <- curHead
			blockNum++
			time.Sleep(500 * time.Millisecond)
		}
	}

	var rpcClient *rpc.Client
	go func() {
		defer close(headCh)
		var (
			notificationSupported bool  = true
			err                   error = nil
		)
		for ; ; time.Sleep(500 * time.Millisecond) {
			log.Info("Start process new heads event")
			rpcClient, _ = s.connector.Connect()
			if notificationSupported {
				err = listenNewHeads(rpcClient)
				if err == rpc.ErrNotificationsUnsupported || err == rpc.ErrSubscriptionNotFound {
					notificationSupported = false
					continue
				}
			} else {
				err = pollNewHeads(rpcClient)
			}
			if err != nil {
				log.Warn("Event subscription broken. Reconnect to rpc node...", "event", ethEventNewHeads, "error", err)
			} else {
				return
			}
		}
	}()

	for head := range headCh {
		parentBlock := s.blockchain.GetBlockByHash(head.ParentHash)
		if parentBlock != nil {
			s.downloader.ImportBlock(head.Hash())
			continue
		}
		if !s.downloader.Synchronising() {
			go s.downloader.StartSync(head.Number.Uint64())
		}
	}
}

// processNewPendingTxs listen and import pending transactions from RPC node to local txpool
func (s *chainSyncer) processNewPendingTxs(wg *sync.WaitGroup) {
	defer wg.Done()
	txHashCh := make(chan common.Hash)
	listenPendingTxs := func(rpcClient *rpc.Client) error {
		sub, err := rpcClient.EthSubscribe(context.Background(), txHashCh, ethEventNewPendingTransactions)
		if err != nil {
			return err
		}
		select {
		case <-s.quitCh:
			return nil
		case err := <-sub.Err():
			return err
		}
	}

	addTxPool := func(rpcClient *rpc.Client, txHash common.Hash) {
		if !s.downloader.Synchronising() {
			client := ethclient.NewClient(rpcClient)
			tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
			if err != nil {
				return
			}
			if isPending {
				s.txpool.AddRemotes(types.Transactions{tx})
			}
		}
	}

	var rpcClient *rpc.Client
	go func() {
		defer close(txHashCh)
		for ; ; time.Sleep(500 * time.Millisecond) {
			rpcClient, _ = s.connector.Connect()
			err := listenPendingTxs(rpcClient)
			if err == rpc.ErrNotificationsUnsupported || err == rpc.ErrSubscriptionNotFound {
				log.Warn("Could not sync txpool. RPC node subscription failed.", "event", ethEventNewPendingTransactions, "error", err)
				return
			}
			if err != nil {
				log.Warn("Event subscription broken. Reconnect to rpc node...", "event", ethEventNewPendingTransactions, "error", err)
			} else {
				return
			}
		}
	}()
	for txHash := range txHashCh {
		go addTxPool(rpcClient, txHash)
	}
}

// startSync starts it own goroutines to process events from remote automatically restarts when rpc connection closed
func (s *chainSyncer) runSync() {
	s.quitCh = make(chan struct{})
	s.wg.Add(2)
	go s.processNewHeads(&s.wg)
	go s.processNewPendingTxs(&s.wg)
	s.wg.Wait()
}

func (s *chainSyncer) stop() {
	close(s.quitCh)
	s.wg.Wait()
}

func newChainSyncer(connector *RpcConnector, blockchain *core.BlockChain, txpool *core.TxPool) *chainSyncer {
	return &chainSyncer{
		connector:  connector,
		blockchain: blockchain,
		txpool:     txpool,
		downloader: NewDownloader(connector, blockchain),
		wg:         sync.WaitGroup{},
	}
}
