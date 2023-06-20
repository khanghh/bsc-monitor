package service

import (
	"net/http"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rpc"
)

// Context provides an interface for lifecycle to do start/stop operations.
type Context interface {
	// RegisterAPIs registers RPC APIs to the HTTP/WS RPC endpoints.
	RegisterAPIs(api []rpc.API)

	// RegisterHandler mounts a handler on the given path on the canonical HTTP server.
	RegisterHandler(name, path string, handler http.Handler)

	// OpenDatabase opens an existing database with the given name (or creates one if none
	// previously exists) within the data directory.
	OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error)

	// Database is a getter for the opened database by namespace.
	Database(namespace string) (ethdb.Database, error)
}

// Lifecycle encompasses the behavior of services that can be started and stopped
// on the node. Lifecycle management is delegated to the node, but it is the
// responsibility of the service-specific package to configure and register the
// service on the node using the `RegisterLifecycle` method.
type Lifecycle interface {
	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start(ctx Context) error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop(ctx Context) error
}

func containsLifecycle(lfs []Lifecycle, l Lifecycle) bool {
	for _, obj := range lfs {
		if obj == l {
			return true
		}
	}
	return false
}
