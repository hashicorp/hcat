package hcat

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
func datacentersFunc(r Recaller, used, missing *DepSet) func(ignore ...bool) ([]string, error) {
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

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.([]string), nil
		}

		missing.Add(d)

		return result, nil
	}
}

// envFunc returns a function which checks the value of an environment variable.
// Invokers can specify their own environment, which takes precedences over any
// real environment variables
func envFunc(env []string) func(string) (string, error) {
	return func(s string) (string, error) {
		for _, e := range env {
			split := strings.SplitN(e, "=", 2)
			k, v := split[0], split[1]
			if k == s {
				return v, nil
			}
		}
		return os.Getenv(s), nil
	}
}

// fileFunc returns or accumulates file dependencies.
func fileFunc(r Recaller, used, missing *DepSet, sandboxPath string) func(string) (string, error) {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}
		err := pathInSandbox(sandboxPath, s)
		if err != nil {
			return "", err
		}
		d, err := idep.NewFileQuery(s)
		if err != nil {
			return "", err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}

		missing.Add(d)

		return "", nil
	}
}

// keyFunc returns or accumulates key dependencies.
func keyFunc(r Recaller, used, missing *DepSet) func(string) (string, error) {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return "", err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}

		missing.Add(d)

		return "", nil
	}
}

// keyExistsFunc returns true if a key exists, false otherwise.
func keyExistsFunc(r Recaller, used, missing *DepSet) func(string) (bool, error) {
	return func(s string) (bool, error) {
		if len(s) == 0 {
			return false, nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return false, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value != nil, nil
		}

		missing.Add(d)

		return false, nil
	}
}

// keyWithDefaultFunc returns or accumulates key dependencies that have a
// default value.
func keyWithDefaultFunc(r Recaller, used, missing *DepSet) func(string, string) (string, error) {
	return func(s, def string) (string, error) {
		if len(s) == 0 {
			return def, nil
		}

		d, err := idep.NewKVGetQuery(s)
		if err != nil {
			return "", err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			if value == nil || value.(string) == "" {
				return def, nil
			}
			return value.(string), nil
		}

		missing.Add(d)

		return def, nil
	}
}

func safeLsFunc(r Recaller, used, missing *DepSet) func(string) ([]*dep.KeyPair, error) {
	// call lsFunc but explicitly mark that empty data set returned on monitored KV prefix is NOT safe
	return lsFunc(r, used, missing, false)
}

// lsFunc returns or accumulates keyPrefix dependencies.
func lsFunc(r Recaller, used, missing *DepSet, emptyIsSafe bool) func(string) ([]*dep.KeyPair, error) {
	return func(s string) ([]*dep.KeyPair, error) {
		result := []*dep.KeyPair{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := idep.NewKVListQuery(s)
		if err != nil {
			return result, err
		}

		used.Add(d)

		// Only return non-empty top-level keys
		if value, ok := r.Recall(d.String()); ok {
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
		missing.Add(d)

		return result, nil
	}
}

// nodeFunc returns or accumulates catalog node dependency.
func nodeFunc(r Recaller, used, missing *DepSet) func(...string) (*dep.CatalogNode, error) {
	return func(s ...string) (*dep.CatalogNode, error) {

		d, err := idep.NewCatalogNodeQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.(*dep.CatalogNode), nil
		}

		missing.Add(d)

		return nil, nil
	}
}

// nodesFunc returns or accumulates catalog node dependencies.
func nodesFunc(r Recaller, used, missing *DepSet) func(...string) ([]*dep.Node, error) {
	return func(s ...string) ([]*dep.Node, error) {
		result := []*dep.Node{}

		d, err := idep.NewCatalogNodesQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.([]*dep.Node), nil
		}

		missing.Add(d)

		return result, nil
	}
}

// secretFunc returns or accumulates secret dependencies from Vault.
func secretFunc(r Recaller, used, missing *DepSet) func(...string) (*dep.Secret, error) {
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

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			result = value.(*dep.Secret)
			return result, nil
		}

		missing.Add(d)

		return result, nil
	}
}

// secretsFunc returns or accumulates a list of secret dependencies from Vault.
func secretsFunc(r Recaller, used, missing *DepSet) func(string) ([]string, error) {
	return func(s string) ([]string, error) {
		var result []string

		if len(s) == 0 {
			return result, nil
		}

		d, err := idep.NewVaultListQuery(s)
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			result = value.([]string)
			return result, nil
		}

		missing.Add(d)

		return result, nil
	}
}

// byMeta returns Services grouped by one or many ServiceMeta fields.
func byMeta(meta string, services []*dep.HealthService) (groups map[string][]*dep.HealthService, err error) {
	re := regexp.MustCompile("[^a-zA-Z0-9_-]")
	normalize := func(x string) string {
		return re.ReplaceAllString(x, "_")
	}
	getOrDefault := func(m map[string]string, key string) string {
		realKey := strings.TrimSuffix(key, "|int")
		if val := m[realKey]; val != "" {
			return val
		}
		if strings.HasSuffix(key, "|int") {
			return "0"
		}
		return fmt.Sprintf("_no_%s_", realKey)
	}

	metas := strings.Split(meta, ",")

	groups = make(map[string][]*dep.HealthService)

	for _, s := range services {
		sm := s.ServiceMeta
		keyParts := []string{}
		for _, meta := range metas {
			value := getOrDefault(sm, meta)
			if strings.HasSuffix(meta, "|int") {
				value = getOrDefault(sm, meta)
				i, err := strconv.Atoi(value)
				if err != nil {
					return nil, errors.Wrap(err, fmt.Sprintf("cannot parse %v as number ", value))
				}
				value = fmt.Sprintf("%05d", i)
			}
			keyParts = append(keyParts, normalize(value))
		}
		key := strings.Join(keyParts, "_")
		groups[key] = append(groups[key], s)
	}

	return groups, nil
}

// serviceFunc returns or accumulates health service dependencies.
func serviceFunc(r Recaller, used, missing *DepSet) func(...string) ([]*dep.HealthService, error) {
	return func(s ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if len(s) == 0 || s[0] == "" {
			return result, nil
		}

		d, err := idep.NewHealthServiceQuery(strings.Join(s, "|"))
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.([]*dep.HealthService), nil
		}

		missing.Add(d)

		return result, nil
	}
}

// servicesFunc returns or accumulates catalog services dependencies.
func servicesFunc(r Recaller, used, missing *DepSet) func(...string) ([]*dep.CatalogSnippet, error) {
	return func(s ...string) ([]*dep.CatalogSnippet, error) {
		result := []*dep.CatalogSnippet{}

		d, err := idep.NewCatalogServicesQuery(strings.Join(s, ""))
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.([]*dep.CatalogSnippet), nil
		}

		missing.Add(d)

		return result, nil
	}
}

// connectFunc returns or accumulates health connect dependencies.
func connectFunc(r Recaller, used, missing *DepSet) func(...string) ([]*dep.HealthService, error) {
	return func(s ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if len(s) == 0 || s[0] == "" {
			return result, nil
		}

		d, err := idep.NewHealthConnectQuery(strings.Join(s, "|"))
		if err != nil {
			return nil, err
		}

		used.Add(d)

		if value, ok := r.Recall(d.String()); ok {
			return value.([]*dep.HealthService), nil
		}

		missing.Add(d)

		return result, nil
	}
}

func connectCARootsFunc(r Recaller, used, missing *DepSet,
) func(...string) ([]*api.CARoot, error) {
	return func(...string) ([]*api.CARoot, error) {
		d := idep.NewConnectCAQuery()
		used.Add(d)
		if value, ok := r.Recall(d.String()); ok {
			return value.([]*api.CARoot), nil
		}
		missing.Add(d)
		return nil, nil
	}
}

func connectLeafFunc(r Recaller, used, missing *DepSet,
) func(...string) (*api.LeafCert, error) {
	return func(s ...string) (*api.LeafCert, error) {
		if len(s) == 0 || s[0] == "" {
			return nil, nil
		}
		d := idep.NewConnectLeafQuery(s[0])
		used.Add(d)
		if value, ok := r.Recall(d.String()); ok {
			return value.(*api.LeafCert), nil
		}
		missing.Add(d)
		return nil, nil

	}
}

func safeTreeFunc(r Recaller, used, missing *DepSet) func(string) ([]*dep.KeyPair, error) {
	// call treeFunc but explicitly mark that empty data set returned on
	// monitored KV prefix is NOT safe
	return treeFunc(r, used, missing, false)
}

// treeFunc returns or accumulates keyPrefix dependencies.
func treeFunc(r Recaller, used, missing *DepSet, emptyIsSafe bool) func(string) ([]*dep.KeyPair, error) {
	return func(s string) ([]*dep.KeyPair, error) {
		result := []*dep.KeyPair{}

		if len(s) == 0 {
			return result, nil
		}

		d, err := idep.NewKVListQuery(s)
		if err != nil {
			return result, err
		}

		used.Add(d)

		// Only return non-empty top-level keys
		if value, ok := r.Recall(d.String()); ok {
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
		missing.Add(d)

		return result, nil
	}
}

// byKey accepts a slice of KV pairs and returns a map of the top-level
// key to all its subkeys. For example:
//
//		elasticsearch/a //=> "1"
//		elasticsearch/b //=> "2"
//		redis/a/b //=> "3"
//
// Passing the result from Consul through byTag would yield:
//
// 		map[string]map[string]string{
//	  	"elasticsearch": &dep.KeyPair{"a": "1"}, &dep.KeyPair{"b": "2"},
//			"redis": &dep.KeyPair{"a/b": "3"}
//		}
//
// Note that the top-most key is stripped from the Key value. Keys that have no
// prefix after stripping are removed from the list.
func byKey(pairs []*dep.KeyPair) (map[string]map[string]*dep.KeyPair, error) {
	m := make(map[string]map[string]*dep.KeyPair)
	for _, pair := range pairs {
		parts := strings.Split(pair.Key, "/")
		top := parts[0]
		key := strings.Join(parts[1:], "/")

		if key == "" {
			// Do not add a key if it has no prefix after stripping.
			continue
		}

		if _, ok := m[top]; !ok {
			m[top] = make(map[string]*dep.KeyPair)
		}

		newPair := *pair
		newPair.Key = key
		m[top][key] = &newPair
	}

	return m, nil
}

// byTag is a template func that takes the provided services and
// produces a map based on Service tags.
//
// The map key is a string representing the service tag. The map value is a
// slice of Services which have the tag assigned.
func byTag(in interface{}) (map[string][]interface{}, error) {
	m := make(map[string][]interface{})

	switch typed := in.(type) {
	case nil:
	case []*dep.CatalogSnippet:
		for _, s := range typed {
			for _, t := range s.Tags {
				m[t] = append(m[t], s)
			}
		}
	case []*idep.CatalogService:
		for _, s := range typed {
			for _, t := range s.ServiceTags {
				m[t] = append(m[t], s)
			}
		}
	case []*dep.HealthService:
		for _, s := range typed {
			for _, t := range s.Tags {
				m[t] = append(m[t], s)
			}
		}
	default:
		return nil, fmt.Errorf("byTag: wrong argument type %T", in)
	}

	return m, nil
}

// pathInSandbox returns an error if the provided path doesn't fall within the
// sandbox or if the file can't be evaluated (missing, invalid symlink, etc.)
func pathInSandbox(sandbox, path string) error {
	if sandbox != "" {
		s, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		s, err = filepath.Rel(sandbox, s)
		if err != nil {
			return err
		}
		if strings.HasPrefix(s, "..") {
			return fmt.Errorf("'%s' is outside of sandbox", path)
		}
	}
	return nil
}
