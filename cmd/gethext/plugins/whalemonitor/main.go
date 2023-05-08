package main

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	pluginNamespace = "WhaleMonitor"
)

var (
	logger                    log.Logger
	IERC20                    *abiutils.Interface
	bigETHTransferThreshold   = new(big.Int).Mul(big.NewInt(100), big.NewInt(params.Ether))
	bigTokenTransferThreshold = map[common.Address]uint64{
		common.HexToAddress("0xA3183498b579bd228aa2B62101C40CC1da978F24"): 50000, // test token
		common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"): 50000, // USDT
		common.HexToAddress("0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"): 50000, // BUSD
		common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"): 100,   // WBNB
		common.HexToAddress("0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c"): 2,     // BTCB
		common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8"): 50,    // WETH
		common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"): 50000, // USDC
		common.HexToAddress("0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82"): 10000, // CAKE
	}
)

type ERC20Token struct {
	Address  common.Address
	Name     string
	Symbol   string
	Decimals *big.Int
}

func (t *ERC20Token) Amount(val uint64) *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(val)), t.Decimals)
}

type txProcessor struct {
	client   *rpc.Client
	state    *state.StateDB
	txResult *reexec.TxContext
}

func (t *txProcessor) getTokenInfo(addr common.Address) (*ERC20Token, error) {
	erc20 := &ERC20Token{Address: addr}
	batch := []rpc.BatchElem{
		{Method: "name", Result: &erc20.Name},
		{Method: "symbol", Result: &erc20.Symbol},
		{Method: "decimals", Result: &erc20.Decimals},
	}
	if err := t.client.BatchCall(batch); err != nil {
		return erc20, err
	}
	return erc20, nil
}

func (t *txProcessor) processCallFrame(frame *reexec.CallFrame) {
	if frame.Value != nil {
		if frame.Value.Cmp(bigETHTransferThreshold) > 0 {
			txHash := t.txResult.Transaction.Hash()
			logger.Info("Big ETH transfer", "from", frame.From, "to", frame.To, "value", frame.Value, "tx", txHash.Hex())
		}
	}
	threshold, exist := bigTokenTransferThreshold[frame.To]
	if !exist {
		return
	}
	bytecode := t.state.GetCode(frame.To)
	if len(bytecode) == 0 {
		return
	}
	contract, err := abiutils.DefaultParser().ParseContract(bytecode)
	if err != nil {
		return
	}
	txHash := t.txResult.Transaction.Hash()
	if erc20, ok := contract.Implements["IERC20"]; ok && len(frame.Input) >= 4 {
		methodSig, data := frame.Input[0:4], frame.Input[4:]
		method, err := erc20.MethodById(methodSig)
		if err != nil {
			return
		}
		token, err := t.getTokenInfo(frame.To)
		if err != nil {
			return
		}
		switch method.RawName {
		case "transfer":
			var args struct {
				To     common.Address
				Amount *big.Int
			}
			if erc20.UnpackInput(&args, "transfer", data); err != nil {
				logger.Error("Could not unpack input", "method", "transfer", "input", hexutil.Encode(frame.Input), "tx", txHash.Hex())
				return
			}
			if args.Amount != nil && args.Amount.Cmp(token.Amount(threshold)) > 0 {
				logger.Info("Big ERC20 token transfer", "from", frame.From, "to", frame.To, "token", token.Symbol, "amount", args.Amount, "tx", txHash.Hex())
			}
		case "transferFrom":
			var args struct {
				From   common.Address
				To     common.Address
				Amount *big.Int
			}
			if erc20.UnpackInput(&args, "transferFrom", data); err != nil {
				logger.Error("Could not unpack input", "method", "transferFrom", "input", hexutil.Encode(frame.Input), "tx", txHash.Hex())
				return
			}
			if args.Amount != nil && args.Amount.Cmp(token.Amount(threshold)) > 0 {
				logger.Info("Big ERC20 token transfer", "from", frame.From, "to", frame.To, "token", token.Symbol, "amount", args.Amount.Uint64(), "tx", txHash.Hex())
			}
		}
	}
}

func (t *txProcessor) processCallStack(callstack []reexec.CallFrame) {
	for _, frame := range callstack {
		t.processCallFrame(&frame)
		if len(frame.Calls) > 0 {
			t.processCallStack(frame.Calls)
		}
	}
}

type WhaleMonitorPlugin struct {
	ctx    *plugin.PluginCtx
	client *rpc.Client
}

func (p *WhaleMonitorPlugin) processTx(wg *sync.WaitGroup, state *state.StateDB, txRet *reexec.TxContext) {
	defer func() {
		wg.Done()
	}()
	if txRet != nil && !txRet.Reverted {
		proc := txProcessor{
			client:   p.client,
			state:    state,
			txResult: txRet,
		}
		proc.processCallStack(txRet.CallStack)
	}
}

func (p *WhaleMonitorPlugin) ProcessBlock(state *state.StateDB, block *types.Block, txResults []*reexec.TxContext) error {
	wg := sync.WaitGroup{}
	for _, txRet := range txResults {
		wg.Add(1)
		go p.processTx(&wg, state, txRet)
	}
	return nil
}

func (p *WhaleMonitorPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	var err error
	logger = plugin.NewLogger(pluginNamespace)
	IERC20, err = abiutils.DefaultParser().LookupInterface("IERC20")
	if err != nil {
		return err
	}
	client, err := ctx.Node.Attach()
	if err != nil {
		return err
	}
	p.client = client
	p.ctx = ctx
	ctx.Monitor.RegisterProcessor(pluginNamespace, p)
	return nil
}

func (p *WhaleMonitorPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &WhaleMonitorPlugin{}
}
