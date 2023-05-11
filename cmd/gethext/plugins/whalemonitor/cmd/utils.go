package main

import (
	"math"
	"math/big"
)

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
