package dependency

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*KVExistsGetQuery)(nil)
)

// KVExistsGetQuery uses a non-blocking query to lookup a single key in the KV store.
// The query returns whether the key exists and the value of the key if it exists.
type KVExistsGetQuery struct {
	BlockingQuery
	KVExistsQuery
}

// NewKVExistsGetQueryV1 processes options in the format of "key key=value"
// e.g. "my/key dc=dc1"
func NewKVExistsGetQueryV1(key string, opts []string) (*KVExistsGetQuery, error) {
	if key == "" || key == "/" {
		return nil, fmt.Errorf("kv.exists.get: key required")
	}

	q, err := NewKVExistsQueryV1(key, opts)
	if err != nil {
		return nil, err
	}
	return &KVExistsGetQuery{KVExistsQuery: *q}, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *KVExistsGetQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *KVExistsGetQuery) ID() string {
	key := d.key
	var opts []string
	if d.dc != "" {
		opts = append(opts, "dc="+d.dc)
	}
	if d.ns != "" {
		opts = append(opts, "ns="+d.ns)
	}
	if len(opts) > 0 {
		key = fmt.Sprintf("%s?%s", key, strings.Join(opts, "&"))
	}
	return fmt.Sprintf("kv.exists.get(%s)", key)
}

// Stringer interface reuses ID
func (d *KVExistsGetQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *KVExistsGetQuery) Stop() {
	close(d.stopCh)
}

func (d *KVExistsGetQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}

// Fetch queries the Consul API defined by the given client.
func (d *KVExistsGetQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
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
		return nil, nil, errors.Wrap(err, d.ID())
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	if pair == nil {
		return &dep.KeyPair{
			Path:   d.key,
			Key:    d.key,
			Exists: false,
		}, rm, nil
	}

	return &dep.KeyPair{
		Path:        pair.Key,
		Key:         pair.Key,
		Value:       string(pair.Value),
		Exists:      true,
		CreateIndex: pair.CreateIndex,
		ModifyIndex: pair.ModifyIndex,
		LockIndex:   pair.LockIndex,
		Flags:       pair.Flags,
		Session:     pair.Session,
	}, rm, nil
}
