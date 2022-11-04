package vaulttoken

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/events"
	"github.com/hashicorp/vault/api"
)

type VaultTokenConfig struct {
	Clients      *hcat.ClientSet
	Context      context.Context
	RetryFunc    hcat.RetryFunc
	EventHandler events.EventHandler

	Token, AgentTokenFile string
	Unwrap, Renew         bool
}
type vaultTokenWatcher struct {
	hcat.Watcher
	event events.EventHandler
}

// VaultTokenWatcher monitors the vault token for updates
func VaultTokenWatcher(c VaultTokenConfig) (*vaultTokenWatcher, error) {
	raw_token := strings.TrimSpace(c.Token)
	if raw_token == "" {
		return nil, nil
	}

	unwrap := c.Unwrap
	vault := c.Clients.Vault()
	// get/set token once when kicked off, async after that..
	token, err := unpackToken(vault, raw_token, unwrap)
	if err != nil {
		return nil, fmt.Errorf("vaultwatcher: %w", err)
	}
	vault.SetToken(token)

	var once sync.Once
	var watcher *vaultTokenWatcher
	two := 2
	getWatcher := func() *vaultTokenWatcher {
		once.Do(func() {
			watcher = &vaultTokenWatcher{
				Watcher: *hcat.NewWatcher(hcat.WatcherInput{
					Clients:        c.Clients,
					VaultRetryFunc: c.RetryFunc,
					EventHandler:   c.EventHandler,
					DataBufferSize: &two,
				}), event: c.EventHandler,
			}
		})
		return watcher
	}

	// Vault Agent Token File process //
	tokenFile := strings.TrimSpace(c.AgentTokenFile)
	if tokenFile != "" {
		w := getWatcher()
		watchLoop, err := watchTokenFile(w, c)
		if err != nil {
			return nil, fmt.Errorf("vaultwatcher: %w", err)
		}
		go watchLoop()
	}

	// Vault Token Renewal process //
	renewVault := vault.Token() != "" && c.Renew
	if renewVault {
		w := getWatcher()
		vt, err := NewVaultTokenQuery(token)
		n := callbackNotifier{dep: vt}
		if err != nil {
			w.Stop() // need to stop token file loop. use context.
			return nil, fmt.Errorf("vaultwatcher: %w", err)
		}
		w.Track(n, vt)
	}

	return watcher, nil
}

// split out channel into module level variable to enable testing.
// not 100% this as a good way to do this but it works for now.
// small buffer to facilitate testing
var tokenUpdate = make(chan string, 1)

// kicks off using the watcher to monitor the agent token file for updates
// handles updating the client with new tokens as they are written to the file
func watchTokenFile(w *vaultTokenWatcher, c VaultTokenConfig) (func(), error) {
	atf, err := NewVaultAgentTokenQuery(c.AgentTokenFile)

	pipeit := func(d any) bool {
		s, ok := d.(string)
		if ok {
			tokenUpdate <- s
		}
		return ok
	}

	n := callbackNotifier{dep: atf, fun: pipeit}
	if err != nil {
		return nil, fmt.Errorf("vaultwatcher: %w", err)
	}
	w.Track(n, atf)
	w.Poll(atf)

	vault := c.Clients.Vault()
	raw_token := c.Token
	return func() {
		for {
			select {
			case err := <-w.WaitCh(c.Context):
				if err != nil {
					w.event(events.Trace{
						ID:      w.ID(),
						Message: "non-fatal token watcher error: " + err.Error(),
					})
				}
			case new_raw_token := <-tokenUpdate:
				if new_raw_token == raw_token {
					continue
				}
				token, err := unpackToken(vault, new_raw_token, c.Unwrap)
				switch err {
				case nil:
					raw_token = new_raw_token
					vault.SetToken(token)
					w.event(events.Trace{ // used with testing
						ID:      w.ID(),
						Message: "tokenfile token updated",
					})
				default:
					w.event(events.Trace{
						ID:      w.ID(),
						Message: "non-fatal token watcher error: " + err.Error(),
					})
				}
			case <-c.Context.Done():
				return
			}
		}
	}, nil
}

type vaultClient interface {
	SetToken(string)
	Logical() *api.Logical
}

// unpackToken grabs the real token from raw_token (unwrap, etc)
func unpackToken(client vaultClient, token string, unwrap bool) (string, error) {
	// If vault agent specifies wrap_ttl for the token it is returned as
	// a SecretWrapInfo struct marshalled into JSON instead of the normal raw
	// token. This checks for that and pulls out the token if it is the case.
	var wrapinfo api.SecretWrapInfo
	if err := json.Unmarshal([]byte(token), &wrapinfo); err == nil {
		token = wrapinfo.Token
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("empty token")
	}

	if unwrap {
		client.SetToken(token) // needs to be set to unwrap
		secret, err := client.Logical().Unwrap(token)
		switch {
		case err != nil:
			return token, fmt.Errorf("vault unwrap: %s", err)
		case secret == nil:
			return token, fmt.Errorf("vault unwrap: no secret")
		case secret.Auth == nil:
			return token, fmt.Errorf("vault unwrap: no secret auth")
		case secret.Auth.ClientToken == "":
			return token, fmt.Errorf("vault unwrap: no token returned")
		default:
			token = secret.Auth.ClientToken
		}
	}
	return token, nil
}
