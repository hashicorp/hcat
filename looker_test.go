package hcat

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClientSet(t *testing.T) {
	t.Run("client-api-init", func(t *testing.T) {
		ts := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `"test"`)
			}))
		ts.Listener, _ = net.Listen("tcp", "127.0.0.1:8500")
		ts.Start()
		defer ts.Close()
		// ^ fake consul
		cs := NewClientSet()
		err := cs.AddConsul(ConsulInput{})
		if err != nil {
			t.Fatal(err)
		}
		err = cs.AddVault(VaultInput{})
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Stop()
		if c := cs.Consul(); c == nil {
			t.Fatal("Consul Client failed to load.")
		}
		if v := cs.Vault(); v == nil {
			t.Fatal("Consul Client failed to load.")
		}
	})

	t.Run("env", func(t *testing.T) {
		cs := NewClientSet()
		defer cs.Stop()
		// All os environment variables should be present
		parentEnv := make(map[string]bool)
		for _, e := range os.Environ() {
			parentEnv[e] = true
		}
		for _, e := range cs.Env() {
			if !parentEnv[e] {
				t.Fatal("Missing parent environment variable")
			}
		}
		// Check inject
		cs.InjectEnv("foo=bar")
		foundit := false
		for _, e := range cs.Env() {
			if e == "foo=bar" {
				foundit = true
				break
			}
		}
		if !foundit {
			t.Fatal("Injecting environment variable failed")
		}
		// check that it still pulls in os environ
		os.Setenv("key", "value")
		for _, e := range cs.Env() {
			if e == "key=value" {
				foundit = true
				break
			}
		}
		if !foundit {
			t.Fatal("System environment variable failed")
		}
	})
}
