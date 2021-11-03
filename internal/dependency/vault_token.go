package dependency

import (
	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency = (*VaultTokenQuery)(nil)
)

// VaultTokenQuery is the dependency to Vault for a secret
type VaultTokenQuery struct {
	isVault
	stopCh      chan struct{}
	secret      *dep.Secret
	vaultSecret *api.Secret
}

// NewVaultTokenQuery creates a new dependency.
func NewVaultTokenQuery(token string) (*VaultTokenQuery, error) {
	vaultSecret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   token,
			Renewable:     true,
			LeaseDuration: 1,
		},
	}
	return &VaultTokenQuery{
		stopCh:      make(chan struct{}, 1),
		vaultSecret: vaultSecret,
		secret:      transformSecret(vaultSecret, 0),
	}, nil
}

// Fetch queries the Vault API
func (d *VaultTokenQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	if vaultSecretRenewable(d.secret) {
		err := renewSecret(clients, d)
		if err != nil {
			return nil, nil, errors.Wrap(err, d.ID())
		}
	}

	return nil, nil, ErrLeaseExpired
}

func (d *VaultTokenQuery) stopChan() chan struct{} {
	return d.stopCh
}

func (d *VaultTokenQuery) secrets() (*dep.Secret, *api.Secret) {
	return d.secret, d.vaultSecret
}

// CanShare returns if this dependency is shareable.
func (d *VaultTokenQuery) CanShare() bool {
	return false
}

// Stop halts the dependency's fetch function.
func (d *VaultTokenQuery) Stop() {
	close(d.stopCh)
}

// ID returns the human-friendly version of this dependency.
func (d *VaultTokenQuery) ID() string {
	return "vault.token"
}

// Stringer interface reuses ID
func (d *VaultTokenQuery) String() string {
	return d.ID()
}

func (d *VaultTokenQuery) SetOptions(opts QueryOptions) {}
