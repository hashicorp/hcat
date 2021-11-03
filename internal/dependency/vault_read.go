package dependency

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*VaultReadQuery)(nil)
)

// VaultReadQuery is the dependency to Vault for a secret
type VaultReadQuery struct {
	isVault
	stopCh  chan struct{}
	sleepCh chan time.Duration

	rawPath     string
	queryValues url.Values
	secret      *dep.Secret
	isKVv2      *bool
	secretPath  string
	opts        QueryOptions

	// vaultSecret is the actual Vault secret which we are renewing
	vaultSecret *api.Secret
}

// NewVaultReadQuery creates a new datacenter dependency.
func NewVaultReadQuery(s string) (*VaultReadQuery, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil, fmt.Errorf("vault.read: invalid format: %q", s)
	}

	secretURL, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	return &VaultReadQuery{
		stopCh:      make(chan struct{}, 1),
		sleepCh:     make(chan time.Duration, 1),
		rawPath:     secretURL.Path,
		queryValues: secretURL.Query(),
	}, nil
}

// Fetch queries the Vault API
func (d *VaultReadQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
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

	err := d.fetchSecret(clients)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	if !vaultSecretRenewable(d.secret) {
		dur := leaseCheckWait(d.secret)
		d.sleepCh <- dur
	}

	return respWithMetadata(d.secret)
}

func (d *VaultReadQuery) fetchSecret(clients dep.Clients) error {
	opts := d.opts.Merge(&QueryOptions{})
	vaultSecret, err := d.readSecret(clients, opts)
	if err == nil {
		d.vaultSecret = vaultSecret
		// the cloned secret which will be exposed to the template
		d.secret = transformSecret(vaultSecret, opts.DefaultLease)
	}
	return err
}

func (d *VaultReadQuery) stopChan() chan struct{} {
	return d.stopCh
}

func (d *VaultReadQuery) secrets() (*dep.Secret, *api.Secret) {
	return d.secret, d.vaultSecret
}

// CanShare returns if this dependency is shareable.
func (d *VaultReadQuery) CanShare() bool {
	return false
}

// Stop halts the given dependency's fetch.
func (d *VaultReadQuery) Stop() {
	close(d.stopCh)
}

// ID returns the human-friendly version of this dependency.
func (d *VaultReadQuery) ID() string {
	if v := d.queryValues["version"]; len(v) > 0 {
		return fmt.Sprintf("vault.read(%s.v%s)", d.rawPath, v[0])
	}
	return fmt.Sprintf("vault.read(%s)", d.rawPath)
}

// Stringer interface reuses ID
func (d *VaultReadQuery) String() string {
	return d.ID()
}

func (d *VaultReadQuery) readSecret(clients dep.Clients, opts *QueryOptions) (*api.Secret, error) {
	vaultClient := clients.Vault()

	// Check whether this secret refers to a KV v2 entry if we haven't yet.
	if d.isKVv2 == nil {
		mountPath, isKVv2, err := isKVv2(vaultClient, d.rawPath)
		if err != nil {
			isKVv2 = false
			d.secretPath = d.rawPath
		} else if isKVv2 {
			d.secretPath = addPrefixToVKVPath(d.rawPath, mountPath, "data")
		} else {
			d.secretPath = d.rawPath
		}
		d.isKVv2 = &isKVv2
	}

	vaultSecret, err := vaultClient.Logical().ReadWithData(d.secretPath,
		d.queryValues)

	if err != nil {
		return nil, errors.Wrap(err, d.ID())
	}
	if vaultSecret == nil || deletedKVv2(vaultSecret) {
		return nil, fmt.Errorf("no secret exists at %s", d.secretPath)
	}
	return vaultSecret, nil
}

func (d *VaultReadQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}

func deletedKVv2(s *api.Secret) bool {
	switch md := s.Data["metadata"].(type) {
	case map[string]interface{}:
		return md["deletion_time"] != ""
	}
	return false
}
