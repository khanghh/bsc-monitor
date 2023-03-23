// Created on 2023/2/21 by khanghh
// Project: github.com/verichains/chain-monitor
// Copyright (c) 2023 Verichains Lab

package task

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

const (
	TaskMaxCount    = 100
	TaskKillTimeout = 10 * time.Second
)

var (
	ErrTaskLimitReached  = errors.New("maximum task limit reached")
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotExists     = errors.New("task does not exist")
	ErrTaskKillTimedOut  = errors.New("task kill timed out")
)

type TaskStatus uint32

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusPaused
	StatusStopped
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "runnnig"
	case StatusPaused:
		return "paused"
	case StatusStopped:
		return "stopped"
	}
	return "unknown"
}

type Task interface {
	Status() TaskStatus
	Run()
	Wait()
	Pause()
	Resume()
	Abort()
}

type TaskManager struct {
	tasks    map[string]Task
	wg       sync.WaitGroup
	mtx      sync.Mutex
	quitLock sync.Mutex
	quitCh   chan struct{}
}

func (tm *TaskManager) RunTask(name string, task Task) error {
	if err := tm.AddTask(name, task); err != nil {
		return err
	}
	go task.Run()
	return nil
}

func (tm *TaskManager) AddTask(name string, task Task) error {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	if _, exist := tm.tasks[name]; exist {
		return ErrTaskAlreadyExists
	}
	tm.tasks[name] = task
	return nil
}

func (tm *TaskManager) KillTask(name string) error {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	if task, exists := tm.tasks[name]; exists {
		termCh := make(chan struct{})
		go func() {
			task.Abort()
			task.Wait()
			termCh <- struct{}{}
		}()
		select {
		case <-time.After(TaskKillTimeout):
			log.Error(fmt.Sprintf("Could not kill task `%s`", name), "error", ErrTaskKillTimedOut)
			return ErrTaskKillTimedOut
		case <-termCh:
			delete(tm.tasks, name)
		}
	}
	return nil
}

func (tm *TaskManager) GetTask(name string) (Task, error) {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	task, exists := tm.tasks[name]
	if exists {
		return task, nil
	}
	return nil, ErrTaskNotExists
}

func (tm *TaskManager) Stop() {
	tm.quitLock.Lock()
	defer tm.quitLock.Unlock()
	close(tm.quitCh)
	for taskName := range tm.tasks {
		tm.KillTask(taskName)
	}
	tm.wg.Wait()
	log.Info("TaskManager stopped")
}

func NewTaskManager(cfg *Config) (*TaskManager, error) {
	if err := cfg.Sanitize(); err != nil {
		return nil, err
	}
	return &TaskManager{
		tasks:  make(map[string]Task),
		quitCh: make(chan struct{}),
	}, nil
}
