// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dependency

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency  = (*KVGetQuery)(nil)
	_ BlockingQuery = (*KVGetQuery)(nil)

	// KVGetQueryRe is the regular expression to use.
	KVGetQueryRe = regexp.MustCompile(`\A` + keyRe + dcRe + `\z`)
)

// KVGetQuery queries the KV store for a single key.
type KVGetQuery struct {
	KVExistsQuery
	isBlocking
}

// NewKVGetQueryV1 processes options in the format of "key key=value"
// e.g. "my/key dc=dc1"
func NewKVGetQueryV1(key string, opts []string) (*KVGetQuery, error) {
	if key == "" || key == "/" {
		return nil, fmt.Errorf("kv.get: key required")
	}

	q, err := NewKVExistsQueryV1(key, opts)
	if err != nil {
		return nil, err
	}
	return &KVGetQuery{KVExistsQuery: *q}, nil
}

// NewKVGetQuery parses a string into a (non-blocking) KV lookup.
func NewKVGetQuery(s string) (*KVGetQuery, error) {
	if !KVGetQueryRe.MatchString(s) {
		return nil, fmt.Errorf("kv.get: invalid format: %q", s)
	}

	q, err := NewKVExistsQuery(s)
	if err != nil {
		return nil, err
	}
	return &KVGetQuery{KVExistsQuery: *q}, nil
}

// Fetch queries the Consul API defined by the given client.
func (d *KVGetQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
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
		return nil, rm, nil
	}

	value := dep.KvValue(pair.Value)
	return value, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *KVGetQuery) CanShare() bool {
	return true
}

// ID returns the human-friendly version of this dependency.
func (d *KVGetQuery) ID() string {
	key := d.key
	if d.dc != "" {
		key = key + "@" + d.dc
	}

	return fmt.Sprintf("kv.get(%s)", key)
}

// Stringer interface reuses ID
func (d *KVGetQuery) String() string {
	return d.ID()
}

// Stop halts the dependency's fetch function.
func (d *KVGetQuery) Stop() {
	close(d.stopCh)
}

func (d *KVGetQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
