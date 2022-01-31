package hcat

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientSet(t *testing.T) {
	t.Run("client-api-init", func(t *testing.T) {
		ts := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `"test"`)
			}))

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		err = listener.Close()
		require.NoError(t, err)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ts.Listener, err = net.Listen("tcp", addr)
		require.NoError(t, err)

		ts.Start()
		defer ts.Close()
		// ^ fake consul
		cs := NewClientSet()
		err = cs.AddConsul(ConsulInput{
			Address: addr,
		})
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
			t.Fatal("Vault Client failed to load.")
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
