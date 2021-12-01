package tfunc

import (
	"os"
	"text/template"
)

// All available template functions
func AllUnversioned() template.FuncMap {
	all := make(template.FuncMap)
	allfuncs := []func() template.FuncMap{
		ConsulFilters, Env, Control, Helpers, Math}
	for _, f := range allfuncs {
		for k, v := range f() {
			all[k] = v
		}
	}
	return all
}

// Consul querying functions
func ConsulV0() template.FuncMap {
	return template.FuncMap{
		"datacenters":  datacentersFunc,
		"key":          keyFunc,
		"keyExists":    keyExistsFunc,
		"keyOrDefault": keyWithDefaultFunc,
		"ls":           lsFunc(true),
		"safeLs":       safeLsFunc,
		"node":         nodeFunc,
		"nodes":        nodesFunc,
		"service":      serviceFunc,
		"connect":      connectFunc,
		"services":     servicesFunc,
		"tree":         treeFunc(true),
		"safeTree":     safeTreeFunc,
		"caRoots":      connectCARootsFunc,
		"caLeaf":       connectLeafFunc,
	}
}

// ConsulV1 is a set of template functions for querying Consul endpoints.
// The functions support Consul v1 API filter expressions and Consul enterprise
// namespaces.
func ConsulV1() template.FuncMap {
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

// Functions to filter consul results
func ConsulFilters() template.FuncMap {
	return template.FuncMap{
		"byKey":  byKey,
		"byTag":  byTag,
		"byMeta": byMeta,
	}
}

// Vault querying functions
func VaultV0() template.FuncMap {
	return template.FuncMap{
		"secret":  secretFunc,
		"secrets": secretsFunc,
	}
}

// Environment querying functions
func Env() template.FuncMap {
	return template.FuncMap{
		"env":          envFunc(os.Environ()),
		"envOrDefault": envOrDefaultFunc(os.Environ()),
	}
}

// Flow control functions
func Control() template.FuncMap {
	return template.FuncMap{
		"contains":       contains,
		"containsAll":    containsSomeFunc(true, true),
		"containsAny":    containsSomeFunc(false, false),
		"containsNone":   containsSomeFunc(true, false),
		"containsNotAll": containsSomeFunc(false, true),
		"in":             in,
		"loop":           loop,
	}
}

// Mathimatical functions
func Math() template.FuncMap {
	return template.FuncMap{
		"add":      add,
		"subtract": subtract,
		"multiply": multiply,
		"divide":   divide,
		"modulo":   modulo,
		"minimum":  minimum,
		"maximum":  maximum,
	}
}

// And the rest... (maybe organize these more?)
func Helpers() template.FuncMap {
	return template.FuncMap{
		// Parsing
		"parseBool":  parseBool,
		"parseFloat": parseFloat,
		"parseInt":   parseInt,
		"parseJSON":  parseJSON,
		"parseUint":  parseUint,
		"parseYAML":  parseYAML,
		// ToSomething
		"toLower":               toLower,
		"toUpper":               toUpper,
		"toTitle":               toTitle,
		"toJSON":                toJSON,
		"toJSONPretty":          toJSONPretty,
		"toUnescapedJSON":       toUnescapedJSON,
		"toUnescapedJSONPretty": toUnescapedJSONPretty,
		"toTOML":                toTOML,
		"toYAML":                toYAML,
		// (D)Encoding
		"base64Decode":    base64Decode,
		"base64Encode":    base64Encode,
		"base64URLDecode": base64URLDecode,
		"base64URLEncode": base64URLEncode,
		"sha256Hex":       sha256Hex,
		"md5sum":          md5sum,
		// String
		"join":            join,
		"split":           split,
		"trimSpace":       trimSpace,
		"indent":          indent,
		"replaceAll":      replaceAll,
		"regexReplaceAll": regexReplaceAll,
		"regexMatch":      regexMatch,
		// Data type (map, slice, etc) oriented
		"explode":              explode,
		"explodeMap":           explodeMap,
		"mergeMap":             mergeMap,
		"mergeMapWithOverride": mergeMapWithOverride,
		// Misc/Other
		"timestamp":   timestamp,
		"sockaddr":    sockaddr,
		"writeToFile": writeToFile,
	}
}
