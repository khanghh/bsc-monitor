package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type TokenTransferMonitor struct {
	*handler
	transfers []whalemonitor.TokenTransfer
}

func (m *TokenTransferMonitor) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	m.transfers = make([]whalemonitor.TokenTransfer, 0)
	_, tx := ctx.Transaction()
	from, err := types.Sender(ctx.Signer(), tx)
	if tx.To() == nil || err != nil {
		return
	}
	to := *tx.To()
	if tx.Value().Cmp(big.NewInt(0)) > 0 {
		m.transfers = append(m.transfers, whalemonitor.TokenTransfer{
			From:  from,
			To:    to,
			Value: tx.Value(),
			Balances: map[common.Address]*big.Int{
				from: ctx.State().GetBalance(from),
				to:   ctx.State().GetBalance(to),
			},
		})
	}
}

func (m *TokenTransferMonitor) OnCallEnter(ctx *reexec.Context, call *reexec.CallFrame) {}

func (m *TokenTransferMonitor) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
	if call.Error != nil {
		return
	}

	_, tx := ctx.Transaction()
	if call.Value != nil && call.Value.Cmp(big.NewInt(0)) > 0 {
		m.transfers = append(m.transfers, whalemonitor.TokenTransfer{
			From:  call.From,
			To:    call.To,
			Value: call.Value,
			Balances: map[common.Address]*big.Int{
				call.From: ctx.State().GetBalance(call.From),
				call.To:   ctx.State().GetBalance(call.To),
			},
		})
	}

	contract, err := abiutils.DefaultParser().ParseContract(ctx.State().GetCode(call.To))
	if err != nil {
		return
	}

	if erc20, ok := contract.Implements["IERC20"]; ok && len(call.Input) >= 4 {
		args, err := ParseERC20TransferArgs(call.From, &erc20, call.Input)
		if err != nil {
			return
		}
		token, err := m.getERC20Info(call.To)
		if err != nil {
			log.Debug("Could not get ERC20 token transfer", "token", call.To.Hex(), "tx", tx.Hash().Hex(), "error", err)
			return
		}
		m.transfers = append(m.transfers, whalemonitor.TokenTransfer{
			From:  args.From,
			To:    args.To,
			Token: token,
			Value: args.Amount,
		})
	}
}

func (m *TokenTransferMonitor) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, resetGas uint64) {
	_, tx := ctx.Transaction()
	if ret.Reverted || tx.To() == nil {
		return
	}

	// Check if any token transfer meet the whale threshold
	for _, transfer := range m.transfers {
		var threshold *big.Int = nil
		if transfer.Token == nil {
			threshold = ParseAmount(m.config.Thresholds[nilAddress], 18)
		} else if thrsVal, exist := m.config.Thresholds[transfer.Token.Address]; exist {
			threshold = ParseAmount(thrsVal, transfer.Token.Decimals)
		}
		if threshold != nil && transfer.Value.Cmp(threshold) >= 0 {
			log.Warn("Whale transfer detected!", "tx", tx.Hash().Hex())
			m.feed.Send(whalemonitor.WhaleEvent{
				Type:      whalemonitor.TypeTokenTransfer,
				TxHash:    tx.Hash(),
				Transfers: m.transfers,
			})
			return
		}
	}
}

func NewTokenTransferMonitor(handler *handler) *TokenTransferMonitor {
	return &TokenTransferMonitor{
		handler: handler,
	}
}
