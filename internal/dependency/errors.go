package dependency

import (
	"errors"

	consulapi "github.com/hashicorp/consul/api"
)

// ErrStopped is a special error that is returned when a dependency is
// prematurely stopped, usually due to a configuration reload or a process
// interrupt.
var ErrStopped = errors.New("dependency stopped")

// ErrContinue is a special error which says to continue (retry) on error.
var ErrContinue = errors.New("dependency continue")

var ErrLeaseExpired = errors.New("lease expired or is not renewable")

// ConsulAPIStatus contains information about the api status from Consul
type ConsulAPIStatus struct {
	Code int
	Body string
}

// DecodeConsulStatusError returns the decoded parameters
// from a Consul API StatusError as a ConsulAPIStatus
func DecodeConsulStatusError(err error) (ConsulAPIStatus, bool) {
	var serr consulapi.StatusError
	if errors.As(err, &serr) {
		return ConsulAPIStatus{serr.Code, serr.Body}, true
	}

	return ConsulAPIStatus{0, ""}, false
}
