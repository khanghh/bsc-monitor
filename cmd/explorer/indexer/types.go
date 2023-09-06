package indexer

import (
	"github.com/ethereum/go-ethereum/common"
)

type AccountIndexData struct {
	SentTxs     []common.Hash
	InternalTxs []common.Hash
	TokenTxs    []common.Hash
	Holders     []common.Address
}

// AccountIndexRefs holds the reference numbers for each indexed field of an account at a specific block.
// These refs provide an easy way to iterate over each indexed field in database.
type AccountIndexRefs struct {
	LastSentTxRef     []byte
	LastInternalTxRef []byte
	LastTokenTxRef    []byte
	LastHolderRef     []byte
}

// AccountStats represents the statistical data of an account.
type AccountStats struct {
	SentTxCount     uint64 // Total number of sent transactions from the account.
	InternalTxCount uint64 // Total number of internal transactions involving the account.
	TokenTxCount    uint64 // Total number of token transactions involving the account.
	HolderCount     uint64 // Total number of holders associated with the account.
}

// AccountInfo holds basic information about an Ethereum account or contract.
type AccountInfo struct {
	Name    string      // Name represents the name of the account.
	Tags    []string    // Tags includes additional tags for the account, providing easy insights.
	FirstTx common.Hash // FirstTx refers to the first transaction made by the account or the contract deployment.
}

// ContractInfo is additional data for account if it is a contract
type ContractInfo struct {
	Name       string         // Name represents the known name of the contract.
	Interfaces []string       // List of interface names the contract implemented
	MethodSigs []string       // List of 4-bytes method signatures in contract
	OwnABI     []byte         // JSON ABI which elements are excluded from implemented intefaces
	Creator    common.Address // Contract creator address
	Destructed bool           // Destructed represents whether contract was destroyed or not
}

type Account struct {
	Address common.Address
	*AccountInfo
	*ContractInfo
}
