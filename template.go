package hat

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"text/template"

	"github.com/pkg/errors"
)

// ErrTemplateMissingContents is the error returned when a template does
// not specify "content" argument, which is not valid.
var errTemplateMissingContents = errors.New("template: missing 'contents'")

// Template is the internal representation of an individual template to process.
// The template retains the relationship between it's contents and is
// responsible for it's own execution.
type Template struct {
	// contents is the string contents for the template. It is either given
	// during template creation or read from disk when initialized.
	contents string

	// leftDelim and rightDelim are the template delimiters.
	leftDelim  string
	rightDelim string

	// hexMD5 stores the hex version of the MD5
	hexMD5 string

	// errMissingKey causes the template processing to exit immediately if a map
	// is indexed with a key that does not exist.
	errMissingKey bool

	// FuncMapMerge a map of functions that add-to or override
	// those used when executing the template. (text/template)
	funcMapMerge template.FuncMap

	// sandboxPath adds a prefix to any path provided to the `file` function
	// and causes an error if a relative path tries to traverse outside that
	// prefix.
	sandboxPath string

	// Renderer is the default renderer used for this template
	renderer Renderer
}

// Implemented by FileRenderer
type Renderer interface {
	Render([]byte) (RenderResult, error)
}

// Recaller is the read interface for the cache
// Implemented by Store and Watcher (which wraps Store)
type Recaller interface {
	Recall(string) (interface{}, bool)
}

// NewTemplateInput is used as input when creating the template.
type NewTemplateInput struct {
	// Contents are the raw template contents.
	Contents string

	// ErrMissingKey causes the template parser to exit immediately with an
	// error when a map is indexed with a key that does not exist.
	ErrMissingKey bool

	// LeftDelim and RightDelim are the template delimiters.
	LeftDelim  string
	RightDelim string

	// FuncMapMerge a map of functions that add-to or override those used when
	// executing the template. (text/template)

	// There is a special case for the FuncMapMerge where, if matched, gets
	// called with the cache, used and missing sets (like the dependency
	// functions) should return a function that works with the templates
	// FuncMap. 2 variants are accepted:
	// func(Recaller, *depSet, *depSet) func(string) (interface{}, error)
	// func(Recaller, *depSet, *depSet) func(string) interface{}
	// (note the returned funcs match those accepted by the FuncMap)
	FuncMapMerge template.FuncMap

	// SandboxPath adds a prefix to any path provided to the `file` function
	// and causes an error if a relative path tries to traverse outside that
	// prefix.
	SandboxPath string

	// Renderer is the default renderer used for this template
	Renderer Renderer
}

// NewTemplate creates and parses a new Consul Template template at the given
// path. If the template does not exist, an error is returned. During
// initialization, the template is read and is parsed for dependencies. Any
// errors that occur are returned.
func NewTemplate(i *NewTemplateInput) (*Template, error) {
	if i == nil {
		i = &NewTemplateInput{}
	}

	if i.Contents == "" {
		return nil, errTemplateMissingContents
	}

	var t Template
	t.contents = i.Contents
	t.leftDelim = i.LeftDelim
	t.rightDelim = i.RightDelim
	t.errMissingKey = i.ErrMissingKey
	t.sandboxPath = i.SandboxPath
	t.funcMapMerge = i.FuncMapMerge

	// Compute the MD5, encode as hex
	hash := md5.Sum([]byte(t.contents))
	t.hexMD5 = hex.EncodeToString(hash[:])

	return &t, nil
}

// ID returns the identifier for this template.
func (t *Template) ID() string {
	return t.hexMD5
}

// Render calls the stored Renderer with the passed content
func (t *Template) Render(content []byte) (RenderResult, error) {
	return t.renderer.Render(content)
}

// ExecuteResult is the result of the template execution.
type ExecuteResult struct {
	// Used is the set of dependencies that were used.
	Used DepSet

	// Missing is the set of dependencies that were missing.
	Missing DepSet

	// Output the (possibly partially) filled in template
	Output []byte
}

// Execute evaluates this template in the provided context.
func (t *Template) Execute(r Recaller) (*ExecuteResult, error) {
	var used, missing = NewDepSet(), NewDepSet()

	tmpl := template.New(t.ID())
	tmpl.Delims(t.leftDelim, t.rightDelim)

	tmpl.Funcs(funcMap(&funcMapInput{
		t:            tmpl,
		store:        r,
		used:         used,
		missing:      missing,
		funcMapMerge: t.funcMapMerge,
		sandboxPath:  t.sandboxPath,
	}))

	if t.errMissingKey {
		tmpl.Option("missingkey=error")
	} else {
		tmpl.Option("missingkey=zero")
	}

	tmpl, err := tmpl.Parse(t.contents)
	if err != nil {
		return nil, errors.Wrap(err, "parse")
	}

	// Execute the template into the writer
	var b bytes.Buffer
	if err := tmpl.Execute(&b, nil); err != nil {
		return nil, errors.Wrap(err, "execute")
	}

	return &ExecuteResult{
		Used:    *used,
		Missing: *missing,
		Output:  b.Bytes(),
	}, nil
}

// funcMapInput is input to the funcMap, which builds the template functions.
type funcMapInput struct {
	t            *template.Template
	store        Recaller
	env          []string
	funcMapMerge template.FuncMap
	sandboxPath  string
	used         *DepSet
	missing      *DepSet
}

// funcMap is the map of template functions to their respective functions.
func funcMap(i *funcMapInput) template.FuncMap {
	var scrat scratch

	r := template.FuncMap{
		// API functions
		"datacenters":  datacentersFunc(i.store, i.used, i.missing),
		"file":         fileFunc(i.store, i.used, i.missing, i.sandboxPath),
		"key":          keyFunc(i.store, i.used, i.missing),
		"keyExists":    keyExistsFunc(i.store, i.used, i.missing),
		"keyOrDefault": keyWithDefaultFunc(i.store, i.used, i.missing),
		"ls":           lsFunc(i.store, i.used, i.missing, true),
		"safeLs":       safeLsFunc(i.store, i.used, i.missing),
		"node":         nodeFunc(i.store, i.used, i.missing),
		"nodes":        nodesFunc(i.store, i.used, i.missing),
		"secret":       secretFunc(i.store, i.used, i.missing),
		"secrets":      secretsFunc(i.store, i.used, i.missing),
		"service":      serviceFunc(i.store, i.used, i.missing),
		"connect":      connectFunc(i.store, i.used, i.missing),
		"services":     servicesFunc(i.store, i.used, i.missing),
		"tree":         treeFunc(i.store, i.used, i.missing, true),
		"safeTree":     safeTreeFunc(i.store, i.used, i.missing),
		"caRoots":      connectCARootsFunc(i.store, i.used, i.missing),
		"caLeaf":       connectLeafFunc(i.store, i.used, i.missing),

		// scratch
		"scratch": func() *scratch { return &scrat },

		// Helper functions
		"base64Decode":    base64Decode,
		"base64Encode":    base64Encode,
		"base64URLDecode": base64URLDecode,
		"base64URLEncode": base64URLEncode,
		"byKey":           byKey,
		"byTag":           byTag,
		"contains":        contains,
		"containsAll":     containsSomeFunc(true, true),
		"containsAny":     containsSomeFunc(false, false),
		"containsNone":    containsSomeFunc(true, false),
		"containsNotAll":  containsSomeFunc(false, true),
		"env":             envFunc(i.env),
		"executeTemplate": executeTemplateFunc(i.t),
		"explode":         explode,
		"explodeMap":      explodeMap,
		"in":              in,
		"indent":          indent,
		"loop":            loop,
		"join":            join,
		"trimSpace":       trimSpace,
		"parseBool":       parseBool,
		"parseFloat":      parseFloat,
		"parseInt":        parseInt,
		"parseJSON":       parseJSON,
		"parseUint":       parseUint,
		"parseYAML":       parseYAML,
		"plugin":          plugin,
		"regexReplaceAll": regexReplaceAll,
		"regexMatch":      regexMatch,
		"replaceAll":      replaceAll,
		"sha256Hex":       sha256Hex,
		"timestamp":       timestamp,
		"toLower":         toLower,
		"toJSON":          toJSON,
		"toJSONPretty":    toJSONPretty,
		"toTitle":         toTitle,
		"toTOML":          toTOML,
		"toUpper":         toUpper,
		"toYAML":          toYAML,
		"split":           split,
		"byMeta":          byMeta,
		"sockaddr":        sockaddr,
		// Math functions
		"add":      add,
		"subtract": subtract,
		"multiply": multiply,
		"divide":   divide,
		"modulo":   modulo,
		"minimum":  minimum,
		"maximum":  maximum,
	}

	type depFunc1 = func(Recaller, *DepSet, *DepSet) func(string) interface{}
	type depFunc2 = func(Recaller, *DepSet, *DepSet) func(string) (interface{}, error)
	for k, v := range i.funcMapMerge {
		switch f := v.(type) {
		case depFunc1:
			r[k] = f(i.store, i.used, i.missing)
		case depFunc2:
			r[k] = f(i.store, i.used, i.missing)
		default:
			r[k] = v
		}
	}

	return r
}
