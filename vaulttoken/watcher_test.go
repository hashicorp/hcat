// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vaulttoken

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/hcat/events"
	"github.com/hashicorp/vault/api"
)

//type VaultTokenConfig struct {
//	Token, VaultAgentTokenFile string
//	UnwrapToken, RenewToken    bool
//	RetryFunc                  hcat.RetryFunc
//	clients                    *hcat.ClientSet
//	doneCh                     chan struct{}
//}

// approle auto-auth setup in watch_test.go, TestMain()
func TestVaultTokenWatcher(t *testing.T) {
	// Don't set the below to run in parallel. They mess with the single
	// running vault's permissions.
	t.Run("noop", func(t *testing.T) {
		conf := VaultTokenConfig{}
		watcher, err := VaultTokenWatcher(conf)
		if err != nil {
			t.Error(err)
		}
		if watcher != nil {
			t.Error("watcher should be nil")
		}
	})

	t.Run("fixed_token", func(t *testing.T) {
		testClients.Vault().SetToken(vaultToken)
		conf := VaultTokenConfig{Clients: testClients}
		conf.Token = vaultToken
		watcher, err := VaultTokenWatcher(conf)
		if err != nil {
			t.Error(err)
		}
		if watcher != nil {
			t.Error("watcher should be nil")
		}
		if testClients.Vault().Token() != vaultToken {
			t.Error("Token should be " + vaultToken)
		}
	})

	t.Run("secretwrapped_token", func(t *testing.T) {
		testClients.Vault().SetToken("not a token")
		defer testClients.Vault().SetToken(vaultToken)
		conf := VaultTokenConfig{Clients: testClients}
		data, err := json.Marshal(&api.SecretWrapInfo{Token: vaultToken})
		if err != nil {
			t.Error(err)
		}
		conf.Token = string(data)
		_, err = VaultTokenWatcher(conf)
		if err != nil {
			t.Error(err)
		}
		if testClients.Vault().Token() != vaultToken {
			t.Error("Token should be " + vaultToken)
		}
	})

	t.Run("tokenfile", func(t *testing.T) {
		// setup
		testClients.Vault().SetToken(vaultToken)
		tokenfile := runVaultAgent(testClients, tokenRoleId)
		defer func() {
			testClients.Vault().SetToken(vaultToken)
			os.Remove(tokenfile)
		}()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		/// XXX refactor out so I can use this below in TestVaultTokenRefreshToken
		waitForUpdate := make(chan events.Trace)
		tokenUpdateEvent := func(e events.Event) {
			switch e := e.(type) {
			case events.Trace:
				// fmt.Printf("%#v\n", e) // in case of deadlock, uncomment
				if e.Message == "tokenfile token updated" {
					waitForUpdate <- e
				}
			}
		}
		/// XXX
		conf := VaultTokenConfig{
			Clients:      testClients,
			Context:      ctx,
			EventHandler: tokenUpdateEvent,
		}
		conf.Token = vaultToken
		conf.AgentTokenFile = tokenfile

		// test data
		watcher, err := VaultTokenWatcher(conf)
		if err != nil {
			t.Error(err)
		}
		defer watcher.Stop()

		// tests
		<-waitForUpdate

		if testClients.Vault().Token() == vaultToken {
			t.Error("Token should not be " + vaultToken)
		}
	})

	t.Run("renew", func(t *testing.T) {
		// exercise the renewer: the action is all inside the vault api
		// calls and vault so there's little to check.. so we just try
		// to call it and make sure it doesn't error
		testClients.Vault().SetToken(vaultToken)
		renew := true
		_, err := testClients.Vault().Auth().Token().Create(
			&api.TokenCreateRequest{
				ID:        "b_token",
				TTL:       "1m",
				Renewable: &renew,
			})
		if err != nil {
			t.Error(err)
		}
		conf := VaultTokenConfig{Clients: testClients}
		conf.Token = "b_token"
		conf.Renew = renew
		watcher, err := VaultTokenWatcher(conf)
		if err != nil {
			t.Error(err)
		}
		defer watcher.Stop()

		select {
		case err := <-watcher.WaitCh(context.Background()):
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Millisecond * 100):
			// give it a chance to throw an error
		}
	})
}

func TestVaultTokenRefreshToken(t *testing.T) {
	zero := 0
	waitForUpdate := make(chan events.Trace)
	tokenUpdateEvent := func(e events.Event) {
		switch e := e.(type) {
		case events.Trace:
			// fmt.Printf("%#v\n", e) // in case of deadlock, uncomment
			if e.Message == "tokenfile token updated" {
				waitForUpdate <- e
			}
		}
	}
	watcher := &vaultTokenWatcher{
		Watcher: *hcat.NewWatcher(hcat.WatcherInput{
			Clients: testClients,
			// force watcher to be synchronous so we can control test flow
			DataBufferSize: &zero,
		}), event: tokenUpdateEvent,
	}
	wrapinfo := api.SecretWrapInfo{
		Token: "btoken",
	}
	b, _ := json.Marshal(wrapinfo)
	type testcase struct {
		name, raw_token, exp_token string
	}
	vault := testClients.Vault()
	testcases := []testcase{
		{name: "noop", raw_token: "foo", exp_token: "foo"},
		{name: "spaces", raw_token: " foo ", exp_token: "foo"},
		{name: "secretwrap", raw_token: string(b), exp_token: "btoken"},
	}
	for i, tc := range testcases {
		tc := tc // avoid for-loop pointer wart
		name := fmt.Sprintf("%d_%s", i, tc.name)
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			conf := VaultTokenConfig{
				Clients: testClients, Token: "non-blank", Context: ctx,
			}
			watchLoop, err := watchTokenFile(watcher, conf)
			if err != nil {
				t.Error(err)
			}
			go func() {
				watchLoop()
			}()
			tokenUpdate <- tc.raw_token
			// tests
			<-waitForUpdate

			if vault.Token() != tc.exp_token {
				t.Errorf("bad token, expected: '%s', received '%s'",
					tc.exp_token, vault.Token())
			}
		})
	}
	watcher.Stop()
}

// When vault-agent uses the wrap_ttl option it writes a json blob instead of
// a raw token. This verifies it will extract the token from that when needed.
// It doesn't test unwrap. The integration test covers that for now.
func TestVaultTokenUnpackToken(t *testing.T) {
	t.Run("table_test", func(t *testing.T) {
		wrapinfo := api.SecretWrapInfo{
			Token: "btoken",
		}
		b, _ := json.Marshal(wrapinfo)
		testcases := []struct{ in, out string }{
			{in: "", out: ""},
			{in: "atoken", out: "atoken"},
			{in: string(b), out: "btoken"},
		}
		for _, tc := range testcases {
			dummy := &setTokenFaker{}
			token, _ := unpackToken(dummy, tc.in, false)
			if token != tc.out {
				t.Errorf("unpackToken, wanted: '%v', got: '%v'", tc.out, token)
			}
		}
	})
	t.Run("unwrap_test", func(t *testing.T) {
		vault := testClients.Vault()
		vault.SetToken(vaultToken)
		vault.SetWrappingLookupFunc(func(operation, path string) string {
			if path == "auth/token/create" {
				return "30s"
			}
			return ""
		})
		defer vault.SetWrappingLookupFunc(nil)

		secret, err := vault.Auth().Token().Create(&api.TokenCreateRequest{
			Lease: "1h",
		})
		if err != nil {
			t.Fatal(err)
		}

		unwrap := true
		wrappedToken := secret.WrapInfo.Token
		token, err := unpackToken(vault, wrappedToken, unwrap)
		if err != nil {
			t.Fatal(err)
		}
		if token == wrappedToken {
			t.Errorf("tokens should not match")
		}
	})
}

type setTokenFaker struct {
	Token string
}

func (t *setTokenFaker) SetToken(token string) {}
func (t *setTokenFaker) Logical() *api.Logical { return nil }

func fakedDepNotify(name string) fakeDep {
	return fakeDep{name: name, data: make(chan string)}
}

type fakeDep struct {
	name string
	data chan string
}

func (d fakeDep) Send(s string) {
	d.data <- s
}

// notifier interface (+ID)
var _ hcat.Notifier = (*fakeDep)(nil)

func (d fakeDep) Notify(_ any) bool {
	return true
}

// dependency interface
var _ dep.Dependency = (*fakeDep)(nil)

func (d fakeDep) Fetch(dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	s := <-d.data
	return s, nil, nil
}
func (d fakeDep) ID() string     { return d.name }
func (d fakeDep) String() string { return d.ID() }
func (d fakeDep) Stop() {
	close(d.data)
}
