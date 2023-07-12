package main

import (
	"testing"
	"time"

	"github.com/khanghh/goja-nodejs/console"
	"github.com/khanghh/goja-nodejs/require"
	"github.com/stretchr/testify/assert"
)

func TestJSRunner_CompileAndRun(t *testing.T) {
	filename := "main.js"
	code := `(function test() {
		return "passed!";
	})(); `

	registry := require.NewRegistry()
	registry.RegisterNativeModule(console.ModuleName, console.Default())

	runner := newRunner("test", registry)
	ret, err := runner.CompileAndRun(filename, code)
	if err != nil {
		t.Fatal(err)
	}

	if !ret.StrictEquals(runner.runtime.ToValue("passed!")) {
		t.Fatalf("Unexpected result: %v", ret)
	}
}

func TestJSRunner_Wait(t *testing.T) {
	filename := "main.js"
	code := `
	var count = 0;
	var t = setInterval(function() {
		console.log("tick");
		if (++count > 2) {
			clearInterval(t);
		}
	}, 1000);`

	registry := require.NewRegistry()
	registry.RegisterNativeModule(console.ModuleName, console.Default())
	runner := newRunner("test", registry)
	var err error
	doneCh := make(chan struct{})
	go func() {
		_, err = runner.CompileAndRun(filename, code)
		close(doneCh)
	}()
	time.Sleep(1 * time.Second)
	runner.Wait()
	<-doneCh
	assert.Equal(t, err, nil)
	assert.Equal(t, runner.CurrentState(), StateStopped)
	assert.Equal(t, runner.loop.Running(), false)
}

func TestJSRunner_Stop(t *testing.T) {
	filename := "main.js"
	code := `
	var count = 0;
	var t = setInterval(function() {
		console.log("tick");
		if (++count > 2) {
			clearInterval(t);
		}
	}, 1000);`

	registry := require.NewRegistry()
	registry.RegisterNativeModule(console.ModuleName, console.Default())
	runner := newRunner("test", registry)
	var err error
	doneCh := make(chan struct{})
	go func() {
		_, err = runner.CompileAndRun(filename, code)
		close(doneCh)
	}()
	time.Sleep(1 * time.Second)
	runner.Stop()
	<-doneCh
	assert.EqualError(t, err, "interrupted")
	assert.Equal(t, runner.CurrentState(), StateInterrupted)
	assert.Equal(t, runner.loop.Running(), false)
}
