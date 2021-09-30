package dep

import (
	"time"

	"github.com/hashicorp/consul/api"
)

// Node is a node entry in Consul
type Node struct {
	ID              string
	Node            string
	Address         string
	Datacenter      string
	TaggedAddresses map[string]string
	Meta            map[string]string
}

// CatalogNode is a wrapper around the node and its services.
type CatalogNode struct {
	Node     *Node
	Services []*CatalogNodeService
}

// ServiceTags is a slice of tags assigned to a Service
type ServiceTags []string

// CatalogNodeService is a service on a single node.
type CatalogNodeService struct {
	ID                string
	Service           string
	Tags              ServiceTags
	Meta              map[string]string
	Port              int
	Address           string
	EnableTagOverride bool
}

// CatalogSnippet is a catalog entry in Consul.
type CatalogSnippet struct {
	Name string
	Tags ServiceTags
}

// HealthService is a service entry in Consul.
type HealthService struct {
	Node                string
	NodeID              string
	NodeAddress         string
	NodeDatacenter      string
	NodeTaggedAddresses map[string]string
	NodeMeta            map[string]string
	ServiceMeta         map[string]string
	Address             string
	ID                  string
	Name                string
	Kind                string
	Tags                ServiceTags
	Checks              api.HealthChecks
	Status              string
	Port                int
	Weights             api.AgentWeights
	Namespace           string
}

// KvValue is here to type the KV return string
type KvValue string

type KVExists bool

// KeyPair is a simple Key-Value pair
type KeyPair struct {
	Path   string
	Key    string
	Value  string
	Exists bool

	// Lesser-used, but still valuable keys from api.KV
	CreateIndex uint64
	ModifyIndex uint64
	LockIndex   uint64
	Flags       uint64
	Session     string
}

// Secret is the structure returned for every secret within Vault.
type Secret struct {
	// The request ID that generated this response
	RequestID string

	LeaseID       string
	LeaseDuration int
	Renewable     bool

	// Data is the actual contents of the secret. The format of the data
	// is arbitrary and up to the secret backend.
	Data map[string]interface{}

	// Warnings contains any warnings related to the operation. These
	// are not issues that caused the command to fail, but that the
	// client should be aware of.
	Warnings []string

	// Auth, if non-nil, means that there was authentication information
	// attached to this response.
	Auth *SecretAuth

	// WrapInfo, if non-nil, means that the initial response was wrapped in the
	// cubbyhole of the given token (which has a TTL of the given number of
	// seconds)
	WrapInfo *SecretWrapInfo
}

// SecretAuth is the structure containing auth information if we have it.
type SecretAuth struct {
	ClientToken string
	Accessor    string
	Policies    []string
	Metadata    map[string]string

	LeaseDuration int
	Renewable     bool
}

// SecretWrapInfo contains wrapping information if we have it. If what is
// contained is an authentication token, the accessor for the token will be
// available in WrappedAccessor.
type SecretWrapInfo struct {
	Token           string
	TTL             int
	CreationTime    time.Time
	WrappedAccessor string
}
