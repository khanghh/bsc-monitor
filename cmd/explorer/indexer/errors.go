package indexer

import (
	"errors"
)

var (
	ErrMissingTrieNode   = errors.New("missing trie node")
	ErrNoAccountInfo     = errors.New("account info not found")
	ErrNoContractInfo    = errors.New("contract info not found")
	ErrNoAccountStats    = errors.New("account statistics not found")
	ErrIndexLayerStale   = errors.New("index layer stale")
	ErrIndexLayerMissing = errors.New("index layer missing")
	ErrCircularUpdate    = errors.New("circular update index layer")
)
