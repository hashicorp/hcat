package vaulttoken

import (
	"os"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

// Ensure implements
var _ dep.Dependency = (*VaultAgentTokenQuery)(nil)

const (
	// VaultAgentTokenSleepTime is the amount of time to sleep between queries, since
	// the fsnotify library is not compatible with solaris and other OSes yet.
	VaultAgentTokenSleepTime = 15 * time.Second
)

// VaultAgentTokenQuery is the dependency to Vault Agent token
type VaultAgentTokenQuery struct {
	stopCh chan struct{}
	stat   os.FileInfo
	path   string
}

// NewVaultAgentTokenQuery creates a new dependency.
func NewVaultAgentTokenQuery(path string) (*VaultAgentTokenQuery, error) {
	return &VaultAgentTokenQuery{
		stopCh: make(chan struct{}, 1),
		path:   path,
	}, nil
}

// Fetch retrieves this dependency and returns the result or any errors that
// occur in the process.
func (d *VaultAgentTokenQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	var token string
	select {
	case <-d.stopCh:
		return "", nil, dep.ErrStopped
	case r := <-d.watch(d.stat):

		if r.err != nil {
			return "", nil, errors.Wrap(r.err, d.ID())
		}

		raw_token, err := os.ReadFile(d.path)
		if err != nil {
			return "", nil, errors.Wrap(err, d.ID())
		}

		d.stat = r.stat
		token = string(raw_token)
	}

	return token, &dep.ResponseMetadata{
		LastIndex: uint64(time.Now().Unix()),
	}, nil
}

// ID returns the human-friendly version of this dependency.
func (d *VaultAgentTokenQuery) ID() string {
	return "vault-agent.token"
}

// Stop halts the dependency's fetch function.
func (d *VaultAgentTokenQuery) Stop() {
	close(d.stopCh)
}

// Stringer interface reuses ID
func (d *VaultAgentTokenQuery) String() string {
	return d.ID()
}

type watchResult struct {
	stat os.FileInfo
	err  error
}

// watch watches the file for changes
func (d *VaultAgentTokenQuery) watch(lastStat os.FileInfo) <-chan *watchResult {
	ch := make(chan *watchResult, 1)

	go func(lastStat os.FileInfo) {
		for {
			stat, err := os.Stat(d.path)
			if err != nil {
				select {
				case <-d.stopCh:
					return
				case ch <- &watchResult{err: err}:
					return
				}
			}

			changed := lastStat == nil ||
				lastStat.Size() != stat.Size() ||
				lastStat.ModTime() != stat.ModTime()

			if changed {
				select {
				case <-d.stopCh:
					return
				case ch <- &watchResult{stat: stat}:
					return
				}
			}

			time.Sleep(VaultAgentTokenSleepTime)
		}
	}(lastStat)

	return ch
}
