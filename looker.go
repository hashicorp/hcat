package hat

import (
	"os"

	dep "github.com/hashicorp/hat/internal/dependency"
)

// Interface for looking up data from Consul, Vault and the Environment.
type Looker interface {
	dep.Clients
	Env() []string
	Stop()
}

// internal clientSet focuses only on external (consul/vault) dependencies
// at this point so we extend it here to include environment variables to meet
// the looker interface.
type clientSet struct {
	dep.ClientSet
}

// You should do any messaging of the Environment variables during startup
// As this will just use the raw Environment.
func (cs *clientSet) Env() []string {
	return os.Environ()
}
