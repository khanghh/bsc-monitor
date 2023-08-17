package indexer

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	nilAddress = common.Address{}
	nilHash    = common.Hash{}
)

type AccountIndexData struct {
	SentTxs     []common.Hash
	InternalTxs []common.Hash
	TokenTxs    []common.Hash
	Holders     []common.Address
}

func (s *AccountIndexData) AddSentTx(tx common.Hash) {
	s.SentTxs = append(s.SentTxs, tx)
}

func (s *AccountIndexData) AddInternalTx(tx common.Hash) {
	s.InternalTxs = append(s.InternalTxs, tx)
}

func (s *AccountIndexData) AddTokenTx(tx common.Hash) {
	s.TokenTxs = append(s.TokenTxs, tx)
}

func (s *AccountIndexData) AddHolder(addr common.Address) {
	s.Holders = append(s.Holders, addr)
}

// AccountIndexState is extra data for state trie (trie leaf is `types.StateAccount`)
type AccountIndexState struct {
	LastSentTxRef     []byte
	LastInternalTxRef []byte
	LastTokenTxRef    []byte
	LastHolderRef     []byte
}

type AccountStats struct {
	SentTxCount     uint64
	InternalTxCount uint64
	TokenTxCount    uint64
	HolderCount     uint64
}

// AccountInfo holds basic information of an account
type AccountInfo struct {
	Name    string
	Tags    []string
	FirstTx common.Hash
}

// ContractInfo is additional data for account if it's a contract
type ContractInfo struct {
	Interfaces []string       // List of interface names the contract implemented
	MethodSigs []string       // List of 4-bytes method signatures in contract
	OwnABI     []byte         // JSON ABI which elements are excluded from implemented intefaces
	Creator    common.Address // Contract creator address
}

type AccountDetail struct {
	Address common.Address
	*AccountInfo
	*ContractInfo
}

func isEmptyAccountInfo(acc *AccountInfo) bool {
	return acc.Name == "" &&
		len(acc.Tags) == 0 &&
		acc.FirstTx == nilHash
}
