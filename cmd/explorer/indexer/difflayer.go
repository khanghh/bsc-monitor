package indexer

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// diffLayer is in-memory index layer managed by IndexDB for caching uncommited index data
type diffLayer struct {
	parent indexLayer
	root   common.Hash
	data   *indexData
	stale  bool
	lock   sync.RWMutex
}

func (dl *diffLayer) Root() common.Hash {
	return dl.root
}

func (dl *diffLayer) Parent() indexLayer {
	return dl.parent
}

func (dl *diffLayer) Stale() bool {
	return dl.stale
}

func (dl *diffLayer) getAccountInfo(addr common.Address, depth int) (*AccountInfo, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if acc, ok := dl.data.accounts[addr]; ok {
		return acc, nil
	}
	if diff, ok := dl.parent.(*diffLayer); ok {
		return diff.getAccountInfo(addr, depth+1)
	}
	return dl.parent.AccountInfo(addr)
}

func (dl *diffLayer) getContractInfo(addr common.Address, depth int) (*ContractInfo, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if contract, ok := dl.data.contracts[addr]; ok {
		return contract, nil
	}
	if diff, ok := dl.parent.(*diffLayer); ok {
		return diff.getContractInfo(addr, depth+1)
	}
	return dl.parent.ContractInfo(addr)
}

func (dl *diffLayer) getAccountStats(addr common.Address, depth int) (*AccountStats, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()
	if stats, ok := dl.data.accountStats[addr]; ok {
		return stats, nil
	}
	if diff, ok := dl.parent.(*diffLayer); ok {
		return diff.getAccountStats(addr, depth+1)
	}
	return dl.parent.AccountStats(addr)
}

func (dl *diffLayer) AccountInfo(addr common.Address) (*AccountInfo, error) {
	return dl.getAccountInfo(addr, 0)
}

func (dl *diffLayer) ContractInfo(addr common.Address) (*ContractInfo, error) {
	return dl.getContractInfo(addr, 0)
}

func (dl *diffLayer) AccountStats(addr common.Address) (*AccountStats, error) {
	return dl.getAccountStats(addr, 0)
}

func newDiffLayer(parent indexLayer, root common.Hash, data *indexData) *diffLayer {
	for addr, accData := range data.accountData {
		stats, err := parent.AccountStats(addr)
		if err != nil {
			stats = &AccountStats{}
		}
		data.accountStats[addr] = &AccountStats{
			SentTxCount:     stats.SentTxCount + uint64(len(accData.SentTxs)),
			InternalTxCount: stats.SentTxCount + uint64(len(accData.InternalTxs)),
			TokenTxCount:    stats.SentTxCount + uint64(len(accData.TokenTxs)),
			HolderCount:     stats.SentTxCount + uint64(len(accData.Holders)),
		}
	}
	return &diffLayer{
		parent: parent,
		root:   root,
		data:   data,
	}
}
