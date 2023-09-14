package indexer

import (
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

func uniqueAppend(arr []string, item string) []string {
	for _, elem := range arr {
		if elem == item {
			return arr
		}
	}
	return append(arr, item)
}

type accountInfoUpdater struct {
	account *AccountInfo
}

func (s *accountInfoUpdater) SetName(name string) *accountInfoUpdater {
	s.account.Name = name
	return s
}

func (s *accountInfoUpdater) AddTags(tags ...string) *accountInfoUpdater {
	for _, item := range tags {
		s.account.Tags = uniqueAppend(s.account.Tags, item)
	}
	return s
}

func (s *accountInfoUpdater) SetFirstTx(hash common.Hash) *accountInfoUpdater {
	s.account.FirstTx = hash
	return s
}

type contractInfoUpdater struct {
	contract *ContractInfo
}

func (s *contractInfoUpdater) AddInterfaces(ifs ...string) *contractInfoUpdater {
	for _, item := range ifs {
		s.contract.Interfaces = uniqueAppend(s.contract.Interfaces, item)
	}
	return s
}

func (s *contractInfoUpdater) AddMethodSigs(sigs ...string) *contractInfoUpdater {
	for _, sig := range sigs {
		s.contract.MethodSigs = uniqueAppend(s.contract.MethodSigs, sig)
	}
	return s
}

func (s *contractInfoUpdater) SetOwnABI(blob []byte) *contractInfoUpdater {
	if s.contract != nil {
		s.contract.OwnABI = blob
	}
	return s
}

func (s *contractInfoUpdater) SetCreator(creator common.Address) *contractInfoUpdater {
	if s.contract != nil {
		s.contract.Creator = creator
	}
	return s
}

type accountIndexCollector struct {
	accountData *AccountIndexData
}

func (s *accountIndexCollector) AddSentTx(tx common.Hash) *accountIndexCollector {
	s.accountData.SentTxs = append(s.accountData.SentTxs, tx)
	return s
}

func (s *accountIndexCollector) AddInternalTx(tx common.Hash) *accountIndexCollector {
	s.accountData.InternalTxs = append(s.accountData.InternalTxs, tx)
	return s
}

func (s *accountIndexCollector) AddTokenTx(tx common.Hash) *accountIndexCollector {
	s.accountData.TokenTxs = append(s.accountData.TokenTxs, tx)
	return s
}

func (s *accountIndexCollector) AddHolder(addr common.Address) *accountIndexCollector {
	s.accountData.Holders = append(s.accountData.Holders, addr)
	return s
}

// txIndexHook collects various index data from the transaction being executed
type txIndexHook struct {
	data         *indexData
	tmpAccounts  map[common.Address]*AccountInfo
	tmpContracts map[common.Address]*ContractInfo
}

func (h *txIndexHook) updateAccountInfo(addr common.Address) *accountInfoUpdater {
	if _, ok := h.data.accounts[addr]; !ok {
		h.data.accounts[addr] = &AccountInfo{}
	}
	return &accountInfoUpdater{account: h.data.accounts[addr]}
}

func (h *txIndexHook) updateContractInfo(addr common.Address) *contractInfoUpdater {
	if _, ok := h.data.accounts[addr]; !ok {
		h.data.contracts[addr] = &ContractInfo{}
	}
	return &contractInfoUpdater{contract: h.data.contracts[addr]}
}

func (h *txIndexHook) updateAccountIndex(addr common.Address) *accountIndexCollector {
	if _, ok := h.data.accountData[addr]; !ok {
		h.data.accountData[addr] = &AccountIndexData{}
	}
	return &accountIndexCollector{accountData: h.data.accountData[addr]}
}

func (h *txIndexHook) OnTxStart(ctx *reexec.Context, gasLimit uint64) {
	h.tmpAccounts = make(map[common.Address]*AccountInfo)
	h.tmpContracts = make(map[common.Address]*ContractInfo)
}

func (h *txIndexHook) OnCallEnter(ctx *reexec.Context, call *reexec.CallFrame) {
}

func (p *txIndexHook) OnCallExit(ctx *reexec.Context, call *reexec.CallFrame) {
	if call.Error == nil {
		return
	}
	_, tx := ctx.Transaction()
	if call.Type == vm.CREATE || call.Type == vm.CREATE2 {
		p.updateAccountInfo(call.To).SetFirstTx(tx.Hash())
		p.updateContractInfo(call.To).SetCreator(call.From)
	}
}

func (p *txIndexHook) OnTxEnd(ctx *reexec.Context, ret *reexec.TxResult, restGas uint64) {
	_, tx := ctx.Transaction()
	sender, err := types.Sender(ctx.Signer(), tx)
	if err != nil {
		return
	}
	defer p.updateAccountIndex(sender).AddSentTx(tx.Hash())
	if tx.Nonce() == 0 {
		p.updateAccountInfo(sender).SetFirstTx(tx.Hash())
		log.Info(fmt.Sprintf("Add new account %#v ", sender), "number", ctx.Block().NumberU64(), "tx", tx.Hash().Hex())
	}

	if ret.Reverted {
		return
	}

}

func newTxHook(block *types.Block) *txIndexHook {
	return &txIndexHook{
		data: &indexData{
			blockNum:     block.NumberU64(),
			accounts:     make(map[common.Address]*AccountInfo),
			contracts:    make(map[common.Address]*ContractInfo),
			accountData:  make(map[common.Address]*AccountIndexData),
			accountStats: make(map[common.Address]*AccountStats),
			accountRefs:  make(map[common.Address]*AccountIndexRefs),
		},
	}
}
