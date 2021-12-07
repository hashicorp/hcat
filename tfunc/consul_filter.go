package tfunc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

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
