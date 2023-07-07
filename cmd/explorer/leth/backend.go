package leth

import (
	"sync"
)

type backend struct {
	leth *LightEthereum

	wg sync.WaitGroup
}

func (h *backend) runSync() {
	defer h.wg.Done()
}

func (h *backend) run() error {
	h.wg.Add(1)
	go h.runSync()
	return nil
}

func (h *backend) stop() error {
	h.wg.Wait()
	return nil
}
