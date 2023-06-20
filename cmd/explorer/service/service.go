package service

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	extNamespace      = "/eth/db/ethexplorer"
	extDatabaseName   = "ethexplorer"
	extDatabaseHandle = 512
	extDatabaseCache  = 1024
)

const (
	stateStopped = iota
	stateRunning
	stateStopping
)

type ServiceStack struct {
	rpcAPIs    []rpc.API                   // List of APIs currently provided by the node
	lifecycles []Lifecycle                 // All registered backends, services, and auxiliary services that have a lifecycle
	state      int32                       // Tracks the current state of the service
	databases  map[string]*closeTrackingDB // All open databases

	lock     sync.Mutex
	quitCh   chan struct{}
	quitLock sync.Mutex
	term     chan struct{}
}

func (s *ServiceStack) Run() error {
	if !atomic.CompareAndSwapInt32(&s.state, stateStopped, stateRunning) {
		return ErrServiceRunning
	}

	// Start all registered lifecycles.
	var err error
	var started []Lifecycle
	for _, lifecycle := range s.lifecycles {
		if err = lifecycle.Start(s); err != nil {
			break
		}
		started = append(started, lifecycle)
	}
	// Check if any lifecycle failed to start.
	if err != nil {
		s.stopServices(started)
	}
	return err
}

func (s *ServiceStack) stopServices(running []Lifecycle) error {
	// Stop running lifecycles in reverse order.
	failure := &StopError{Services: make(map[reflect.Type]error)}
	for i := len(running) - 1; i >= 0; i-- {
		if err := running[i].Stop(s); err != nil {
			failure.Services[reflect.TypeOf(running[i])] = err
		}
	}

	if len(failure.Services) > 0 {
		return failure
	}
	return nil
}

func (s *ServiceStack) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.state, stateRunning, stateStopping) {
		return errors.New("not running")
	}
	log.Info("Stopping explorer service...")
	s.quitLock.Lock()
	select {
	case <-s.quitCh:
	default:
	}
	s.quitLock.Unlock()
	return nil
}

func (s *ServiceStack) Wait() error {
	<-s.term
	return nil
}

func containsLifecycle(lfs []Lifecycle, l Lifecycle) bool {
	for _, obj := range lfs {
		if obj == l {
			return true
		}
	}
	return false
}

func (s *ServiceStack) RegisterLifeCycle(lifecycle Lifecycle) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.state != stateStopped {
		panic("can't register lifecycle when service is running")
	}
	if containsLifecycle(s.lifecycles, lifecycle) {
		panic(fmt.Sprintf("attempt to register lifecycle %T more than once", lifecycle))
	}
	s.lifecycles = append(s.lifecycles, lifecycle)
}

func (s *ServiceStack) RegisterAPIs(api []rpc.API) {
}

func (s *ServiceStack) RegisterHandler(handler []http.Handler) {
}

func (s *ServiceStack) OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error) {
	return nil, nil
}

func (s *ServiceStack) Database(name string) (ethdb.Database, error) {
	if db, exist := s.databases[name]; exist {
		return db, nil
	}
	return nil, ErrNoDatabase
}

func NewServiceStack(cfg *Config) (*ServiceStack, error) {
	instance := &ServiceStack{
		quitCh: make(chan struct{}),
	}
	return instance, nil
}
