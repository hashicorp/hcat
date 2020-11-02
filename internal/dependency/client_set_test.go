package dependency

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	vapi "github.com/hashicorp/vault/api"
)

func TestClientSet_unwrapVaultToken(t *testing.T) {
	// Don't use t.Parallel() here as the SetWrappingLookupFunc is a global
	// setting and breaks other tests if run in parallel

	vault := testClients.Vault()

	// Create a wrapped token
	vault.SetWrappingLookupFunc(func(operation, path string) string {
		return "30s"
	})
	defer vault.SetWrappingLookupFunc(nil)

	wrappedToken, err := vault.Auth().Token().Create(&vapi.TokenCreateRequest{
		Lease: "1h",
	})
	if err != nil {
		t.Fatal(err)
	}

	token := vault.Token()

	if token == wrappedToken.WrapInfo.Token {
		t.Errorf("expected %q to not be %q", token,
			wrappedToken.WrapInfo.Token)
	}

	if _, err := vault.Auth().Token().LookupSelf(); err != nil {
		t.Fatal(err)
	}
}

func TestClientSet_hasLeader(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var err error
		client := testClients.Consul()
		if err = hasLeader(client, time.Minute); err != nil {
			t.Fatal("unexpected hasLeader error:", err)
		}
	})

	t.Run("non temp error", func(t *testing.T) {
		t.Parallel()

		cconf := capi.DefaultConfig()
		cconf.Address = "bad.host:8500"
		client, err := capi.NewClient(cconf)
		if err != nil {
			t.Fatal("client create error:", err)
		}
		if err = hasLeader(client, time.Minute); err == nil {
			t.Fatal("hasLeader should have returned an error")
		}
	})

	t.Run("retry exceeds", func(t *testing.T) {
		t.Parallel()

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Force timeout by setting client transport timeout to a shorter duration
			// than the delayed response
			time.Sleep(20 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("leader.address:8500"))
		}))
		defer testServer.Close()

		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = 10 * time.Millisecond
		cconf := capi.Config{
			Address:    testServer.URL,
			HttpClient: testServer.Client(),
			Transport:  transport,
		}
		client, err := capi.NewClient(&cconf)
		if err != nil {
			t.Fatal("client create error:", err)
		}

		startTime := time.Now()
		if err = hasLeader(client, 3*time.Second); err == nil {
			t.Fatal("hasLeader should have returned an error")
		}

		// Test retry logic reaches the maxRetryWait
		// retries once and exists before the next retry with delay 4s
		elapsed := time.Now().Sub(startTime)
		expected := 2 * time.Second
		if elapsed < expected {
			t.Fatal("hasLeader should have exceeded retry duration but returned early", elapsed, expected)
		}
	})
}
