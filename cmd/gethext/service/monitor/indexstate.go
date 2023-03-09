package monitor

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/extdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

type stateObject struct {
	indexdb *IndexDB
	origin  common.Hash

	accounts      map[common.Address]*AccountDetail     // commited account infos
	accStates     map[common.Address]*AccountIndexState // commited index states
	dirtyChange   map[common.Address]*accountIndex
	dirtyAccounts map[common.Address]*AccountDetail
}

func (s *stateObject) DirtyAccounts() []common.Address {
	accounts := make([]common.Address, 0, len(s.dirtyAccounts))
	for account := range s.dirtyAccounts {
		accounts = append(accounts, account)
	}
	return accounts
}

// GetAccountDetail retrieves account info and contract info of the given address
func (s *stateObject) GetAccountDetail(addr common.Address) *AccountDetail {
	if acc, exist := s.dirtyAccounts[addr]; exist {
		return acc
	}
	if acc, exist := s.accounts[addr]; exist {
		return acc
	}
	accInfo, err := s.indexdb.AccountInfo(addr)
	if err != nil {
		accInfo = &AccountInfo{}
	}
	contractInfo, _ := s.indexdb.ContractInfo(addr)
	s.accounts[addr] = &AccountDetail{
		Address:      addr,
		AccountInfo:  accInfo,
		ContractInfo: contractInfo,
	}
	return s.accounts[addr]
}

func (s *stateObject) SetAccountDetail(addr common.Address, accInfo *AccountInfo, contractInfo *ContractInfo) *AccountDetail {
	s.dirtyAccounts[addr] = &AccountDetail{
		Address:      addr,
		AccountInfo:  accInfo,
		ContractInfo: contractInfo,
	}
	return s.dirtyAccounts[addr]
}

func (s *stateObject) AccountIndex(addr common.Address) *accountIndex {
	if accIndex, exist := s.dirtyChange[addr]; exist {
		return accIndex
	}
	indexState, err := s.indexdb.AccountExtState(s.origin, addr)
	if err != nil {
		indexState = new(AccountIndexState)
	}
	s.dirtyChange[addr] = &accountIndex{
		IndexState: indexState,
		ChangeSet:  &AccountIndexData{},
	}
	return s.dirtyChange[addr]
}

func (s *stateObject) getOriginState(addr common.Address) *AccountIndexState {
	if indexState, exist := s.accStates[addr]; exist {
		return indexState
	}
	indexState, err := s.indexdb.AccountExtState(s.origin, addr)
	if err != nil {
		indexState = new(AccountIndexState)
	}
	return indexState
}

func (s *stateObject) commitAccounts(db ethdb.KeyValueWriter) error {
	for addr, accDetail := range s.dirtyAccounts {
		if accDetail.AccountInfo != nil {
			enc, _ := rlp.EncodeToBytes(accDetail.AccountInfo)
			extdb.WriteAccountInfo(db, addr, enc)
		}
	}
	return nil
}

func (s *stateObject) commitChanges(db ethdb.KeyValueWriter, newRoot common.Hash) error {
	for addr, accIndex := range s.dirtyChange {
		changeSet := accIndex.ChangeSet
		indexState := accIndex.IndexState
		originState := s.getOriginState(addr)
		for i, txHash := range changeSet.SentTxs {
			txIndex := originState.SentTxCount + uint64(i)
			extdb.WriteAccountSentTx(db, addr, txHash, txIndex)
		}
		for i, txHash := range changeSet.InternalTxs {
			txIndex := originState.InternalTxCount + uint64(i)
			extdb.WriteAccountInternalTx(db, addr, txHash, txIndex)
		}
		hash, err := s.indexdb.getIndexStateHash(newRoot, addr)
		if err != nil {
			log.Error("Could not calculate index state hash", "root", newRoot, "error", err)
			return err
		}
		enc, _ := rlp.EncodeToBytes(indexState)
		extdb.WriteAccountExtState(db, hash, enc)
	}
	return nil
}

// Commit writes pending change sets and indexing states for new state root
func (s *stateObject) Commit(newRoot common.Hash) error {
	batch := s.indexdb.diskdb.NewBatch()
	s.commitAccounts(batch)
	if err := s.commitChanges(batch, newRoot); err != nil {
		return err
	}
	return batch.Write()
}

func newStateObject(indexdb *IndexDB, origin common.Hash) *stateObject {
	return &stateObject{
		indexdb:       indexdb,
		origin:        origin,
		accounts:      make(map[common.Address]*AccountDetail),
		accStates:     make(map[common.Address]*AccountIndexState),
		dirtyChange:   make(map[common.Address]*accountIndex),
		dirtyAccounts: make(map[common.Address]*AccountDetail),
	}
}
