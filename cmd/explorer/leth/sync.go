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
	ethEventNewHeads = "newHeads"
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
			notificationSupported bool
			err                   error
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
			s.importBlock(head.Hash())
			continue
		}
	}
}

func (s *chainSyncer) importBlock(hash common.Hash) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	block, err := ethclient.NewClient(s.connector.Client()).BlockByHash(ctx, hash)
	if err != nil {
		return err
	}
	if _, err := s.blockchain.InsertChain(types.Blocks{block}); err != nil {
		return err
	}
	return nil
}

// startSync starts it own goroutines to process events from remote automatically restarts when rpc connection closed
func (s *chainSyncer) runSync() {
	s.quitCh = make(chan struct{})
	s.wg.Add(2)
	go s.processNewHeads(&s.wg)
	s.wg.Wait()
}

func (s *chainSyncer) stop() {
	close(s.quitCh)
	s.wg.Wait()
}

func newChainSyncer(connector *RpcConnector, blockchain *core.BlockChain) *chainSyncer {
	return &chainSyncer{
		connector:  connector,
		blockchain: blockchain,
		wg:         sync.WaitGroup{},
	}
}
