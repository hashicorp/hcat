package dependency

import (
	"encoding/gob"
	"fmt"
	"net/url"
	"regexp"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*CatalogServiceQuery)(nil)

	// CatalogServiceQueryRe is the regular expression to use.
	CatalogServiceQueryRe = regexp.MustCompile(`\A` + tagRe + serviceNameRe + dcRe + nearRe + `\z`)
)

func init() {
	gob.Register([]*dep.CatalogSnippet{})
}

// CatalogService is a catalog entry in Consul.
type CatalogService struct {
	ID              string
	Node            string
	Address         string
	Datacenter      string
	TaggedAddresses map[string]string
	NodeMeta        map[string]string
	ServiceID       string
	ServiceName     string
	ServiceAddress  string
	ServiceTags     dep.ServiceTags
	ServiceMeta     map[string]string
	ServicePort     int
	Namespace       string
}

// CatalogServiceQuery is the representation of a requested catalog services
// dependency from inside a template.
type CatalogServiceQuery struct {
	isConsul
	stopCh chan struct{}

	dc   string
	name string
	near string
	tag  string
	opts QueryOptions
}

// NewCatalogServiceQuery parses a string into a CatalogServiceQuery.
func NewCatalogServiceQuery(s string) (*CatalogServiceQuery, error) {
	if !CatalogServiceQueryRe.MatchString(s) {
		return nil, fmt.Errorf("catalog.service: invalid format: %q", s)
	}

	m := regexpMatch(CatalogServiceQueryRe, s)
	return &CatalogServiceQuery{
		stopCh: make(chan struct{}, 1),
		dc:     m["dc"],
		name:   m["name"],
		near:   m["near"],
		tag:    m["tag"],
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of CatalogService objects.
func (d *CatalogServiceQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Near:       d.near,
	})

	u := &url.URL{
		Path:     "/v1/catalog/service/" + d.name,
		RawQuery: opts.String(),
	}
	if d.tag != "" {
		q := u.Query()
		q.Set("tag", d.tag)
		u.RawQuery = q.Encode()
	}

	entries, qm, err := clients.Consul().Catalog().Service(d.name, d.tag, opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	var list []*CatalogService
	for _, s := range entries {
		list = append(list, &CatalogService{
			ID:              s.ID,
			Node:            s.Node,
			Address:         s.Address,
			Datacenter:      s.Datacenter,
			TaggedAddresses: s.TaggedAddresses,
			NodeMeta:        s.NodeMeta,
			ServiceID:       s.ServiceID,
			ServiceName:     s.ServiceName,
			ServiceAddress:  s.ServiceAddress,
			ServiceTags:     dep.ServiceTags(deepCopyAndSortTags(s.ServiceTags)),
			ServiceMeta:     s.ServiceMeta,
			ServicePort:     s.ServicePort,
			Namespace:       s.Namespace,
		})
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return list, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogServiceQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *CatalogServiceQuery) ID() string {
	name := d.name
	if d.tag != "" {
		name = d.tag + "." + name
	}
	if d.dc != "" {
		name = name + "@" + d.dc
	}
	if d.near != "" {
		name = name + "~" + d.near
	}
	return fmt.Sprintf("catalog.service(%s)", name)
}

// Stringer interface reuses ID
func (d *CatalogServiceQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *CatalogServiceQuery) Stop() {
	close(d.stopCh)
}
func (d *CatalogServiceQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
