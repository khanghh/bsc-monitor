//
// Created on 2023/2/22 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type MissingTrieError struct {
	Root common.Hash // state root of the missing node
}

func (err *MissingTrieError) Error() string {
	return fmt.Sprintf("missing trie node %x", err.Root)
}

type NoAccountStateError struct {
	Root    common.Hash // state root of the missing node
	Address common.Address
}

func (err *NoAccountStateError) Error() string {
	return fmt.Sprintf("no state found for account %x at root %x", err.Address, err.Root)
}

type NoAccountInfoError struct {
	Account common.Address
}

func (err *NoAccountInfoError) Error() string {
	return fmt.Sprintf("account info of %x not found", err.Account)
}

type NoContractInfoError struct {
	Address common.Address
}

func (err *NoContractInfoError) Error() string {
	return fmt.Sprintf("contract info of %x not found", err.Address)
}
