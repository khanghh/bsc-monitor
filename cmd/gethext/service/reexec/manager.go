package reexec

import (
	"errors"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

const (
	ReExecMaxTaskCount = 100
	ReExecKillTimeout  = 10 * time.Second
)

var (
	ErrTaskLimitReached  = errors.New("max re-exec task limit reached")
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotExists     = errors.New("task does not exist")
)

type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusPrepairing
	StatusRunning
	StatusPaused
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusPrepairing:
		return "prepairing"
	case StatusRunning:
		return "runnnig"
	case StatusPaused:
		return "paused"
	}
	return "stopped"
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
	tasks  map[string]Task
	wg     sync.WaitGroup
	mtx    sync.Mutex
	quitCh chan struct{}
}

func (tm *TaskManager) RunTask(opts ReExecOptions) (Task, error) {
	return nil, nil
}

func (tm *TaskManager) CancelTask(name string) error {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	if task, exists := tm.tasks[name]; exists {
		errCh := make(chan error)
		go func() {
			task.Abort()
			errCh <- nil
		}()
		select {
		case <-time.After(ReExecKillTimeout):
			log.Error("Could not kill task " + name)
		case <-errCh:
			delete(tm.tasks, name)
		}
	}
	return nil
}

func (tm *TaskManager) KillTask(name string) error {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	if task, exists := tm.tasks[name]; exists {
		errCh := make(chan error)
		go func() {
			task.Abort()
			errCh <- nil
		}()
		select {
		case <-time.After(ReExecKillTimeout):
			log.Error("Could not kill task " + name)
		case <-errCh:
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
	tm.mtx.Lock()
	defer tm.mtx.Unlock()
	close(tm.quitCh)
	for taskName := range tm.tasks {
		tm.KillTask(taskName)
	}
	tm.wg.Wait()
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
