package hcat

import (
	"fmt"
	"text/template"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

var errFuncNotImplemented = fmt.Errorf("function is not implemented")

// FuncMapConsulV1 is a set of template functions for querying Consul endpoints.
// The functions support Consul v1 API filter expressions and Consul enterprise
// namespaces.
func FuncMapConsulV1() template.FuncMap {
	return template.FuncMap{
		"service":      v1ServiceFunc,
		"connect":      v1ConnectFunc,
		"services":     v1ServicesFunc,
		"keys":         v1KVListFunc,
		"key":          v1KVGetFunc,
		"keyExists":    v1KVExistsFunc,
		"keyExistsGet": v1KVExistsGetFunc,

		// Set of Consul functions that are not yet implemented for v1. These
		// intentionally error instead of defaulting to the v0 implementations
		// to avoid introducing breaking changes when they are supported.
		"node":  v1TODOFunc,
		"nodes": v1TODOFunc,
	}
}

// v1TODOFunc is a placeholder function to return an error instead of inheriting
// the default template functions.
func v1TODOFunc(recall Recaller) interface{} {
	return func(s ...string) (interface{}, error) {
		return nil, errFuncNotImplemented
	}
}

// v1ServicesFunc returns information on registered Consul services
//
// Endpoint: /v1/catalog/services
// Template: {{ services <filter options> ... }}
func v1ServicesFunc(recall Recaller) interface{} {
	return func(opts ...string) ([]*dep.CatalogSnippet, error) {
		result := []*dep.CatalogSnippet{}

		d, err := idep.NewCatalogServicesQueryV1(opts)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.CatalogSnippet), nil
		}

		return result, nil
	}
}

// v1ServiceFunc returns or accumulates health information of Consul services.
//
// Endpoint: /v1/health/service/:service
// Template: {{ service "serviceName" <filter options> ... }}
func v1ServiceFunc(recall Recaller) interface{} {
	return func(service string, opts ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if service == "" {
			return result, nil
		}

		d, err := idep.NewHealthServiceQueryV1(service, opts)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.HealthService), nil
		}

		return result, nil
	}
}

// v1ConnectFunc returns or accumulates health information of datacenter nodes
// and its services that are in Consul Connect, the the service mesh collection
// of Consul.
//
// Endpoint: /v1/health/connect/:service
// Template: {{ connect "serviceName" <filter options> ... }}
func v1ConnectFunc(recall Recaller) interface{} {
	return func(service string, opts ...string) ([]*dep.HealthService, error) {
		result := []*dep.HealthService{}

		if service == "" {
			return result, nil
		}

		d, err := idep.NewHealthConnectQueryV1(service, opts)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.HealthService), nil
		}

		return result, nil
	}
}

// v1KVListFunc returns list of key value pairs
//
// Endpoint: /v1/kv/:prefix?recurse
// Template: {{ keys "prefix" <filter options> ... }}
func v1KVListFunc(recall Recaller) interface{} {
	return func(prefix string, opts ...string) ([]*dep.KeyPair, error) {
		result := []*dep.KeyPair{}

		if prefix == "" {
			return result, nil
		}

		d, err := idep.NewKVListQueryV1(prefix, opts)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.([]*dep.KeyPair), nil
		}

		return result, nil
	}
}

// v1KVGetFunc returns a single key value if it exists
//
// Endpoint: /v1/kv/:key
// Template: {{ key "key" <filter options> ... }}
func v1KVGetFunc(recall Recaller) interface{} {
	return func(key string, opts ...string) (dep.KvValue, error) {
		var result dep.KvValue

		if key == "" {
			return result, nil
		}

		d, err := idep.NewKVGetQueryV1(key, opts)
		if err != nil {
			return "", err
		}

		if value, ok := recall(d); ok {
			return value.(dep.KvValue), nil
		}

		return result, nil
	}
}

// v1KVExistsFunc returns if a key value exists
//
// Endpoint: /v1/kv/:key
// Template: {{ keyExists "key" <filter options> ... }}
func v1KVExistsFunc(recall Recaller) interface{} {
	return func(key string, opts ...string) (dep.KVExists, error) {
		var result dep.KVExists

		if key == "" {
			return result, nil
		}

		d, err := idep.NewKVExistsQueryV1(key, opts)
		if err != nil {
			return dep.KVExists(false), err
		}

		if value, ok := recall(d); ok {
			return value.(dep.KVExists), nil
		}

		return result, nil
	}
}

// v1KVExistsGetFunc checks if a key exists and
// if the key exists, returns the key-value pair
//
// Endpoint: /v1/kv/:key
// Template: {{ keyExistsGet "key" <filter options> ... }}
func v1KVExistsGetFunc(recall Recaller) interface{} {
	return func(key string, opts ...string) (*dep.KeyPair, error) {
		var result *dep.KeyPair

		if key == "" {
			return result, nil
		}

		d, err := idep.NewKVExistsGetQueryV1(key, opts)
		if err != nil {
			return result, err
		}

		if value, ok := recall(d); ok {
			return value.(*dep.KeyPair), nil
		}

		return result, nil
	}
}
