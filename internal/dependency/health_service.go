package dependency

import (
	"encoding/gob"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

const (
	HealthAny      = "any"
	HealthPassing  = "passing"
	HealthWarning  = "warning"
	HealthCritical = "critical"
	HealthMaint    = "maintenance"

	NodeMaint    = "_node_maintenance"
	ServiceMaint = "_service_maintenance:"
)

var (
	// Ensure implements
	_ isDependency = (*HealthServiceQuery)(nil)

	// HealthServiceQueryRe is the regular expression to use.
	HealthServiceQueryRe = regexp.MustCompile(`\A` + tagRe + serviceNameRe + dcRe + nearRe + filterRe + `\z`)

	// queryParamOptRe is the regular expression to distinguish between query
	// params and filters. Query parameters only have one "=" where as filters
	// can have "==" or "!=" operators.
	queryParamOptRe = regexp.MustCompile(`[\w\d\s]=[\w\d\s]`)
)

func init() {
	gob.Register([]*dep.HealthService{})
}

// HealthServiceQuery is the representation of all a service query in Consul.
type HealthServiceQuery struct {
	isConsul
	stopCh chan struct{}

	dc      string
	filter  string
	name    string
	ns      string
	near    string
	connect bool
	opts    QueryOptions

	// deprecatedStatusFilters is a list of check statuses for client-side
	// filtering. Accepted values are the Health* constants above.
	deprecatedStatusFilters []string

	// deprecatedTag is the singular tag parsed from the template argument
	// {{ service "tag.service" }} used for the deprecated tag query parameter.
	// Use the filter parameter with the "Service.Tags" selector instead.
	deprecatedTag string

	// passingOnly filters for services that have an overall aggregated status
	// of passing. When true, sdk adds ?passing=1 to api request
	passingOnly bool
}

// NewHealthServiceQueryV1 processes the strings to build a service dependency.
func NewHealthServiceQueryV1(s string, opts []string) (*HealthServiceQuery, error) {
	return healthServiceQueryV1(s, false, opts)
}

// NewHealthConnectQueryV1 Query processes the strings to build a connect dependency.
func NewHealthConnectQueryV1(s string, opts []string) (*HealthServiceQuery, error) {
	return healthServiceQueryV1(s, true, opts)
}

// NewHealthServiceQuery processes the strings to build a service dependency.
func NewHealthServiceQuery(s string) (*HealthServiceQuery, error) {
	return healthServiceQuery(s, false)
}

// NewHealthConnect Query processes the strings to build a connect dependency.
func NewHealthConnectQuery(s string) (*HealthServiceQuery, error) {
	return healthServiceQuery(s, true)
}

// healthServiceQueryV1 queries the health API with expanded filtering support
func healthServiceQueryV1(service string, connect bool, opts []string) (*HealthServiceQuery, error) {
	if service == "" {
		return nil, fmt.Errorf("health.service: service name required: %q", service)
	}

	healthServiceQuery := HealthServiceQuery{
		stopCh:      make(chan struct{}, 1),
		connect:     connect,
		name:        service,
		passingOnly: true,
	}

	// Split query parameters and filters
	var filters []string
	for _, opt := range opts {
		if strings.TrimSpace(opt) == "" {
			continue
		}

		if queryParamOptRe.MatchString(opt) {
			queryParam := strings.SplitN(opt, "=", 2)
			query := strings.TrimSpace(queryParam[0])
			value := strings.TrimSpace(queryParam[1])
			switch query {
			case "dc", "datacenter":
				healthServiceQuery.dc = value
				continue
			case "ns", "namespace":
				healthServiceQuery.ns = value
				continue
			case "near":
				healthServiceQuery.near = value
				continue
			}
		}

		if strings.Contains(opt, "Checks.Status") {
			// Disable if any filter option includes "Checks.Status"
			healthServiceQuery.passingOnly = false
		}

		// Evaluate the grammer of the filter before attempting to query Consul.
		// Defer to the Consul API to evaluate the kind and type of filter selectors.
		_, err := bexpr.CreateFilter(opt)
		if err != nil {
			return nil, fmt.Errorf(
				"health.service: invalid filter: %q for %q: %s", opt, service, err)
		}
		filters = append(filters, opt)
	}

	if len(filters) > 0 {
		healthServiceQuery.filter = strings.Join(filters, " and ")
	}

	return &healthServiceQuery, nil
}

func healthServiceQuery(s string, connect bool) (*HealthServiceQuery, error) {
	if !HealthServiceQueryRe.MatchString(s) {
		return nil, fmt.Errorf("health.service: invalid format: %q", s)
	}

	m := regexpMatch(HealthServiceQueryRe, s)

	var filters []string
	if filter := m["filter"]; filter != "" {
		split := strings.Split(filter, ",")
		for _, f := range split {
			f = strings.TrimSpace(f)
			switch f {
			case HealthAny,
				HealthPassing,
				HealthWarning,
				HealthCritical,
				HealthMaint:
				filters = append(filters, f)
			case "":
			default:
				return nil, fmt.Errorf(
					"health.service: invalid filter: %q in %q", f, s)
			}
		}
		sort.Strings(filters)
	} else {
		filters = []string{HealthPassing}
	}

	return &HealthServiceQuery{
		stopCh:                  make(chan struct{}, 1),
		dc:                      m["dc"],
		name:                    m["name"],
		near:                    m["near"],
		connect:                 connect,
		deprecatedStatusFilters: filters,
		deprecatedTag:           m["tag"],
		passingOnly:             len(filters) == 1 && filters[0] == HealthPassing,
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of HealthService objects.
func (d *HealthServiceQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Filter:     d.filter,
		Namespace:  d.ns,
		Near:       d.near,
	})

	nodes := clients.Consul().Health().Service
	if d.connect {
		nodes = clients.Consul().Health().Connect
	}
	entries, qm, err := nodes(d.name, d.deprecatedTag, d.passingOnly, opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	list := make([]*dep.HealthService, 0, len(entries))
	for _, entry := range entries {
		// Determine the overall status of this service from its checks.
		status := entry.Checks.AggregatedStatus()

		// Do status filtering on client-side if there are non-passing status filters.
		if !acceptStatus(d.deprecatedStatusFilters, status) {
			continue
		}

		// Get the address of the service, falling back to the address of the
		// node.
		address := entry.Service.Address
		if address == "" {
			address = entry.Node.Address
		}

		list = append(list, &dep.HealthService{
			Node:                entry.Node.Node,
			NodeID:              entry.Node.ID,
			Kind:                string(entry.Service.Kind),
			NodeAddress:         entry.Node.Address,
			NodeDatacenter:      entry.Node.Datacenter,
			NodeTaggedAddresses: entry.Node.TaggedAddresses,
			NodeMeta:            entry.Node.Meta,
			ServiceMeta:         entry.Service.Meta,
			Address:             address,
			ID:                  entry.Service.ID,
			Name:                entry.Service.Service,
			Tags: dep.ServiceTags(
				deepCopyAndSortTags(entry.Service.Tags)),
			Status:    status,
			Checks:    entry.Checks,
			Port:      entry.Service.Port,
			Weights:   entry.Service.Weights,
			Namespace: entry.Service.Namespace,
		})
	}

	// Sort unless the user explicitly asked for nearness
	if d.near == "" {
		sort.Stable(ByNodeThenID(list))
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return list, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *HealthServiceQuery) CanShare() bool {
	return true
}

// Stop halts the dependency's fetch function.
func (d *HealthServiceQuery) Stop() {
	close(d.stopCh)
}

// ID returns the human-friendly version of this dependency.
func (d *HealthServiceQuery) ID() string {
	name := d.name
	if d.deprecatedTag != "" {
		name = d.deprecatedTag + "." + name
	}
	if d.dc != "" {
		name = name + "@" + d.dc
	}
	if d.near != "" {
		name = name + "~" + d.near
	}
	if len(d.deprecatedStatusFilters) > 0 {
		name = name + "|" + strings.Join(d.deprecatedStatusFilters, ",")
	}

	var opts []string
	if d.ns != "" {
		opts = append(opts, fmt.Sprintf("ns=%s", d.ns))
	}
	if d.filter != "" {
		opts = append(opts, fmt.Sprintf("filter=%s", d.filter))
	}
	if len(opts) > 0 {
		name = fmt.Sprintf("%s?%s", name, strings.Join(opts, "&"))
	}
	return fmt.Sprintf("health.service(%s)", name)
}

// Stringer interface reuses ID
func (d *HealthServiceQuery) String() string {
	return d.ID()
}

func (d *HealthServiceQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}

// acceptStatus returns if a check status matches the list of statuses to filter on
func acceptStatus(filters []string, status string) bool {
	if len(filters) == 0 {
		// nothing to filter on, status is acceptable
		return true
	}
	for _, filter := range filters {
		if filter == status || filter == HealthAny {
			return true
		}
	}
	return false
}

// ByNodeThenID is a sortable slice of Service
type ByNodeThenID []*dep.HealthService

// Len, Swap, and Less are used to implement the sort.Sort interface.
func (s ByNodeThenID) Len() int      { return len(s) }
func (s ByNodeThenID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByNodeThenID) Less(i, j int) bool {
	if s[i].Node < s[j].Node {
		return true
	} else if s[i].Node == s[j].Node {
		return s[i].ID <= s[j].ID
	}
	return false
}
