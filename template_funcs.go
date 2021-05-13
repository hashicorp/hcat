package hcat

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
	"github.com/pkg/errors"
)

// DenyFunc always returns an error, to be used in place of template functions
// that you want denied. For use with the FuncMapMerge.
func DenyFunc(...interface{}) (string, error) {
	return "", errors.New("function disabled")
}

// now is function that represents the current time in UTC. This is here
// primarily for the tests to override times.
var now = func() time.Time { return time.Now().UTC() }

// datacentersFunc returns or accumulates datacenter dependencies.
func datacentersFunc(recall Recaller) func(ignore ...bool) ([]string, error) {
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
func keyFunc(recall Recaller) func(string) (string, error) {
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
func keyExistsFunc(recall Recaller) func(string) (bool, error) {
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
func keyWithDefaultFunc(recall Recaller) func(string, string) (string, error) {
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

func safeLsFunc(recall Recaller) func(string) ([]*dep.KeyPair, error) {
	// call lsFunc but explicitly mark that empty data set returned on
	// monitored KV prefix is NOT safe
	return lsFunc(recall, false)
}

// lsFunc returns or accumulates keyPrefix dependencies.
func lsFunc(recall Recaller, emptyIsSafe bool) func(string) ([]*dep.KeyPair, error) {
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

// nodeFunc returns or accumulates catalog node dependency.
func nodeFunc(recall Recaller) func(...string) (*dep.CatalogNode, error) {
	return func(s ...string) (*dep.CatalogNode, error) {

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
func nodesFunc(recall Recaller) func(...string) ([]*dep.Node, error) {
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

// secretFunc returns or accumulates secret dependencies from Vault.
func secretFunc(recall Recaller) func(...string) (*dep.Secret, error) {
	return func(s ...string) (*dep.Secret, error) {
		var result *dep.Secret

		if len(s) == 0 {
			return result, nil
		}

		// TODO: Refactor into separate template functions
		path, rest := s[0], s[1:]
		data := make(map[string]interface{})
		for _, str := range rest {
			parts := strings.SplitN(str, "=", 2)
			if len(parts) != 2 {
				return result, fmt.Errorf("not k=v pair %q", str)
			}

			k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			data[k] = v
		}

		var d dep.Dependency
		var err error

		if len(rest) == 0 {
			d, err = idep.NewVaultReadQuery(path)
		} else {
			d, err = idep.NewVaultWriteQuery(path, data)
		}

		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			result = value.(*dep.Secret)
			return result, nil
		}

		return result, nil
	}
}

// secretsFunc returns or accumulates a list of secret dependencies from Vault.
func secretsFunc(recall Recaller) func(string) ([]string, error) {
	return func(s string) ([]string, error) {
		var result []string

		if len(s) == 0 {
			return result, nil
		}

		d, err := idep.NewVaultListQuery(s)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			result = value.([]string)
			return result, nil
		}

		return result, nil
	}
}

// serviceFunc returns or accumulates health service dependencies.
func serviceFunc(recall Recaller) func(...string) ([]*dep.HealthService, error) {
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
func servicesFunc(recall Recaller) func(...string) ([]*dep.CatalogSnippet, error) {
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
func connectFunc(recall Recaller) func(...string) ([]*dep.HealthService, error) {
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

func connectCARootsFunc(recall Recaller) func(...string) ([]*api.CARoot, error) {
	return func(...string) ([]*api.CARoot, error) {
		d := idep.NewConnectCAQuery()
		if value, ok := recall(d); ok {
			return value.([]*api.CARoot), nil
		}
		return nil, nil
	}
}

func connectLeafFunc(recall Recaller) func(...string) (*api.LeafCert, error) {
	return func(s ...string) (*api.LeafCert, error) {
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

func safeTreeFunc(recall Recaller) func(string) ([]*dep.KeyPair, error) {
	// call treeFunc but explicitly mark that empty data set returned on
	// monitored KV prefix is NOT safe
	return treeFunc(recall, false)
}

// treeFunc returns or accumulates keyPrefix dependencies.
func treeFunc(recall Recaller, emptyIsSafe bool) func(string) ([]*dep.KeyPair, error) {
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
