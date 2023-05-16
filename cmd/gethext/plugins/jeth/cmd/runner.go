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
	StateRunnig
	StateInterrupted
)

// JsRunner represents a JavaScript runner instance.
// It manages the execution of JavaScript code within a Goja runtime environment,
// communicates with an Ethereum RPC client, and writes output logs.
type JsRunner struct {
	Name    string               // A unique identifier for the runner instance
	runtime *goja.Runtime        // The Goja JavaScript runtime for executing code
	loop    *eventloop.EventLoop // The Goja JavaScript event loop
	state   RunnerState
	term    chan struct{}
}

func (r *JsRunner) Status() string {
	switch r.state {
	case StateStopped:
		return "stopped"
	case StateInterrupted:
		return "interupted"
	case StateRunnig:
		return "running"
	}
	return "unknown"
}

func (r *JsRunner) Wait() error {
	<-r.term
	return nil
}

func (r *JsRunner) Stop() error {
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
func (r *JsRunner) CompileAndRun(filename, code string) error {
	if !atomic.CompareAndSwapInt32((*int32)(&r.state), int32(StateStopped), int32(StateRunnig)) {
		return fmt.Errorf("runner is busy")
	}
	r.term = make(chan struct{})
	r.loop.Run(func(vm *goja.Runtime) {
		_, err := vm.RunScript(filename, code)
		if err != nil {
			fmt.Println("error", err)
		}
		close(r.term)
	})
	<-r.term
	return nil
}

func newRunner(name string, registry *require.Registry) *JsRunner {
	loop := eventloop.NewEventLoop(eventloop.WithRegistry(registry))
	return &JsRunner{
		Name:    name,
		runtime: loop.Runtime(),
		loop:    loop,
		term:    make(chan struct{}),
	}
}
