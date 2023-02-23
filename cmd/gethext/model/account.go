package model

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type AccountInfo struct {
	Name    string         `json:"name"`
	Address common.Address `json:"address"`
	Tags    []string       `json:"tags"`
	FirstTx common.Hash    `json:"created"`
}

type ContractInfo struct {
	Type string `json:"type"`
	ABI  []byte `json:"abi"`
}

type AccountState struct {
	Nonce     uint64      `json:"nonce"`
	Balance   *big.Int    `json:"balance"`
	Root      common.Hash `json:"storageRoot"`
	CodeHash  []byte      `json:"codeHash"`
	ExtraData []byte      `json:"extraData"`
}

type Account struct {
	AccountInfo
	*AccountState
}
