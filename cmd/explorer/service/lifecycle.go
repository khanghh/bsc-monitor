package service

// Lifecycle is an interface managed by the ServiceStack. Each separate service, backend, or auxiliary service that has a
// lifecycle is considered here. ServiceStack handles the starting and stopping states of the Lifecycle.
type Lifecycle interface {
	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start() error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}

func containsLifecycle(lfs []Lifecycle, l Lifecycle) bool {
	for _, obj := range lfs {
		if obj == l {
			return true
		}
	}
	return false
}
