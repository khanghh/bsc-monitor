//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package extdb

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

func readStateAccount(tr *trie.Trie, addr common.Address) (*types.StateAccount, error) {
	enc, err := tr.TryGet(addr.Bytes())
	if err != nil {
		return nil, err
	}
	state := new(types.StateAccount)
	if len(enc) > 0 {
		if err := rlp.DecodeBytes(enc, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func writeStateAccount(tr *trie.Trie, addr common.Address, state *types.StateAccount) error {
	enc, err := rlp.EncodeToBytes(state)
	if err != nil {
		return err
	}
	return tr.TryUpdate(addr.Bytes(), enc)
}

func increaseNonce(tr *trie.Trie, addr common.Address) error {
	state, err := readStateAccount(tr, addr)
	if err != nil {
		return err
	}
	state.Nonce += 1
	return writeStateAccount(tr, addr, state)
}

type accState struct {
	SentTxs uint64
}

func (s *accState) MarshalRLP() []byte {
	data, _ := rlp.EncodeToBytes(s)
	return data
}

func (s *accState) UnmarshalRLP(data []byte) error {
	return rlp.DecodeBytes(data, s)
}

func TestSnapshotDB(t *testing.T) {
	diskdb := rawdb.NewMemoryDatabase()
	triedb := trie.NewDatabase(rawdb.NewMemoryDatabase())
	snap := NewSnapshotDB(diskdb, triedb)

	tr, _ := trie.New(nilHash, triedb)

	acc1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	acc2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	// acc3 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	// txs1 := []common.Hash{
	// 	common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
	// 	common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
	// }
	// txs2 := []common.Hash{
	// 	common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"),
	// 	common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444"),
	// }

	if err := increaseNonce(tr, acc1); err != nil {
		panic(err)
	}
	if err := increaseNonce(tr, acc2); err != nil {
		panic(err)
	}
	root, _, err := tr.Commit(nil)
	if err != nil {
		panic(err)
	}
	changes := make(map[common.Address]ExtSateRLP)
	changes[acc1] = &accState{
		SentTxs: 156759,
	}
	changes[acc2] = &accState{
		SentTxs: 1778,
	}
	if err := snap.Update(root, changes); err != nil {
		panic(err)
	}
	enc1, err := snap.getAccountState(tr, acc1)
	if err != nil {
		panic(err)
	}
	state1 := new(accState)
	state1.UnmarshalRLP(enc1.ExtraData)
	fmt.Println(state1)

	// state1, err := readStateAccount(tr, acc1)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(state1.Nonce)
}
