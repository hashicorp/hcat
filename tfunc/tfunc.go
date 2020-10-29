package tfunc

import (
	"os"
	"text/template"
)

func All() template.FuncMap {
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

func Env() template.FuncMap {
	return template.FuncMap{
		"env": envFunc(os.Environ()),
	}
}

func ConsulFilters() template.FuncMap {
	return template.FuncMap{
		"byKey":  byKey,
		"byTag":  byTag,
		"byMeta": byMeta,
	}
}

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
		"toLower":      toLower,
		"toUpper":      toUpper,
		"toTitle":      toTitle,
		"toJSON":       toJSON,
		"toJSONPretty": toJSONPretty,
		"toTOML":       toTOML,
		"toYAML":       toYAML,
		// (D)Encoding
		"base64Decode":    base64Decode,
		"base64Encode":    base64Encode,
		"base64URLDecode": base64URLDecode,
		"base64URLEncode": base64URLEncode,
		"sha256Hex":       sha256Hex,
		// String
		"join":            join,
		"split":           split,
		"trimSpace":       trimSpace,
		"indent":          indent,
		"replaceAll":      replaceAll,
		"regexReplaceAll": regexReplaceAll,
		"regexMatch":      regexMatch,
		// Other
		"explode":    explode,
		"explodeMap": explodeMap,
		"timestamp":  timestamp,
		"sockaddr":   sockaddr,
	}
}
