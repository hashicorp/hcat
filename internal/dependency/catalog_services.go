package dependency

import (
	"encoding/gob"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*CatalogServicesQuery)(nil)

	// CatalogServicesQueryRe is the regular expression to use for CatalogNodesQuery.
	CatalogServicesQueryRe = regexp.MustCompile(`\A` + dcRe + `\z`)
)

func init() {
	gob.Register([]*dep.CatalogSnippet{})
}

// CatalogServicesQuery is the representation of a requested catalog service
// dependency from inside a template.
type CatalogServicesQuery struct {
	isConsul
	stopCh chan struct{}

	dc       string
	ns       string
	nodeMeta map[string]string
	opts     QueryOptions
}

// NewCatalogServicesQueryV1 processes options in the format of "key=value"
// e.g. "dc=dc1"
func NewCatalogServicesQueryV1(opts []string) (*CatalogServicesQuery, error) {
	catalogServicesQuery := CatalogServicesQuery{
		stopCh: make(chan struct{}, 1),
	}

	for _, opt := range opts {
		if strings.TrimSpace(opt) == "" {
			continue
		}

		query, value, err := stringsSplit2(opt, "=")
		if err != nil {
			return nil, fmt.Errorf(
				"catalog.services: invalid query parameter format: %q", opt)
		}
		switch query {
		case "dc", "datacenter":
			catalogServicesQuery.dc = value
		case "ns", "namespace":
			catalogServicesQuery.ns = value
		case "node-meta":
			if catalogServicesQuery.nodeMeta == nil {
				catalogServicesQuery.nodeMeta = make(map[string]string)
			}
			k, v, err := stringsSplit2(value, ":")
			if err != nil {
				return nil, fmt.Errorf(
					"catalog.services: invalid format for query parameter %q: %s",
					query, value,
				)
			}
			catalogServicesQuery.nodeMeta[k] = v
		default:
			return nil, fmt.Errorf(
				"catalog.services: invalid query parameter: %q", opt)
		}
	}

	return &catalogServicesQuery, nil
}

// NewCatalogServicesQuery parses a string of the format @dc.
func NewCatalogServicesQuery(s string) (*CatalogServicesQuery, error) {
	if !CatalogServicesQueryRe.MatchString(s) {
		return nil, fmt.Errorf("catalog.services: invalid format: %q", s)
	}

	m := regexpMatch(CatalogServicesQueryRe, s)
	return &CatalogServicesQuery{
		stopCh: make(chan struct{}, 1),
		dc:     m["dc"],
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of CatalogService objects.
func (d *CatalogServicesQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Namespace:  d.ns,
	}).ToConsulOpts()
	// node-meta is handled specifically for /v1/catalog/services endpoint since
	// it does not support the preferred filter option.
	opts.NodeMeta = d.nodeMeta

	entries, qm, err := clients.Consul().Catalog().Services(opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	var catalogServices []*dep.CatalogSnippet
	for name, tags := range entries {
		catalogServices = append(catalogServices, &dep.CatalogSnippet{
			Name: name,
			Tags: dep.ServiceTags(deepCopyAndSortTags(tags)),
		})
	}

	sort.Stable(ByName(catalogServices))

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return catalogServices, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogServicesQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *CatalogServicesQuery) ID() string {
	var opts []string
	if d.dc != "" {
		opts = append(opts, fmt.Sprintf("@%s", d.dc))
	}
	if d.ns != "" {
		opts = append(opts, fmt.Sprintf("ns=%s", d.ns))
	}
	for k, v := range d.nodeMeta {
		opts = append(opts, fmt.Sprintf("node-meta=%s:%s", k, v))
	}
	if len(opts) > 0 {
		sort.Strings(opts)
		return fmt.Sprintf("catalog.services(%s)", strings.Join(opts, "&"))
	}
	return "catalog.services"
}

// Stringer interface reuses ID
func (d *CatalogServicesQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *CatalogServicesQuery) Stop() {
	close(d.stopCh)
}

func (d *CatalogServicesQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}

// ByName is a sortable slice of CatalogService structs.
type ByName []*dep.CatalogSnippet

func (s ByName) Len() int      { return len(s) }
func (s ByName) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByName) Less(i, j int) bool {
	if s[i].Name <= s[j].Name {
		return true
	}
	return false
}

// stringsSplit2 splits a string
func stringsSplit2(s string, sep string) (string, string, error) {
	split := strings.Split(s, sep)
	if len(split) != 2 {
		return "", "", fmt.Errorf("unexpected split on separator %q: %s", sep, s)
	}
	return strings.TrimSpace(split[0]), strings.TrimSpace(split[1]), nil
}
