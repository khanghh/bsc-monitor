package monitor

import "github.com/ethereum/go-ethereum/common"

type AccountChangeSet struct {
	SentTxs     []common.Hash `json:"t"`
	ReceivedTxs []common.Hash `json:"i"`
	TokenTxs    []common.Hash `json:"x"`
}

// ExtStateAccount is additional state for a account corresponding to account state in state trie
type ExtStateAccount struct {
	SentTxIndex     uint64 `json:"t"` // index of lastest transaction (nonce)
	ReceivedTxIndex uint64 `json:"i"` // index of lastest internal transaction
	TokenTxIndex    uint64 `json:"x"`
}
