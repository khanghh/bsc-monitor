package leth

import (
	"sync"

	"github.com/ethereum/go-ethereum/core"
)

type handler struct {
	connector  *RpcConnector
	blockchain *core.BlockChain
	txpool     *core.TxPool

	syncer *chainSyncer
	wg     sync.WaitGroup
}

func (h *handler) runSync() {
	defer h.wg.Done()
	h.syncer.runSync()
}

func (h *handler) Start() error {
	// start sync blockchain
	h.wg.Add(1)
	go h.runSync()
	return nil
}

func (h *handler) Stop() error {
	h.syncer.stop()
	h.wg.Wait()
	return nil
}

func newHandler(connector *RpcConnector, blockchain *core.BlockChain, txpool *core.TxPool) *handler {
	h := &handler{
		connector:  connector,
		blockchain: blockchain,
		txpool:     txpool,
		syncer:     newChainSyncer(connector, blockchain, txpool),
	}
	return h
}
