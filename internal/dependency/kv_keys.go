package dependency

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*KVKeysQuery)(nil)

	// KVKeysQueryRe is the regular expression to use.
	KVKeysQueryRe = regexp.MustCompile(`\A` + prefixRe + dcRe + `\z`)
)

// KVKeysQuery queries the KV store for a single key.
type KVKeysQuery struct {
	isConsul
	stopCh chan struct{}

	dc     string
	prefix string
	opts   QueryOptions
}

// NewKVKeysQuery parses a string into a dependency.
func NewKVKeysQuery(s string) (*KVKeysQuery, error) {
	if s != "" && !KVKeysQueryRe.MatchString(s) {
		return nil, fmt.Errorf("kv.keys: invalid format: %q", s)
	}

	m := regexpMatch(KVKeysQueryRe, s)
	return &KVKeysQuery{
		stopCh: make(chan struct{}, 1),
		dc:     m["dc"],
		prefix: m["prefix"],
	}, nil
}

// Fetch queries the Consul API defined by the given client.
func (d *KVKeysQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
	})

	list, qm, err := clients.Consul().KV().Keys(d.prefix, "", opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	keys := make([]string, len(list))
	for i, v := range list {
		v = strings.TrimPrefix(v, d.prefix)
		v = strings.TrimLeft(v, "/")
		keys[i] = v
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return keys, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *KVKeysQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *KVKeysQuery) ID() string {
	prefix := d.prefix
	if d.dc != "" {
		prefix = prefix + "@" + d.dc
	}
	return fmt.Sprintf("kv.keys(%s)", prefix)
}

// Stringer interface reuses ID
func (d *KVKeysQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *KVKeysQuery) Stop() {
	close(d.stopCh)
}

func (d *KVKeysQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
