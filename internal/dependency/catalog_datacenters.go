package dependency

import (
	"sort"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*CatalogDatacentersQuery)(nil)

	// CatalogDatacentersQuerySleepTime is the amount of time to sleep between
	// queries, since the endpoint does not support blocking queries.
	CatalogDatacentersQuerySleepTime = 15 * time.Second
)

// CatalogDatacentersQuery is the dependency to query all datacenters
type CatalogDatacentersQuery struct {
	isConsul
	ignoreFailing bool
	stopCh        chan struct{}
	opts          QueryOptions
}

// NewCatalogDatacentersQuery creates a new datacenter dependency.
func NewCatalogDatacentersQuery(ignoreFailing bool) (*CatalogDatacentersQuery, error) {
	return &CatalogDatacentersQuery{
		ignoreFailing: ignoreFailing,
		stopCh:        make(chan struct{}, 1),
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of strings representing the datacenters
func (d *CatalogDatacentersQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	opts := d.opts.Merge(&QueryOptions{})

	// This is pretty ghetto, but the datacenters endpoint does not support
	// blocking queries, so we are going to "fake it until we make it". When we
	// first query, the LastIndex will be "0", meaning we should immediately
	// return data, but future calls will include a LastIndex. If we have a
	// LastIndex in the query metadata, sleep for 15 seconds before asking Consul
	// again.
	//
	// This is probably okay given the frequency in which datacenters actually
	// change, but is technically not edge-triggering.
	if opts.WaitIndex != 0 {
		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(CatalogDatacentersQuerySleepTime):
		}
	}

	result, err := clients.Consul().Catalog().Datacenters()
	if err != nil {
		return nil, nil, errors.Wrapf(err, d.ID())
	}

	// If the user opted in for skipping "down" datacenters, figure out which
	// datacenters are down.
	if d.ignoreFailing {
		dcs := make([]string, 0, len(result))
		for _, dc := range result {
			if _, _, err := clients.Consul().Catalog().Services(&api.QueryOptions{
				Datacenter:        dc,
				AllowStale:        false,
				RequireConsistent: true,
			}); err == nil {
				dcs = append(dcs, dc)
			}
		}
		result = dcs
	}

	sort.Strings(result)

	return respWithMetadata(result)
}

// CanShare returns if this dependency is shareable.
func (d *CatalogDatacentersQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *CatalogDatacentersQuery) ID() string {
	return "catalog.datacenters"
}

// Stringer interface reuses ID
func (d *CatalogDatacentersQuery) String() string {
	return d.ID()
}

// Stop terminates this dependency's fetch.
func (d *CatalogDatacentersQuery) Stop() {
	close(d.stopCh)
}

func (d *CatalogDatacentersQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
