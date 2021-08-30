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
	_ isDependency = (*KVExistsQuery)(nil)

	// KVExistsQueryRe is the regular expression to use.
	KVExistsQueryRe = regexp.MustCompile(`\A` + keyRe + dcRe + `\z`)
)

// KVExistsQuery uses a non-blocking query with the KV store for key lookup.
type KVExistsQuery struct {
	isConsul
	stopCh chan struct{}

	dc   string
	key  string
	ns   string
	opts QueryOptions
}

func (d *KVExistsQuery) SetOptions(opts QueryOptions) {
	opts.WaitIndex = 0
	opts.WaitTime = 0
	d.opts = opts
}

func (d *KVExistsQuery) String() string {
	key := d.key
	if d.dc != "" {
		key = key + "@" + d.dc
	}
	return fmt.Sprintf("kv.exists(%s)", key)
}

// NewKVExistsQueryV1 processes options in the format of "key key=value"
// e.g. "my/key dc=dc1"
func NewKVExistsQueryV1(key string, opts []string) (*KVExistsQuery, error) {
	if key == "" || key == "/" {
		return nil, fmt.Errorf("kv.exists: key required")
	}

	q := KVExistsQuery{
		stopCh: make(chan struct{}, 1),
		key:    strings.TrimPrefix(key, "/"),
	}
	for _, opt := range opts {
		if strings.TrimSpace(opt) == "" {
			continue
		}
		queryParam := strings.Split(opt, "=")
		if len(queryParam) != 2 {
			return nil, fmt.Errorf(
				"kv.exists: invalid query parameter format: %q", opt)
		}
		query := strings.TrimSpace(queryParam[0])
		value := strings.TrimSpace(queryParam[1])
		switch query {
		case "dc", "datacenter":
			q.dc = value
		case "ns", "namespace":
			q.ns = value
		default:
			return nil, fmt.Errorf(
				"kv.exists: invalid query parameter: %q", opt)
		}
	}

	return &q, nil
}

// NewKVExistsQuery parses a string into a KV lookup.
func NewKVExistsQuery(s string) (*KVExistsQuery, error) {
	if !KVExistsQueryRe.MatchString(s) {
		return nil, fmt.Errorf("kv.exists: invalid format: %q", s)
	}

	m := regexpMatch(KVExistsQueryRe, s)
	return &KVExistsQuery{
		stopCh: make(chan struct{}, 1),
		dc:     m["dc"],
		key:    m["key"],
		ns:     "",
	}, nil
}

// Fetch queries the Consul API defined by the given client.
func (d *KVExistsQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Namespace:  d.ns,
	})

	pair, qm, err := clients.Consul().KV().Get(d.key, opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	if pair == nil {
		return dep.KVExists(false), rm, nil
	}

	return dep.KVExists(true), rm, nil
}

// Stop halts the dependency's fetch function.
func (d *KVExistsQuery) Stop() {
	close(d.stopCh)
}

// CanShare returns a boolean if this dependency is shareable.
func (d *KVExistsQuery) CanShare() bool {
	return true
}
