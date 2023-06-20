package service

import (
	"errors"
	"fmt"
	"reflect"
	"syscall"
)

var (
	ErrDatadirUsed     = errors.New("datadir already used by another process")
	ErrServiceStopping = errors.New("service is stopping")
	ErrServiceRunning  = errors.New("service already running")
	ErrServiceStopped  = errors.New("service already stoppped")
	ErrServiceUnknown  = errors.New("unknown service")
	ErrNoDatabase      = errors.New("no database")

	datadirInUseErrnos = map[uint]bool{11: true, 32: true, 35: true}
)

func convertFileLockError(err error) error {
	if errno, ok := err.(syscall.Errno); ok && datadirInUseErrnos[uint(errno)] {
		return ErrDatadirUsed
	}
	return err
}

// StopError is returned if a Node fails to stop either any of its registered
// services or itself.
type StopError struct {
	Server   error
	Services map[reflect.Type]error
}

// Error generates a textual representation of the stop error.
func (e *StopError) Error() string {
	return fmt.Sprintf("server: %v, services: %v", e.Server, e.Services)
}
