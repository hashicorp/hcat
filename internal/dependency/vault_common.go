package dependency

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/vault/api"
)

type renewer interface {
	dep.Dependency
	stopChan() chan struct{}
	secrets() (*dep.Secret, *api.Secret)
}

func renewSecret(clients dep.Clients, d renewer) error {
	secret, vaultSecret := d.secrets()
	renewer, err := clients.Vault().NewRenewer(&api.RenewerInput{
		Secret: vaultSecret,
	})
	if err != nil {
		return err
	}
	go renewer.Renew()
	defer renewer.Stop()

	for {
		select {
		case <-renewer.DoneCh():
			return nil
		case renewal := <-renewer.RenewCh():
			updateSecret(secret, renewal.Secret)
		case <-d.stopChan():
			return ErrStopped
		}
	}
}

// (Test) Options for leaseCheckWait
type LCWopts struct {
	nowF      func() time.Time
	jitterOFF bool
}

// default to live/real values
func (to *LCWopts) getOpts() (func() time.Time, bool) {
	switch {
	case to == nil: // defaults
		return time.Now, true
	case to.nowF != nil:
		return to.nowF, !to.jitterOFF
	}
	return time.Now, !to.jitterOFF
}

// leaseCheckWait accepts a secret and returns the recommended amount of
// time to sleep.
func leaseCheckWait(s *dep.Secret, topts *LCWopts) time.Duration {
	nowFunc, jitter := topts.getOpts()
	// base should be set to the default already
	// be sure not to set base to <=0 below
	base := s.LeaseDuration
	// Handle whether this is an auth or a regular secret.
	if s.Auth != nil && s.Auth.LeaseDuration > 0 {
		base = s.Auth.LeaseDuration
	}

	// Handle if this is a certificate with no lease
	if _, ok := s.Data["certificate"]; ok && s.LeaseID == "" {
		if expInterface, ok := s.Data["expiration"]; ok {
			if expData, err := expInterface.(json.Number).Int64(); err == nil {
				base = int(expData - nowFunc().Unix())
			}
		}
	}

	// Handle if this is an AppRole secret_id with no lease
	if _, ok := s.Data["secret_id"]; ok && s.LeaseID == "" {
		if ttlInterface, ok := s.Data["secret_id_ttl"]; ok {
			ttlData, err := ttlInterface.(json.Number).Int64()
			if err == nil && ttlData > 0 {
				base = int(ttlData) + 1
				log.Printf("[DEBUG] Found approle secret_id and non-zero secret_id_ttl, setting lease duration to %d seconds", base)
			}
		}
	}

	// Handle if this is a secret with a rotation period.  If this is a
	// rotating secret, the rotating secret's TTL will be the duration to sleep
	// before rendering the new secret.
	var rotatingSecret bool
	if _, ok := s.Data["rotation_period"]; ok && s.LeaseID == "" {
		if ttlInterface, ok := s.Data["ttl"]; ok {
			ttlData, err := ttlInterface.(json.Number).Int64()
			if err == nil && ttlData > 0 {
				// Add a second for cushion
				base = int(ttlData) + 1
				rotatingSecret = true
			}
		}
	}

	// Convert to float seconds.
	sleep := float64(time.Duration(base) * time.Second)

	switch {
	case vaultSecretRenewable(s):
		// Renew at 1/3 the remaining lease. This will give us an opportunity
		// to retry at least one more time should the first renewal fail.
		sleep = sleep / 3.0
		// Use some randomness so many clients do not hit Vault simultaneously.
		sleep = sleep * (rand.Float64() + 1) / 2.0
	case !rotatingSecret && jitter:
		// If the secret doesn't have a rotation period, this is a
		// non-renewable leased secret.
		// For non-renewable leases set the renew duration to use much of the
		// secret lease as possible. Use a stagger over 85%-95% of the lease
		// duration so that many clients do not hit Vault simultaneously.
		sleep = sleep * (.85 + rand.Float64()*0.1)
	}

	return time.Duration(sleep)
}

// vaultSecretRenewable determines if the given secret is renewable.
func vaultSecretRenewable(s *dep.Secret) bool {
	if s.Auth != nil {
		return s.Auth.Renewable
	}
	return s.Renewable
}

// transformSecret transforms an api secret into our secret. This does not deep
// copy underlying deep data structures, so it's not safe to modify the vault
// secret as that may modify the data in the transformed secret.
func transformSecret(theirs *api.Secret, defaultLease time.Duration) *dep.Secret {
	ours := &dep.Secret{LeaseDuration: int(defaultLease.Seconds())}
	updateSecret(ours, theirs)
	return ours
}

// updateSecret updates our secret with the new data from the api, careful to
// not overwrite missing data. Renewals don't include the original secret, and
// we don't want to delete that data accidentally.
func updateSecret(ours *dep.Secret, theirs *api.Secret) {
	if theirs.RequestID != "" {
		ours.RequestID = theirs.RequestID
	}

	if theirs.LeaseID != "" {
		ours.LeaseID = theirs.LeaseID
	}

	if theirs.LeaseDuration != 0 {
		ours.LeaseDuration = theirs.LeaseDuration
	}

	if theirs.Renewable {
		ours.Renewable = theirs.Renewable
	}

	if len(theirs.Data) != 0 {
		ours.Data = theirs.Data
	}

	if len(theirs.Warnings) != 0 {
		ours.Warnings = theirs.Warnings
	}

	if theirs.Auth != nil {
		if ours.Auth == nil {
			ours.Auth = &dep.SecretAuth{}
		}

		if theirs.Auth.ClientToken != "" {
			ours.Auth.ClientToken = theirs.Auth.ClientToken
		}

		if theirs.Auth.Accessor != "" {
			ours.Auth.Accessor = theirs.Auth.Accessor
		}

		if len(theirs.Auth.Policies) != 0 {
			ours.Auth.Policies = theirs.Auth.Policies
		}

		if len(theirs.Auth.Metadata) != 0 {
			ours.Auth.Metadata = theirs.Auth.Metadata
		}

		if theirs.Auth.LeaseDuration != 0 {
			ours.Auth.LeaseDuration = theirs.Auth.LeaseDuration
		}

		if theirs.Auth.Renewable {
			ours.Auth.Renewable = theirs.Auth.Renewable
		}
	}

	if theirs.WrapInfo != nil {
		if ours.WrapInfo == nil {
			ours.WrapInfo = &dep.SecretWrapInfo{}
		}

		if theirs.WrapInfo.Token != "" {
			ours.WrapInfo.Token = theirs.WrapInfo.Token
		}

		if theirs.WrapInfo.TTL != 0 {
			ours.WrapInfo.TTL = theirs.WrapInfo.TTL
		}

		if !theirs.WrapInfo.CreationTime.IsZero() {
			ours.WrapInfo.CreationTime = theirs.WrapInfo.CreationTime
		}

		if theirs.WrapInfo.WrappedAccessor != "" {
			ours.WrapInfo.WrappedAccessor = theirs.WrapInfo.WrappedAccessor
		}
	}
}

func isKVv2(client *api.Client, path string) (string, bool, error) {
	// We don't want to use a wrapping call here so save any custom value and
	// restore after
	currentWrappingLookupFunc := client.CurrentWrappingLookupFunc()
	client.SetWrappingLookupFunc(nil)
	defer client.SetWrappingLookupFunc(currentWrappingLookupFunc)
	currentOutputCurlString := client.OutputCurlString()
	client.SetOutputCurlString(false)
	defer client.SetOutputCurlString(currentOutputCurlString)

	r := client.NewRequest("GET", "/v1/sys/internal/ui/mounts/"+path)
	resp, err := client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		// If we get a 404 we are using an older version of vault, default to
		// version 1
		if resp != nil && resp.StatusCode == 404 {
			return "", false, nil
		}

		// anonymous requests may fail to access /sys/internal/ui path
		// Vault v1.1.3 returns 500 status code but may return 4XX in future
		if client.Token() == "" {
			return "", false, nil
		}

		return "", false, err
	}

	secret, err := api.ParseSecret(resp.Body)
	if err != nil {
		return "", false, err
	}
	if secret == nil {
		return "", false, fmt.Errorf("secret at path %s does not exist", path)
	}
	var mountPath string
	if mountPathRaw, ok := secret.Data["path"]; ok {
		mountPath = mountPathRaw.(string)
	}
	var mountType string
	if mountTypeRaw, ok := secret.Data["type"]; ok {
		mountType = mountTypeRaw.(string)
	}
	options := secret.Data["options"]
	if options == nil {
		return mountPath, false, nil
	}
	versionRaw := options.(map[string]interface{})["version"]
	if versionRaw == nil {
		return mountPath, false, nil
	}
	version := versionRaw.(string)
	switch version {
	case "", "1":
		return mountPath, false, nil
	case "2":
		return mountPath, mountType == "kv", nil
	}

	return mountPath, false, nil
}

// shimKVv2Path aligns the supported legacy path to KV v2 specs by inserting
// /data/ into the path for reading secrets. Paths for metadata are not modified.
func shimKVv2Path(rawPath, mountPath string) string {
	switch {
	case rawPath == mountPath, rawPath == strings.TrimSuffix(mountPath, "/"):
		return path.Join(mountPath, "data")
	default:
		p := strings.TrimPrefix(rawPath, mountPath)

		// Only add /data/ prefix to the path if neither /data/ or /metadata/ are
		// present.
		if strings.HasPrefix(p, "data/") || strings.HasPrefix(p, "metadata/") {
			return rawPath
		}
		return path.Join(mountPath, "data", p)
	}
}
