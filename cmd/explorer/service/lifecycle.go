package service

import (
	"net/http"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rpc"
)

type Context interface {
	RegisterAPIs(api []rpc.API)
	RegisterHandler(handler []http.Handler)
	OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error)
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
