package dep

import (
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	vaultapi "github.com/hashicorp/vault/api"
)

// Dependency is an interface for an external dependency to be monitored.
type Dependency interface {
	Fetch(Clients) (interface{}, *ResponseMetadata, error)
	ID() string
	Stop()
	fmt.Stringer
}

// Clients interface for the API clients used for external dependency calls.
type Clients interface {
	Consul() *consulapi.Client
	Vault() *vaultapi.Client
}

// Metadata returned by external dependency Fetch-ing.
// LastIndex is used with the Consul backend. Needed to track changes.
// LastContact is used to help calculate staleness of records.
type ResponseMetadata struct {
	LastIndex   uint64
	LastContact time.Duration
}
