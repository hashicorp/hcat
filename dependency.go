package hcat

import dep "github.com/hashicorp/hcat/internal/dependency"

// We want to move dep.Dependency to be a public part of the API, as it is used
// in several public methods/interfaces, but it currently lives and is used in
// the internal module.

type Dependency dep.Dependency
// used to help shoehorn the dependency setup into hashicat
// until I get a chance to rework it
type queryOptionsSetter dep.QueryOptionsSetter
type queryOptions = dep.QueryOptions
type blockingQuery = dep.BlockingQuery
