// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

// datacentersFunc returns or accumulates datacenter dependencies.
func datacentersFunc(recall hcat.Recaller) interface{} {
	return func(i ...bool) ([]string, error) {
		result := []string{}

		var ignore bool
		switch len(i) {
		case 0:
			ignore = false
		case 1:
			ignore = i[0]
		default:
			return result, fmt.Errorf("datacenters: wrong number of arguments, expected 0 or 1"+
				", but got %d", len(i))
		}

		d, err := idep.NewCatalogDatacentersQuery(ignore)
		if err != nil {
			return result, err
		}

		if value, ok := recall(d); ok {
			return value.([]string), nil
		}

		return result, nil
	}
}

// keyFunc returns or accumulates key dependencies.
func keyFunc(recall hcat.Recaller) interface{} {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return "", err
		}

		if value, ok := recall(d); ok {
			switch v := value.(type) {
			case nil:
				return "", nil
			case string:
				return v, nil
			case dep.KvValue:
				return string(v), nil
			}
		}

		return "", nil
	}
}

// keyExistsFunc returns true if a key exists, false otherwise.
func keyExistsFunc(recall hcat.Recaller) interface{} {
	return func(s string) (bool, error) {
		if len(s) == 0 {
			return false, nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return false, err
		}

		if value, ok := recall(d); ok {
			return value != nil, nil
		}

		return false, nil
	}
}

// keyWithDefaultFunc returns or accumulates key dependencies that have a
// default value.
func keyWithDefaultFunc(recall hcat.Recaller) interface{} {
	return func(s, def string) (string, error) {
		if len(s) == 0 {
			return def, nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return "", err
		}

		if value, ok := recall(d); ok {
			if value == nil || value.(string) == "" {
				return def, nil
			}
			return value.(string), nil
		}

		return def, nil
	}
}

// safeLsFunc returns the same output as `ls` but refuses to render the
// template if the query returns blank/empty data.
func safeLsFunc(recall hcat.Recaller) interface{} {
	// call lsFunc but explicitly mark that empty data set returned on
	// monitored KV prefix is NOT safe
	return lsFunc(false)(recall)
}

// lsFunc returns list of top level key-pairs at a given path.
func lsFunc(emptyIsSafe bool) func(hcat.Recaller) interface{} {
	return func(recall hcat.Recaller) interface{} {
		return func(s string) ([]*dep.KeyPair, error) {
			result := []*dep.KeyPair{}

			if len(s) == 0 {
				return result, nil
			}

			d, err := idep.NewKVListQuery(s)
			if err != nil {
				return result, err
			}

			// Only return non-empty top-level keys
			if value, ok := recall(d); ok {
				for _, pair := range value.([]*dep.KeyPair) {
					if pair.Key != "" && !strings.Contains(pair.Key, "/") {
						result = append(result, pair)
					}
				}

				if len(result) == 0 {
					if emptyIsSafe {
						// Operator used potentially unsafe ls function in the
						// template instead of the safeLs
						return result, nil
					}
				} else {
					// non empty result is good so we just return the data
					return result, nil
				}

				// If we reach this part of the code result is completely empty as
				// value returned has no KV pairs
				// Operator selected to use safeLs on the specific KV prefix so we
				// will refuse to render template by marking d as missing
			}

			// r.Recall either returned an error or safeLs entered unsafe case
			return result, nil
		}
	}
}

// nodeFunc returns or accumulates catalog node dependency.
func nodeFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) (interface{}, error) {

		d, err := idep.NewCatalogNodeQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.(*dep.CatalogNode), nil
		}

		return nil, nil
	}
}

// nodesFunc returns or accumulates catalog node dependencies.
func nodesFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) ([]*dep.Node, error) {
		result := []*dep.Node{}

		d, err := idep.NewCatalogNodesQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.Node), nil
		}

		return result, nil
	}
}

// serviceFunc returns or accumulates health service dependencies.
func serviceFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if len(s) == 0 || s[0] == "" {
			return result, nil
		}

		d, err := idep.NewHealthServiceQuery(strings.Join(s, "|"))
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.HealthService), nil
		}

		return result, nil
	}
}

// servicesFunc returns or accumulates catalog services dependencies.
func servicesFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) ([]*dep.CatalogSnippet, error) {
		result := []*dep.CatalogSnippet{}

		d, err := idep.NewCatalogServicesQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.CatalogSnippet), nil
		}

		return result, nil
	}
}

// connectFunc returns or accumulates health connect dependencies.
func connectFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if len(s) == 0 || s[0] == "" {
			return result, nil
		}

		d, err := idep.NewHealthConnectQuery(strings.Join(s, "|"))
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.HealthService), nil
		}

		return result, nil
	}
}

// connectCARootsFunc returns all connect trusted certificate authority (CA)
// root certificates.
func connectCARootsFunc(recall hcat.Recaller) interface{} {
	return func(...string) ([]*api.CARoot, error) {
		d := idep.NewConnectCAQuery()
		if value, ok := recall(d); ok {
			return value.([]*api.CARoot), nil
		}
		return nil, nil
	}
}

// connectLeafFunc returns leaf certificate representing a single service.
func connectLeafFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) (interface{}, error) {
		if len(s) == 0 || s[0] == "" {
			return nil, nil
		}
		d := idep.NewConnectLeafQuery(s[0])
		if value, ok := recall(d); ok {
			return value.(*api.LeafCert), nil
		}
		return nil, nil

	}
}

// safeTreeFunc returns the same results as `tree` but refuses to render the
// template if the query returns blank/empty data.
func safeTreeFunc(recall hcat.Recaller) interface{} {
	// call treeFunc but explicitly mark that empty data set returned on
	// monitored KV prefix is NOT safe
	return treeFunc(false)(recall)
}

// treeFunc returns *all* kv pairs at the given key path and all nested paths.
func treeFunc(emptyIsSafe bool) func(hcat.Recaller) interface{} {
	return func(recall hcat.Recaller) interface{} {
		return func(s string) ([]*dep.KeyPair, error) {
			result := []*dep.KeyPair{}

			if len(s) == 0 {
				return result, nil
			}

			d, err := idep.NewKVListQuery(s)
			if err != nil {
				return result, err
			}

			// Only return non-empty top-level keys
			if value, ok := recall(d); ok {
				for _, pair := range value.([]*dep.KeyPair) {
					parts := strings.Split(pair.Key, "/")
					if parts[len(parts)-1] != "" {
						result = append(result, pair)
					}
				}

				if len(result) == 0 {
					if emptyIsSafe {
						// Operator used potentially unsafe tree function in the
						// template instead of the safeTree
						return result, nil
					}
				} else {
					// non empty result is good so we just return the data
					return result, nil
				}

				// If we reach this part of the code result is completely empty as
				// value returned no KV pairs
				// Operator selected to use safeTree on the specific KV prefix so
				// we will refuse to render template by marking d as missing
			}

			// r.Recall either returned an error or safeTree entered unsafe case
			return result, nil
		}
	}
}
