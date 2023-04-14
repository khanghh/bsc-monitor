package parlia

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/systemcontracts"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
)

func (p *Parlia) getCurrentValidatorsBeforeLuban(blockHash common.Hash, blockNum *big.Int) ([]common.Address, error) {
	// method
	method := "getValidators"
	if p.chainConfig.IsEuler(blockNum) {
		method = "getMiningValidators"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	data, err := p.validatorSetABI.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for getValidators", "error", err)
		return nil, err
	}
	// call
	header := p.ethAPI.Chain().GetHeader(blockHash, blockNum.Uint64())
	state, err := p.ethAPI.Chain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}
	msgData := (hexutil.Bytes)(data)
	toAddress := common.HexToAddress(systemcontracts.ValidatorContract)
	gas := (hexutil.Uint64)(uint64(math.MaxUint64 / 2))
	result, err := p.doCall(ctx, state, header, ethapi.TransactionArgs{
		Gas:  &gas,
		To:   &toAddress,
		Data: &msgData,
	})
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		log.Error("Call ValidatorContract failed", "method", method, "error", err)
		return nil, result.Err
	}

	ret := []common.Address{}
	if err := p.validatorSetABI.UnpackIntoInterface(&ret, method, result.Return()); err != nil {
		return nil, err
	}
	return ret, nil
}
