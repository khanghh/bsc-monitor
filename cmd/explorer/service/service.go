package service

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/tsdb/fileutil"
)

const (
	stateStopped = iota
	stateRunning
	stateStopping
)

type ServiceStack struct {
	config     *Config
	rpcAPIs    []rpc.API                   // List of APIs currently provided by the node
	lifecycles []Lifecycle                 // All registered backends, services, and auxiliary services that have a lifecycle
	state      int32                       // Tracks the current state of the service
	databases  map[string]*closeTrackingDB // All open databases
	dirLock    fileutil.Releaser           // prevents concurrent use of instance directory

	log    log.Logger    // Logger used by service stack
	lock   sync.Mutex    // Lockere for registration of lifecycles, RPC apis, HTTP handlers
	stopCh chan struct{} // Channel to signal service stack termination
}

func (n *ServiceStack) openDataDir() error {
	if n.config.DataDir == "" {
		return nil
	}

	instdir := filepath.Join(n.config.DataDir, n.config.Name)
	if err := os.MkdirAll(instdir, 0700); err != nil {
		return err
	}
	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	release, _, err := fileutil.Flock(filepath.Join(instdir, "LOCK"))
	if err != nil {
		return convertFileLockError(err)
	}
	n.dirLock = release
	return nil
}

func (n *ServiceStack) closeDataDir() {
	// Release instance directory lock.
	if n.dirLock != nil {
		if err := n.dirLock.Release(); err != nil {
			n.log.Error("Can't release datadir lock", "err", err)
		}
		n.dirLock = nil
	}
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

// doStop is the implementation of Stop, it stops all lifecycle, all openned databases and realease data directory lock
func (s *ServiceStack) doStop(running []Lifecycle) error {
	defer atomic.StoreInt32(&s.state, stateStopped)

	var errs = []error{}
	if err := s.stopServices(running); err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, s.closeDatabases()...)

	// Release data directory lock
	s.closeDataDir()

	// Unlock s.Wait()
	close(s.stopCh)

	// Report any errors that might have occurred.
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("%v", errs)
	}
}

// Run starts all registered lifecycles, RPC services and  HTTP handlers
func (s *ServiceStack) Run() error {
	if atomic.LoadInt32(&s.state) != stateStopped {
		return ErrServiceRunning
	}

	if err := s.openDataDir(); err != nil {
		return err
	}

	// Set service stack state to runnning
	atomic.StoreInt32(&s.state, stateRunning)

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
		return s.doStop(started)
	}
	return err
}

// Stop stops the service stack and releases resources
func (s *ServiceStack) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.state, stateRunning, stateStopping) {
		return errors.New("not running")
	}
	log.Info("Stopping explorer service...")
	return s.doStop(s.lifecycles)
}

// Wait waits for service stack to stop
func (s *ServiceStack) Wait() error {
	if atomic.LoadInt32(&s.state) == stateStopped {
		return ErrServiceStopped
	}
	<-s.stopCh
	return nil
}

// RegisterLifecycle registers the given Lifecycle on the node.
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

// RegisterAPIs registers the APIs a service provides on the node.
func (s *ServiceStack) RegisterAPIs(apis []rpc.API) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.state != stateRunning {
		panic("can't register APIs on running/stopped node")
	}
	s.rpcAPIs = append(s.rpcAPIs, apis...)
}

// RegisterHandler mounts a handler on the given path on the canonical HTTP server.
func (s *ServiceStack) RegisterHandler(name, path string, handler http.Handler) {
	s.lock.Lock()
	defer s.lock.Unlock()
}

// OpenDatabase opens an existing database with the given name (or creates one if no
// previous can be found) from within the node's instance directory.
func (s *ServiceStack) OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error) {
	return nil, nil
}

// Database is getter for openned database by namespace
func (s *ServiceStack) Database(name string) (ethdb.Database, error) {
	if db, exist := s.databases[name]; exist {
		return db, nil
	}
	return nil, ErrNoDatabase
}

func NewServiceStack(config *Config) (*ServiceStack, error) {
	instance := &ServiceStack{
		config:    config,
		databases: make(map[string]*closeTrackingDB),
		stopCh:    make(chan struct{}),
	}
	return instance, nil
}