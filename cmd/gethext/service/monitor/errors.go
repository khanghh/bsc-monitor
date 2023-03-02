//
// Created on 2023/2/22 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package monitor

import (
	"errors"
)

var (
	ErrMissingTrieNode     = errors.New("missing trie node")
	ErrNoAccountIndexState = errors.New("account index state not found")
	ErrNoAccountState      = errors.New("account state not found")
	ErrNoAccountInfo       = errors.New("account info not found")
	ErrNoContractInfo      = errors.New("contract info not found")
)
