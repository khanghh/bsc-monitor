package main

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugin"
	"github.com/ethereum/go-ethereum/cmd/gethext/plugins/whalemonitor"
	"github.com/ethereum/go-ethereum/cmd/gethext/reexec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	pluginNamespace = "WhaleMonitor"
)

var (
	logger                    = plugin.NewLogger(pluginNamespace)
	bigETHTransferThreshold   = new(big.Int).Mul(big.NewInt(500), big.NewInt(params.Ether))
	bigERC20TransferThreshold = map[common.Address]float64{
		common.HexToAddress("0xA3183498b579bd228aa2B62101C40CC1da978F24"): 50000,  // test token
		common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"): 200000, // USDT
		common.HexToAddress("0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"): 200000, // BUSD
		common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"): 500,    // WBNB
		common.HexToAddress("0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c"): 7,      // BTCB
		common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8"): 100,    // WETH
		common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"): 200000, // USDC
		common.HexToAddress("0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82"): 10000,  // CAKE
	}
)

type txProcessor struct {
	client    *rpc.Client
	state     *state.StateDB
	txResult  *reexec.TxContext
	whaleFeed *event.Feed
}

func (t *txProcessor) getTokenInfo(addr common.Address) (*whalemonitor.ERC20Token, error) {
	client := ethclient.NewClient(t.client)
	erc20, err := NewERC20(addr, client)
	if err != nil {
		return nil, err
	}
	name, err := erc20.Name(&bind.CallOpts{})
	if err != nil {
		name = "Unknown"
	}
	symbol, err := erc20.Symbol(&bind.CallOpts{})
	if err != nil {
		symbol = "Unknown"
	}
	decimals, err := erc20.Decimals(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	return &whalemonitor.ERC20Token{
		Address:  addr,
		Name:     name,
		Symbol:   symbol,
		Decimals: uint64(decimals),
	}, nil
}

func (t *txProcessor) processCallFrame(frame *reexec.CallFrame) {
	if frame.Value != nil {
		if frame.Value.Cmp(bigETHTransferThreshold) > 0 {
			txHash := t.txResult.Transaction.Hash()
			logger.Info("Big BNB transfer", "from", frame.From, "to", frame.To, "value", AmountString(frame.Value, 18), "tx", txHash.Hex())
			t.whaleFeed.Send(whalemonitor.WhaleEvent{
				TxHash: txHash,
				From:   frame.From,
				To:     frame.To,
				Value:  frame.Value,
			})
		}
	}
	threshold, exist := bigERC20TransferThreshold[frame.To]
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
			if args.Amount != nil && args.Amount.Cmp(ParseAmount(threshold, token.Decimals)) > 0 {
				logger.Info("Big ERC20 token transfer", "from", frame.From, "to", args.To, "token", token.Symbol, "amount", AmountString(args.Amount, token.Decimals), "tx", txHash.Hex())
				t.whaleFeed.Send(whalemonitor.WhaleEvent{
					TxHash: txHash,
					From:   frame.From,
					To:     args.To,
					Token:  token,
					Value:  args.Amount,
				})
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
			if args.Amount != nil && args.Amount.Cmp(ParseAmount(threshold, token.Decimals)) > 0 {
				logger.Info("Big ERC20 token transfer", "from", args.From, "to", args.To, "token", token.Symbol, "amount", AmountString(args.Amount, token.Decimals), "tx", txHash.Hex())
				t.whaleFeed.Send(whalemonitor.WhaleEvent{
					TxHash: txHash,
					From:   args.From,
					To:     args.To,
					Token:  token,
					Value:  args.Amount,
				})
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
	ctx       *plugin.PluginCtx
	client    *rpc.Client
	whaleFeed event.Feed
}

func (p *WhaleMonitorPlugin) processTx(wg *sync.WaitGroup, state *state.StateDB, txRet *reexec.TxContext) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("panic: %#v\n", err)
		}
		wg.Done()
	}()
	if txRet != nil && !txRet.Reverted {
		proc := txProcessor{
			client:    p.client,
			state:     state,
			txResult:  txRet,
			whaleFeed: &p.whaleFeed,
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

func (p *WhaleMonitorPlugin) SubscribeWhaleEvent(ch chan<- whalemonitor.WhaleEvent) event.Subscription {
	return p.ctx.EventScope.Track(p.whaleFeed.Subscribe(ch))
}

func (p *WhaleMonitorPlugin) initializeNotification() {
	eventCh := make(chan whalemonitor.WhaleEvent)
	subs := p.SubscribeWhaleEvent(eventCh)
	senders := initNotificationSenders(p.ctx)
	go notifyEventLoop(senders, eventCh, subs)
}

func (p *WhaleMonitorPlugin) OnEnable(ctx *plugin.PluginCtx) error {
	client, err := ctx.Node.Attach()
	if err != nil {
		return err
	}
	p.client = client
	p.ctx = ctx
	ctx.Monitor.RegisterProcessor(pluginNamespace, p)
	p.initializeNotification()
	return nil
}

func (p *WhaleMonitorPlugin) OnDisable(ctx *plugin.PluginCtx) error {
	return nil
}

func OnLoad(ctx *plugin.PluginCtx) plugin.Plugin {
	return &WhaleMonitorPlugin{}
}