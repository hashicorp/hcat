package tfunc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
)

// explode is used to expand a list of keypairs into a deeply-nested hash.
func explode(pairs []*dep.KeyPair) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, pair := range pairs {
		if err := explodeHelper(m, pair.Key, pair.Value, pair.Key); err != nil {
			return nil, errors.Wrap(err, "explode")
		}
	}
	return m, nil
}

// explodeHelper is a recursive helper for explode and explodeMap
func explodeHelper(m map[string]interface{}, k string, v interface{}, p string) error {
	if strings.Contains(k, "/") {
		parts := strings.Split(k, "/")
		top := parts[0]
		key := strings.Join(parts[1:], "/")

		if _, ok := m[top]; !ok {
			m[top] = make(map[string]interface{})
		}
		nest, ok := m[top].(map[string]interface{})
		if !ok {
			return fmt.Errorf("not a map: %q: %q already has value %q", p, top, m[top])
		}
		return explodeHelper(nest, key, v, k)
	}

	if k != "" {
		m[k] = v
	}

	return nil
}

// explodeMap turns a single-level map into a deeply-nested hash.
func explodeMap(mapIn map[string]interface{}) (map[string]interface{}, error) {
	mapOut := make(map[string]interface{})

	var keys []string
	for k := range mapIn {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i := range keys {
		if err := explodeHelper(mapOut, keys[i], mapIn[keys[i]], keys[i]); err != nil {
			return nil, errors.Wrap(err, "explodeMap")
		}
	}
	return mapOut, nil
}

type _map = map[string]interface{}

// mergeMap is used to merge two maps
func mergeMap(dstMap _map, srcMap _map, args ...func(*mergo.Config)) (_map, error) {
	if err := mergo.Map(&dstMap, srcMap, args...); err != nil {
		return nil, err
	}
	return dstMap, nil
}

// mergeMapWithOverride is used to merge two maps with dstMap overriding vaules in srcMap
func mergeMapWithOverride(dstMap _map, srcMap _map) (_map, error) {
	return mergeMap(dstMap, srcMap, mergo.WithOverride)
}
