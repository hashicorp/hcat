package dependency

import (
	"encoding/gob"
	"fmt"
	"regexp"
	"sort"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*CatalogNodesQuery)(nil)

	// CatalogNodesQueryRe is the regular expression to use.
	CatalogNodesQueryRe = regexp.MustCompile(`\A` + dcRe + nearRe + `\z`)
)

func init() {
	gob.Register([]*dep.Node{})
}

// CatalogNodesQuery is the representation of all registered nodes in Consul.
type CatalogNodesQuery struct {
	isConsul
	stopCh chan struct{}

	dc   string
	near string
	opts QueryOptions
}

// NewCatalogNodesQuery parses the given string into a dependency. If the name is
// empty then the name of the local agent is used.
func NewCatalogNodesQuery(s string) (*CatalogNodesQuery, error) {
	if !CatalogNodesQueryRe.MatchString(s) {
		return nil, fmt.Errorf("catalog.nodes: invalid format: %q", s)
	}

	m := regexpMatch(CatalogNodesQueryRe, s)
	return &CatalogNodesQuery{
		dc:     m["dc"],
		near:   m["near"],
		stopCh: make(chan struct{}, 1),
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of Node objects
func (d *CatalogNodesQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Near:       d.near,
	})

	n, qm, err := clients.Consul().Catalog().Nodes(opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	nodes := make([]*dep.Node, 0, len(n))
	for _, node := range n {
		nodes = append(nodes, &dep.Node{
			ID:              node.ID,
			Node:            node.Node,
			Address:         node.Address,
			Datacenter:      node.Datacenter,
			TaggedAddresses: node.TaggedAddresses,
			Meta:            node.Meta,
		})
	}

	// Sort unless the user explicitly asked for nearness
	if d.near == "" {
		sort.SliceStable(nodes,
			func(i, j int) bool {
				if nodes[i].Node == nodes[j].Node {
					return nodes[i].Address < nodes[j].Address
				}
				return nodes[i].Node < nodes[j].Node
			})
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return nodes, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogNodesQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *CatalogNodesQuery) ID() string {
	name := ""
	if d.dc != "" {
		name = name + "@" + d.dc
	}
	if d.near != "" {
		name = name + "~" + d.near
	}

	if name == "" {
		return "catalog.nodes"
	}
	return fmt.Sprintf("catalog.nodes(%s)", name)
}

// Stringer interface reuses ID
func (d *CatalogNodesQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *CatalogNodesQuery) Stop() {
	close(d.stopCh)
}

func (d *CatalogNodesQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
