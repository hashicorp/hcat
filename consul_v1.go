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
		"service": v1ServiceFunc,
		"connect": v1ConnectFunc,

		// Set of Consul functions that are not yet implemented for v1. These
		// intentionally error instead of defaulting to the v0 implementations
		// to avoid introducing breaking changes when they are supported.
		"node":     v1TODOFunc,
		"nodes":    v1TODOFunc,
		"services": v1TODOFunc,
	}
}

// v1TODOFunc is a placeholder function to return an error instead of inheriting
// the default template functions.
func v1TODOFunc(recall Recaller) interface{} {
	return func(s ...string) (interface{}, error) {
		return nil, errFuncNotImplemented
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
