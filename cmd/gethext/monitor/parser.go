package monitor

import (
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type blockParser struct {
	data   *blockIndexData
	txAccs []AccountDetail // contracts created after transaction finished
}

func (p *blockParser) OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {
	// fmt.Printf("tx: %#v\n", ctx.Transaction.Hash())
	p.txAccs = make([]AccountDetail, 0)
}

func (p *blockParser) OnTxEnd(ctx *reexec.TxContext, resetGas uint64) {
	txHash := ctx.Transaction.Hash()
	defer p.data.AccountChangeSet(ctx.Message.From()).AddSentTx(txHash)
	if ctx.Transaction.Nonce() == 0 {
		accInfo := AccountInfo{FirstTx: txHash}
		p.data.SetAccountInfo(ctx.Message.From(), &accInfo)
		log.Info(fmt.Sprintf("Add new account %#v ", ctx.Message.From()), "number", ctx.Block.NumberU64(), "tx", txHash.Hex())
	}
	if ctx.Reverted {
		return
	}
	for _, acc := range p.txAccs {
		p.data.SetAccountDetail(&acc)
		log.Info(fmt.Sprintf("Add new contract %#v ", ctx.Message.From()), "number", ctx.Block.NumberU64(), "tx", txHash.Hex())
	}
}

func (p *blockParser) OnCallEnter(ctx *reexec.CallCtx) {
}

func (p *blockParser) OnCallExit(ctx *reexec.CallCtx) {
	if ctx.Error == nil {
		return
	}
	if ctx.Type == vm.CREATE || ctx.Type == vm.CREATE2 {
		p.data.SetContractInfo(ctx.To, &ContractInfo{Creator: ctx.From})
		p.txAccs = append(p.txAccs, AccountDetail{
			Address:      ctx.To,
			AccountInfo:  &AccountInfo{FirstTx: ctx.Transaction.Hash()},
			ContractInfo: &ContractInfo{Creator: ctx.From},
		})
	}
}

func newBlockParser(data *blockIndexData) *blockParser {
	return &blockParser{data: data}
}
