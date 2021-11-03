package dependency

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*VaultWriteQuery)(nil)
)

// VaultWriteQuery is the dependency to Vault for a secret
type VaultWriteQuery struct {
	isVault
	stopCh  chan struct{}
	sleepCh chan time.Duration

	path     string
	data     map[string]interface{}
	dataHash string
	secret   *dep.Secret
	opts     QueryOptions

	// vaultSecret is the actual Vault secret which we are renewing
	vaultSecret *api.Secret
}

// NewVaultWriteQuery creates a new datacenter dependency.
func NewVaultWriteQuery(s string, d map[string]interface{}) (*VaultWriteQuery, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil, fmt.Errorf("vault.write: invalid format: %q", s)
	}

	return &VaultWriteQuery{
		stopCh:   make(chan struct{}, 1),
		sleepCh:  make(chan time.Duration, 1),
		path:     s,
		data:     d,
		dataHash: sha1Map(d),
	}, nil
}

// Fetch queries the Vault API
func (d *VaultWriteQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}
	select {
	case dur := <-d.sleepCh:
		time.Sleep(dur)
	default:
	}

	firstRun := d.secret == nil

	if !firstRun && vaultSecretRenewable(d.secret) {
		err := renewSecret(clients, d)
		if err != nil {
			return nil, nil, errors.Wrap(err, d.ID())
		}
	}

	opts := d.opts.Merge(&QueryOptions{})
	vaultSecret, err := d.writeSecret(clients, opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	// vaultSecret == nil when writing to KVv1 engines
	if vaultSecret == nil {
		return respWithMetadata(d.secret)
	}

	d.vaultSecret = vaultSecret
	// cloned secret which will be exposed to the template
	d.secret = transformSecret(vaultSecret, opts.DefaultLease)

	if !vaultSecretRenewable(d.secret) {
		dur := leaseCheckWait(d.secret)
		d.sleepCh <- dur
	}

	return respWithMetadata(d.secret)
}

// meet renewer interface
func (d *VaultWriteQuery) stopChan() chan struct{} {
	return d.stopCh
}

func (d *VaultWriteQuery) secrets() (*dep.Secret, *api.Secret) {
	return d.secret, d.vaultSecret
}

// CanShare returns if this dependency is shareable.
func (d *VaultWriteQuery) CanShare() bool {
	return false
}

// Stop halts the given dependency's fetch.
func (d *VaultWriteQuery) Stop() {
	close(d.stopCh)
}

// ID returns the human-friendly version of this dependency.
func (d *VaultWriteQuery) ID() string {
	return fmt.Sprintf("vault.write(%s -> %s)", d.path, d.dataHash)
}

// Stringer interface reuses ID
func (d *VaultWriteQuery) String() string {
	return d.ID()
}

// sha1Map returns the sha1 hash of the data in the map. The reason this data is
// hashed is because it appears in the output and could contain sensitive
// information.
func sha1Map(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha1.New()
	for _, k := range keys {
		io.WriteString(h, fmt.Sprintf("%s=%q", k, m[k]))
	}

	return fmt.Sprintf("%.4x", h.Sum(nil))
}

func (d *VaultWriteQuery) writeSecret(clients dep.Clients, opts *QueryOptions) (*api.Secret, error) {
	data := d.data

	_, isv2, _ := isKVv2(clients.Vault(), d.path)
	if isv2 {
		data = map[string]interface{}{"data": d.data}
	}

	vaultSecret, err := clients.Vault().Logical().Write(d.path, data)
	if err != nil {
		return nil, errors.Wrap(err, d.ID())
	}
	// vaultSecret is always nil when KVv1 engine (isv2==false)
	if isv2 && vaultSecret == nil {
		return nil, fmt.Errorf("no secret exists at %s", d.path)
	}

	return vaultSecret, nil
}

func (d *VaultWriteQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
