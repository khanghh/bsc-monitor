package main

import (
	"fmt"
	"sync/atomic"

	"github.com/dop251/goja"
	"github.com/khanghh/goja-nodejs/eventloop"
	"github.com/khanghh/goja-nodejs/require"
)

type RunnerState int32

const (
	StateStopped RunnerState = iota
	StateRunning
	StateInterrupted
)

func (s RunnerState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateInterrupted:
		return "interupted"
	case StateRunning:
		return "running"
	}
	return "unknown"
}

// JsRunner represents a JavaScript runner instance.
// It manages the execution of JavaScript code within a Goja runtime environment,
// communicates with an Ethereum RPC client, and writes output logs.
type JsRunner struct {
	Name    string               // A unique identifier for the runner instance
	runtime *goja.Runtime        // The Goja JavaScript runtime for executing code
	loop    *eventloop.EventLoop // The Goja JavaScript event loop
	state   RunnerState          // Current state of the runner
	stopCh  chan struct{}
}

func (r *JsRunner) CurrentState() RunnerState {
	return RunnerState(atomic.LoadInt32((*int32)(&r.state)))
}

func (r *JsRunner) Running() bool {
	return r.CurrentState() == StateRunning
}

func (r *JsRunner) Wait() {
	<-r.stopCh
}

func (r *JsRunner) Stop() error {
	if !atomic.CompareAndSwapInt32((*int32)(&r.state), int32(StateRunning), int32(StateInterrupted)) {
		return fmt.Errorf("not running")
	}
	r.loop.Stop()
	close(r.stopCh)
	return nil
}

// CompileAndRun is a method for the JsRunner struct which compiles and runs the provided JavaScript code.
//
// It takes two arguments:
// 1. filename: The name or path of the JavaScript module being executed (e.g., "web3/util" or "./path/to/module/index.js").
// 2. code: The JavaScript code to be compiled and executed.
//
// The current directory of the execution is determined by the given name's parent directory.
// For example, if the name is "./path/to/module/index.js",
// then the current directory during execution will be "./path/to/module".
//
// When requiring another module (e.g., require('./moduleA')),
// it will resolve to a file named "moduleA.js" within the current directory (e.g., "./path/to/module/moduleA.js").
func (r *JsRunner) CompileAndRun(filename, code string) (ret goja.Value, err error) {
	state := atomic.LoadInt32((*int32)(&r.state))
	if state == int32(StateRunning) {
		return nil, fmt.Errorf("runner is busy")
	}
	atomic.StoreInt32((*int32)(&r.state), int32(StateRunning))
	r.stopCh = make(chan struct{})
	r.loop.Run(func(vm *goja.Runtime) {
		ret, err = vm.RunScript(filename, code)
	})
	if !atomic.CompareAndSwapInt32((*int32)(&r.state), int32(StateRunning), int32(StateStopped)) {
		return nil, fmt.Errorf("interrupted")
	}
	close(r.stopCh)
	return ret, err
}

func newRunner(name string, registry *require.Registry) *JsRunner {
	loop := eventloop.NewEventLoop(eventloop.WithRegistry(registry))
	runtime := loop.Runtime()
	runtime.SetRandSource(randomSource().Float64)
	return &JsRunner{
		Name:    name,
		runtime: runtime,
		loop:    loop,
		stopCh:  make(chan struct{}),
	}
}
