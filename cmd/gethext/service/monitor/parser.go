package monitor

import (
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/core/vm"
)

// ContractParser detect all methods and interfaces that the contract was implemented
type ContractParser struct {
	abiDir string
}

// ParseMethods parse the contract code to get all methods it have
func (d *ContractParser) ParseMethods(bytecode []byte) []string {
	// Single function calls will follow the following repeating pattern:
	// DUP1
	// PUSH4 <4-byte function signature>
	// EQ
	// PUSH2 <jumpdestination for the function>
	// JUMPI
	return nil
}

func NewContractDetector(abiDir string) *ContractParser {
	return &ContractParser{
		abiDir: abiDir,
	}
}

type blockTxParser struct {
	data   *blockIndex
	txAccs []AccountDetail // contracts created by the transaction
}

func (p *blockTxParser) OnTxStart(ctx *reexec.TxContext, gasLimit uint64) {
	p.txAccs = make([]AccountDetail, 0)
}

func (p *blockTxParser) OnTxEnd(ctx *reexec.TxContext, resetGas uint64) {
	txHash := ctx.Transaction.Hash()
	defer p.data.AccountChangeSet(ctx.Message.From()).AddSentTx(txHash)
	if ctx.Transaction.Nonce() == 0 {
		accInfo := AccountInfo{FirstTx: txHash}
		p.data.SetAccountInfo(ctx.Message.From(), &accInfo)
	}
	if ctx.Reverted {
		return
	}
	for _, acc := range p.txAccs {
		p.data.SetAccountDetail(&acc)
	}
}

func (p *blockTxParser) OnCallEnter(ctx *reexec.CallCtx) {
}

func (p *blockTxParser) OnCallExit(ctx *reexec.CallCtx) {
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

func neweBlockParser(data *blockIndex) *blockTxParser {
	return &blockTxParser{data: data}
}
