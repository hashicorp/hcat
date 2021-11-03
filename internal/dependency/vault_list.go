package dependency

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*VaultListQuery)(nil)
)

// VaultListQuery is the dependency to Vault for a secret
type VaultListQuery struct {
	isVault
	stopCh chan struct{}

	path string
	opts QueryOptions
}

// NewVaultListQuery creates a new datacenter dependency.
func NewVaultListQuery(s string) (*VaultListQuery, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil, fmt.Errorf("vault.list: invalid format: %q", s)
	}

	return &VaultListQuery{
		stopCh: make(chan struct{}, 1),
		path:   s,
	}, nil
}

// Fetch queries the Vault API
func (d *VaultListQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{})

	// If this is not the first query, poll to simulate blocking-queries.
	if opts.WaitIndex != 0 {
		dur := VaultDefaultLeaseDuration
		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(dur):
		}
	}

	// If we got this far, we either didn't have a secret to renew, the secret was
	// not renewable, or the renewal failed, so attempt a fresh list.
	secret, err := clients.Vault().Logical().List(d.path)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	var result []string

	// The secret could be nil if it does not exist.
	if secret == nil || secret.Data == nil {
		return respWithMetadata(result)
	}

	// This is a weird thing that happened once...
	keys, ok := secret.Data["keys"]
	if !ok {
		return respWithMetadata(result)
	}

	list, ok := keys.([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("%s: unexpected response", d)
	}

	for _, v := range list {
		typed, ok := v.(string)
		if !ok {
			return nil, nil, fmt.Errorf("%s: non-string in list", d)
		}
		result = append(result, typed)
	}
	sort.Strings(result)

	return respWithMetadata(result)
}

// CanShare returns if this dependency is shareable.
func (d *VaultListQuery) CanShare() bool {
	return false
}

// Stop halts the given dependency's fetch.
func (d *VaultListQuery) Stop() {
	close(d.stopCh)
}

// ID returns the human-friendly version of this dependency.
func (d *VaultListQuery) ID() string {
	return fmt.Sprintf("vault.list(%s)", d.path)
}

// Stringer interface reuses ID
func (d *VaultListQuery) String() string {
	return d.ID()
}

func (d *VaultListQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
