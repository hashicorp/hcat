package vaulttoken

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/stretchr/testify/assert"
)

func TestVaultAgentTokenQuery_Fetch(t *testing.T) {
	// Don't use t.Parallel() here as the SetToken() calls are global and break
	// other tests if run in parallel

	// reset token back to original
	vc := testClients.Vault()
	defer vc.SetToken(vc.Token())

	// Set up the Vault token file.
	tokenFile, err := ioutil.TempFile("", "token1")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tokenFile.Name())
	testWrite(tokenFile.Name(), []byte("token"))

	d, err := NewVaultAgentTokenQuery(tokenFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	clientSet := testClients
	token, _, err := d.Fetch(clientSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "token", token)

	// Update the contents.
	testWrite(tokenFile.Name(), []byte("another_token"))
	token, _, err = d.Fetch(clientSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "another_token", token)
}

func TestVaultAgentTokenQuery_Fetch_missingFile(t *testing.T) {
	t.Parallel()

	// Use a non-existant token file path.
	d, err := NewVaultAgentTokenQuery("/tmp/invalid-file")
	if err != nil {
		t.Fatal(err)
	}

	clientSet := hcat.NewClientSet()
	clientSet.AddVault(hcat.VaultInput{
		Token: "foo",
	})
	_, _, err = d.Fetch(clientSet)
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatal(err)
	}

	// Token should be unaffected.
	assert.Equal(t, "foo", clientSet.Vault().Token())
}

func testWrite(path string, contents []byte) error {
	if path == "" {
		panic("missing path")
	}

	parent := filepath.Dir(path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}

	f, err := ioutil.TempFile(parent, "")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(contents); err != nil {
		return err
	}

	for _, err := range []error{
		f.Sync(),
		f.Close(),
		os.Chmod(f.Name(), 0o644),
		os.Rename(f.Name(), path),
	} {
		if err != nil {
			return err
		}
	}

	return nil
}
