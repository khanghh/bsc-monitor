package monitor

import (
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type blockParser struct {
	abiParser abiutils.ABIParser
	data      *blockIndexData
	txAccs    []AccountDetail // contracts created after transaction finished
}

func (p *blockParser) OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {
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

func (p *blockParser) createContractInfo(creator common.Address, bytecode []byte) *ContractInfo {
	ids := abiutils.ParseMethodIds(bytecode)
	ifs, methods := p.abiParser.ParseInterfaces(ids)
	ifnames := make([]string, len(ifs))
	for i, intf := range ifs {
		ifnames[i] = intf.Name
	}
	return &ContractInfo{
		Interfaces: ifnames,
		MethodSigs: methods,
		Creator:    creator,
	}
}

func (p *blockParser) OnCallExit(ctx *reexec.CallCtx) {
	if ctx.Error == nil {
		return
	}
	if ctx.Type == vm.CREATE || ctx.Type == vm.CREATE2 {
		p.txAccs = append(p.txAccs, AccountDetail{
			Address:      ctx.To,
			AccountInfo:  &AccountInfo{FirstTx: ctx.Transaction.Hash()},
			ContractInfo: p.createContractInfo(ctx.From, ctx.Input),
		})
	}
}

func newBlockParser(data *blockIndexData) *blockParser {
	return &blockParser{data: data}
}
