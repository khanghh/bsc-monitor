//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type IndexerAPI struct {
	indexer *ChainIndexer
}

func (api *IndexerAPI) AccountSentTxList(addr common.Hash, offset uint64, limit uint64) ([]common.Hash, error) {
	return nil, nil
}

func (api *IndexerAPI) AccountInternalTxList(addr common.Hash, offset uint64, limit uint64) ([]common.Hash, error) {
	return nil, nil
}

func (api *IndexerAPI) AccountTokenTxList(addr common.Hash, offset uint64, limit uint64) ([]common.Hash, error) {
	return nil, nil
}

func (api *IndexerAPI) TokenHolderList(addr common.Address, offset uint64, limit uint64) ([]common.Address, error) {
	return nil, nil
}

func (api *IndexerAPI) AccountList(addr common.Hash, offset uint64, limit uint64) ([]common.Address, error) {
	return nil, nil
}

func (api *IndexerAPI) ContractList(addr common.Hash, offset uint64, limit uint64) (types.Transactions, error) {
	return nil, nil
}

func NewIndexerAPI(indexer *ChainIndexer) *IndexerAPI {
	return &IndexerAPI{indexer}
}
