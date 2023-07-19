package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type TokenTransferMonitor struct {
	*handler
	thresholds map[common.Address]float64
	transfers  []whalemonitor.TokenTransfer
}

func (m *TokenTransferMonitor) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	m.transfers = make([]whalemonitor.TokenTransfer, 0)
	_, tx := ctx.Transaction()
	from, err := types.Sender(ctx.Signer(), tx)
	if tx.To() == nil || err != nil {
		return
	}
	to := *tx.To()
	ethThreshold := ParseAmount(m.thresholds[nilAddress], params.Ether)
	if err == nil && tx.Value().Cmp(ethThreshold) >= 0 {
		balances := map[common.Address]*big.Int{
			from: ctx.State().GetBalance(from),
			to:   ctx.State().GetBalance(to),
		}
		m.transfers = append(m.transfers, whalemonitor.TokenTransfer{
			From:     from,
			To:       to,
			Value:    tx.Value(),
			Balances: balances,
		})
	}
}

func (m *TokenTransferMonitor) OnCallEnter(ctx *reexec.Context, call *reexec.CallFrame) {}

func (m *TokenTransferMonitor) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
	if call.Error != nil {
		return
	}

	contract, err := abiutils.DefaultParser().ParseContract(ctx.State().GetCode(call.To))
	if err != nil {
		return
	}

	if erc20, ok := contract.Implements["IERC20"]; ok && len(call.Input) >= 4 {
		_, tx := ctx.Transaction()
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

	if len(m.transfers) > 0 {
		m.feed.Send(whalemonitor.WhaleEvent{
			Type:      whalemonitor.TypeTokenTransfer,
			TxHash:    tx.Hash(),
			Transfers: m.transfers,
		})
	}
}

func NewTokenTransferMonitor(handler *handler, threshold map[common.Address]float64) *TokenTransferMonitor {
	return &TokenTransferMonitor{
		handler:    handler,
		thresholds: threshold,
	}
}
