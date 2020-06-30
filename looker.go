package hcat

import (
	"os"

	dep "github.com/hashicorp/hcat/internal/dependency"
)

// Interface for looking up data from Consul, Vault and the Environment.
type Looker interface {
	dep.Clients
	Env() []string
	Stop()
}

type ClientSetInput dep.CreateClientInput

// internal clientSet focuses only on external (consul/vault) dependencies
// at this point so we extend it here to include environment variables to meet
// the looker interface.
type clientSet struct {
	*dep.ClientSet
	injectedEnv []string
}

func NewClientSet(in ClientSetInput) *clientSet {
	din := dep.CreateClientInput(in)
	clients := dep.NewClientSet()
	clients.CreateConsulClient(&din)
	clients.CreateVaultClient(&din)
	return &clientSet{
		ClientSet:   clients,
		injectedEnv: []string{},
	}
}

func (cs *clientSet) Stop() {
	if cs.ClientSet != nil {
		cs.ClientSet.Stop()
	}
	cs.injectedEnv = []string{}
}

// InjectEnv adds "key=value" pairs to the environment used for template
// evaluations and child process runs. Note that this is in addition to the
// environment running consul template and in the case of duplicates, the
// last entry wins.
func (cs *clientSet) InjectEnv(env ...string) {
	cs.injectedEnv = append(cs.injectedEnv, env...)
}

// You should do any messaging of the Environment variables during startup
// As this will just use the raw Environment.
func (cs *clientSet) Env() []string {
	return append(os.Environ(), cs.injectedEnv...)
}
