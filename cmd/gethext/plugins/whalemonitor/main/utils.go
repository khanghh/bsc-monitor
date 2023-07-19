package main

import (
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/cmd/gethext/abiutils"
	"github.com/ethereum/go-ethereum/common"
)

type ERC20TransferArgs struct {
	From   common.Address
	To     common.Address
	Amount *big.Int
}

func ParseERC20TransferArgs(sender common.Address, erc20 *abiutils.Interface, input []byte) (*ERC20TransferArgs, error) {
	methodSig, data := input[0:4], input[4:]
	method, err := erc20.MethodById(methodSig)
	if err != nil {
		return nil, err
	}
	if method.RawName == "transfer" || method.RawName == "transferFrom" {
		var args ERC20TransferArgs
		args.From = sender
		if err := erc20.UnpackInput(&args, method.RawName, data); err != nil {
			return nil, err
		}
		return &args, nil
	}
	return nil, errors.New("not a token transfer")
}

func AmountFloat64(val *big.Int, decimals uint64) float64 {
	expDec := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	bigFloatVal := new(big.Float).SetInt(val)
	ret, _ := new(big.Float).Quo(bigFloatVal, expDec).Float64()
	return ret
}

func AmountUint64(val *big.Int, decimals uint64) uint64 {
	expDec := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	return new(big.Int).Quo(val, expDec).Uint64()
}

func AmountString(val *big.Int, decimals uint64) string {
	expDec := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	bigFloatVal := new(big.Float).SetInt(val)
	return new(big.Float).Quo(bigFloatVal, expDec).String()
}

func ParseAmount(val float64, decimals uint64) *big.Int {
	multiplier := new(big.Float).SetFloat64(float64(val) * math.Pow10(int(decimals)))
	ret, _ := multiplier.Int(new(big.Int))
	return ret
}
