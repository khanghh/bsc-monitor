package whalemonitor

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

type WhaleEventType int

const (
	TypeTokenTransfer WhaleEventType = iota
	TypeTokenSwap
	TypeFlashLoan
)

type ERC20Token struct {
	Address  common.Address
	Name     string
	Symbol   string
	Decimals uint64
}

type TokenTransfer struct {
	From     common.Address              // Token sender address
	To       common.Address              // Token receiver address
	Balances map[common.Address]*big.Int // Balances after the token tranfer
	Token    *ERC20Token                 // ERC20 token information, nil if native token
	Value    *big.Int                    // Token transfer amount
}

type WhaleEvent struct {
	Type      WhaleEventType
	TxHash    common.Hash
	Transfers []TokenTransfer
}

type WhaleMonitor interface {
	SubscribeWhaleEvent(ch chan<- WhaleEvent) event.Subscription
}
