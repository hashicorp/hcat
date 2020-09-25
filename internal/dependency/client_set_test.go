package dependency

import (
	"testing"

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
	// good
	var err error
	client := testClients.Consul()
	if err = hasLeader(client); err != nil {
		t.Fatal("unexpected hasLeader error:", err)
	}
	// bad
	cconf := capi.DefaultConfig()
	cconf.Address = "bad.host:8500"
	client, err = capi.NewClient(cconf)
	if err != nil {
		t.Fatal("client create error:", err)
	}
	if err = hasLeader(client); err == nil {
		t.Fatal("hasLeader should have returned an error")
	}
}
