//
// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab
//

package task

import (
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type ReExecProcessor interface {
	ProcessTx(state *state.StateDB, block *types.Block, txIndex int) error
}

type ReExecOptions struct {
	Name       string
	NoState    bool              // Disable state generation
	StartBlock uint64            // Starting block number to process. If not provided, latest block will used instead.
	EndBlock   uint64            // Ending block number to process. If not provided, task keep processing every new block.
	Processors []ReExecProcessor // List of processors to process the transactions
}

// reexecTask re-execute transactions to get blockchain state and
// call registered processor to analyze the transactions
type reexecTask struct {
	*ReExecOptions
	stateCache   *state.StateDB
	currentBlock uint64
	status       uint64
	termCh       chan struct{}
}

func (t *reexecTask) Status() uint64 {
	return 0
}

func (t *reexecTask) Run() error {
	return nil
}

func (t *reexecTask) Wait() {
	<-t.termCh
}

func (t *reexecTask) Pause() {
	close(t.termCh)
}

func (t *reexecTask) Resume() {
	close(t.termCh)
}

func (t *reexecTask) Abort() {
	close(t.termCh)
}

func NewReExecTask(opts *ReExecOptions) *reexecTask {
	return &reexecTask{
		ReExecOptions: opts,
		termCh:        make(chan struct{}),
	}
}
