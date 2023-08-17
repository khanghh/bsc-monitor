package indexer

import (
	"errors"
)

var (
	ErrMissingTrieNode = errors.New("missing trie node")
	ErrNoAccountStats  = errors.New("account statistics not found")
	ErrNoAccountState  = errors.New("account state not found")
	ErrNoAccountInfo   = errors.New("account info not found")
	ErrNoIndexMetadata = errors.New("account index metadata not found")
	ErrNoContractInfo  = errors.New("contract info not found")
)
