package hcat

import (
	"fmt"
	"text/template"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

var errFuncNotImplemented = fmt.Errorf("function is not implemented")

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

func v1TODOFunc(recall Recaller) interface{} {
	return func(s ...string) (interface{}, error) {
		return nil, errFuncNotImplemented
	}
}

// v1ServiceFunc returns or accumulates health service dependencies.
// /v1/health/service/:service
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

// v1ConnectFunc returns or accumulates health connect dependencies.
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
