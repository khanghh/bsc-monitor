//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package indexer

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/extdb"
	"github.com/ethereum/go-ethereum/rlp"
)

// blockIndexData hold data collected during indexing for the given state root
type blockIndexData struct {
	indexdb *IndexDB
	block   *types.Block

	dirtyStates   map[common.Address]*AccountIndexState
	dirtyChanges  map[common.Address]*AccountIndexData
	dirtyAccounts map[common.Address]*AccountDetail
}

func (s *blockIndexData) DirtyAccounts() []common.Address {
	accounts := make([]common.Address, 0, len(s.dirtyAccounts))
	for account := range s.dirtyAccounts {
		accounts = append(accounts, account)
	}
	return accounts
}

// AccountDetail retrieves account info and contract info of the given address
func (s *blockIndexData) AccountDetail(addr common.Address) *AccountDetail {
	if acc, exist := s.dirtyAccounts[addr]; exist {
		return acc
	}
	accDetail, err := s.indexdb.AccountDetail(addr)
	if err != nil {
		accDetail = &AccountDetail{addr, &AccountInfo{}, nil}
	}
	s.dirtyAccounts[addr] = accDetail
	return accDetail
}

// AccountChangeSet create object to hold update data of account extracted from this block
func (s *blockIndexData) AccountChangeSet(addr common.Address) *AccountIndexData {
	if changeSet, exist := s.dirtyChanges[addr]; exist {
		return changeSet
	}
	s.dirtyChanges[addr] = new(AccountIndexData)
	return s.dirtyChanges[addr]
}

func (s *blockIndexData) SetAccountDetail(detail *AccountDetail) {
	s.dirtyAccounts[detail.Address] = detail
}

func (s *blockIndexData) SetAccountInfo(addr common.Address, info *AccountInfo) {
	s.AccountDetail(addr).AccountInfo = info
}

func (s *blockIndexData) SetContractInfo(addr common.Address, info *ContractInfo) {
	s.AccountDetail(addr).ContractInfo = info
}

func (s *blockIndexData) commitStates(batch ethdb.Batch) error {
	if len(s.dirtyStates) == 0 {
		return nil
	}
	tr, err := s.indexdb.OpenIndexTrie(s.block.Root())
	if err != nil {
		return err
	}
	for addr, state := range s.dirtyStates {
		enc, _ := rlp.EncodeToBytes(state)
		err := tr.TryUpdate(addr.Bytes(), enc)
		if err != nil {
			return err
		}
	}
	return tr.Commit(batch)
}

func (s *blockIndexData) commitChanges(batch ethdb.Batch, withState bool) error {
	for addr, changeSet := range s.dirtyChanges {
		state := new(AccountIndexState)
		for idx, txHash := range changeSet.SentTxs {
			ref := extdb.IndexItemRef(s.block.NumberU64(), uint64(idx))
			refNum := binary.BigEndian.Uint64(ref)
			extdb.WriteAccountSentTx(batch, addr, refNum, txHash)
			state.LastSentTxRef = ref
		}
		for idx, txHash := range changeSet.InternalTxs {
			ref := extdb.IndexItemRef(s.block.NumberU64(), uint64(idx))
			refNum := binary.BigEndian.Uint64(ref)
			extdb.WriteAccountInternalTx(batch, addr, refNum, txHash)
			state.LastInternalTxRef = ref
		}
		for idx, txHash := range changeSet.TokenTxs {
			ref := extdb.IndexItemRef(s.block.NumberU64(), uint64(idx))
			refNum := binary.BigEndian.Uint64(ref)
			extdb.WriteAccountTokenTx(batch, addr, refNum, txHash)
			state.LastTokenTxRef = ref
		}
		for idx, addr := range changeSet.Holders {
			ref := extdb.IndexItemRef(s.block.NumberU64(), uint64(idx))
			refNum := binary.BigEndian.Uint64(ref)
			extdb.WriteTokenHolderAddr(batch, addr, refNum, addr)
			state.LastHolderRef = ref
		}
		s.dirtyStates[addr] = state
	}
	if withState {
		return s.commitStates(batch)
	}
	return nil
}

func (s *blockIndexData) commitAccounts(batch ethdb.Batch) error {
	for addr, acc := range s.dirtyAccounts {
		if !isEmptyAccountInfo(acc.AccountInfo) {
			enc, _ := rlp.EncodeToBytes(acc.AccountInfo)
			extdb.WriteAccountInfo(batch, addr, enc)
			if acc.ContractInfo != nil {
				enc, _ := rlp.EncodeToBytes(acc.ContractInfo)
				extdb.WriteContractInfo(batch, addr, enc)
			}
			s.indexdb.cacheAccountDetail(addr, acc)
		}
	}
	return nil
}

// Commit write data collected of this block to the given writer
func (s *blockIndexData) Commit(batch ethdb.Batch, withState bool) error {
	// write account info
	if err := s.commitAccounts(batch); err != nil {
		return err
	}
	// write change set and state
	if err := s.commitChanges(batch, withState); err != nil {
		return err
	}
	return nil
}

func newBlockIndexData(indexdb *IndexDB, block *types.Block) *blockIndexData {
	return &blockIndexData{
		indexdb:       indexdb,
		block:         block,
		dirtyStates:   make(map[common.Address]*AccountIndexState),
		dirtyChanges:  make(map[common.Address]*AccountIndexData),
		dirtyAccounts: make(map[common.Address]*AccountDetail),
	}
}
