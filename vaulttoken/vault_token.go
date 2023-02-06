// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vaulttoken

import (
	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

// Ensure implements
var _ dep.Dependency = (*VaultTokenQuery)(nil)

// VaultTokenQuery is the dependency to Vault for a secret
type VaultTokenQuery struct {
	stopCh chan struct{}
	secret *api.Secret
}

// NewVaultTokenQuery creates a new dependency.
func NewVaultTokenQuery(token string) (*VaultTokenQuery, error) {
	secret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   token,
			Renewable:     true,
			LeaseDuration: 1,
		},
	}
	return &VaultTokenQuery{
		stopCh: make(chan struct{}, 1),
		secret: secret,
	}, nil
}

// Fetch queries the Vault API
func (d *VaultTokenQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, dep.ErrStopped
	default:
	}

	vaultSecretRenewable := d.secret.Renewable
	if d.secret.Auth != nil {
		vaultSecretRenewable = d.secret.Auth.Renewable
	}

	// ??? event/log if this runs and vaultSecretRenewable is false

	if vaultSecretRenewable {
		err := d.renewSecret(clients)
		if err != nil {
			return nil, nil, errors.Wrap(err, d.ID())
		}
	}

	return nil, nil, dep.ErrLeaseExpired
}

func (d *VaultTokenQuery) renewSecret(clients dep.Clients) error {
	renewer, err := clients.Vault().NewRenewer(&api.RenewerInput{
		Secret: d.secret,
	})
	if err != nil {
		return err
	}
	go renewer.Renew()
	defer renewer.Stop()

	for {
		select {
		case err := <-renewer.DoneCh():
			return err
		case renewal := <-renewer.RenewCh():
			d.secret = renewal.Secret
		case <-d.stopCh:
			return dep.ErrStopped
		}
	}
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
