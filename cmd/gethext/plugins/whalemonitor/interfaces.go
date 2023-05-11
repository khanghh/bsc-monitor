package whalemonitor

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

type ERC20Token struct {
	Address  common.Address
	Name     string
	Symbol   string
	Decimals uint64
}

type WhaleEvent struct {
	TxHash common.Hash
	Token  *ERC20Token
	From   common.Address
	To     common.Address
	Value  *big.Int
}

type WhaleMonitor interface {
	SubscribeWhaleEvent(ch chan<- WhaleEvent) event.Subscription
}
