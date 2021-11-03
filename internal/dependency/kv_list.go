package dependency

import (
	"encoding/gob"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*KVListQuery)(nil)

	// KVListQueryRe is the regular expression to use.
	KVListQueryRe = regexp.MustCompile(`\A` + prefixRe + dcRe + `\z`)
)

func init() {
	gob.Register([]*dep.KeyPair{})
}

// KVListQuery queries the KV store for a single key.
type KVListQuery struct {
	isConsul
	stopCh chan struct{}

	dc     string
	prefix string
	ns     string
	opts   QueryOptions
}

// NewKVListQuery processes options in the format of "prefix key=value"
// e.g. "key_prefix dc=dc1"
func NewKVListQueryV1(prefix string, opts []string) (*KVListQuery, error) {
	if prefix == "" || prefix == "/" {
		return nil, fmt.Errorf("kv.list: prefix required")
	}

	q := KVListQuery{
		stopCh: make(chan struct{}, 1),
		prefix: strings.TrimPrefix(prefix, "/"),
	}

	for _, opt := range opts {
		if strings.TrimSpace(opt) == "" {
			continue
		}

		queryParam := strings.Split(opt, "=")
		if len(queryParam) != 2 {
			return nil, fmt.Errorf(
				"kv.list: invalid query parameter format: %q", opt)
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
				"kv.list: invalid query parameter: %q", opt)
		}
	}

	return &q, nil
}

// NewKVListQuery parses a string into a dependency.
func NewKVListQuery(s string) (*KVListQuery, error) {
	if s != "" && !KVListQueryRe.MatchString(s) {
		return nil, fmt.Errorf("kv.list: invalid format: %q", s)
	}

	m := regexpMatch(KVListQueryRe, s)
	return &KVListQuery{
		stopCh: make(chan struct{}, 1),
		dc:     m["dc"],
		prefix: m["prefix"],
		ns:     "",
	}, nil
}

// Fetch queries the Consul API defined by the given client.
func (d *KVListQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Namespace:  d.ns,
	})

	list, qm, err := clients.Consul().KV().List(d.prefix, opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	pairs := make([]*dep.KeyPair, 0, len(list))
	for _, pair := range list {
		key := strings.TrimPrefix(pair.Key, d.prefix)
		key = strings.TrimLeft(key, "/")

		pairs = append(pairs, &dep.KeyPair{
			Path:        pair.Key,
			Key:         key,
			Value:       string(pair.Value),
			Exists:      true,
			CreateIndex: pair.CreateIndex,
			ModifyIndex: pair.ModifyIndex,
			LockIndex:   pair.LockIndex,
			Flags:       pair.Flags,
			Session:     pair.Session,
		})
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return pairs, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *KVListQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *KVListQuery) ID() string {
	prefix := d.prefix
	if d.dc != "" {
		prefix = prefix + "@" + d.dc
	}
	return fmt.Sprintf("kv.list(%s)", prefix)
}

// Stringer interface reuses ID
func (d *KVListQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *KVListQuery) Stop() {
	close(d.stopCh)
}

func (d *KVListQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
