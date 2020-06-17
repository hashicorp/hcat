package hat

import dep "github.com/hashicorp/hat/internal/dependency"

// We want to move dep.Dependency to be a public part of the API, as it is used
// in several public methods/interfaces, but it currently lives and is used in
// the internal module.

type Dependency dep.Dependency
