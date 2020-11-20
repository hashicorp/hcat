package hcat

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"text/template"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

// ErrMissingValues is the error returned when a template doesn't completely
// render due to missing values (values that haven't been fetched yet).
var ErrMissingValues = errors.New("missing template values")
var ErrNoNewValues = errors.New("no new values for template")

// Template is the internal representation of an individual template to process.
// The template retains the relationship between it's contents and is
// responsible for it's own execution.
type Template struct {
	// contents is the string contents for the template. It is either given
	// during template creation or read from disk when initialized.
	contents string

	// dirty indicates that the template's data has been updated and it
	// needs to be re-rendered
	dirty drainableChan

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

// Renderer defines the interface used to render (output) and template.
// FileRenderer implements this to write to disk.
type Renderer interface {
	Render(contents []byte) (RenderResult, error)
}

// Recaller is the read interface for the cache
// Implemented by Store and Watcher (which wraps Store)
type Recaller func(dep.Dependency) (value interface{}, found bool)

// TemplateInput is used as input when creating the template.
type TemplateInput struct {
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
	//
	// There is a special case for FuncMapMerge where, if matched, gets
	// called with the cache, used and missing sets (like the dependency
	// functions) should return a function that matches a signature required
	// by text/template's Funcmap (masked by an interface).
	// This special case function's signature should match:
	//    func(Recaller, *DepSet, *DepSet) interface{}
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
func NewTemplate(i TemplateInput) *Template {

	var t Template
	t.contents = i.Contents
	t.leftDelim = i.LeftDelim
	t.rightDelim = i.RightDelim
	t.errMissingKey = i.ErrMissingKey
	t.sandboxPath = i.SandboxPath
	t.funcMapMerge = i.FuncMapMerge
	t.renderer = i.Renderer
	t.dirty = make(drainableChan, 1)
	t.Notify(nil) // prime template as needing to be run

	// Compute the MD5, encode as hex
	hash := md5.Sum([]byte(t.contents))
	t.hexMD5 = hex.EncodeToString(hash[:])

	return &t
}

// ID returns the identifier for this template.
func (t *Template) ID() string {
	return t.hexMD5
}

// Notify template that a dependency it relies on has been updated.
func (t *Template) Notify(dep.Dependency) {
	select {
	case t.dirty <- struct{}{}:
	default:
	}
}

// Check and clear dirty flag
func (t *Template) isDirty() bool {
	select {
	case <-t.dirty:
		return true
	default:
		return false
	}
}

// Render calls the stored Renderer with the passed content
func (t *Template) Render(content []byte) (RenderResult, error) {
	return t.renderer.Render(content)
}

// Execute evaluates this template in the provided context.
func (t *Template) Execute(w Watcherer) ([]byte, error) {
	if !t.isDirty() {
		return nil, ErrNoNewValues
	}

	tmpl := template.New(t.ID())
	tmpl.Delims(t.leftDelim, t.rightDelim)
	tmpl.Funcs(funcMap(&funcMapInput{
		recaller:     w.Recaller(t),
		funcMapMerge: t.funcMapMerge,
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

	// Checks if all values in use have been fetched.
	// ..also cleans out data no longer used by this template.
	if !w.Complete(t) {
		return nil, ErrMissingValues
	}

	return b.Bytes(), nil
}

// funcMapInput is input to the funcMap, which builds the template functions.
type funcMapInput struct {
	recaller     Recaller
	funcMapMerge template.FuncMap
}

// funcMap is the map of template functions to their respective functions.
func funcMap(i *funcMapInput) template.FuncMap {

	r := template.FuncMap{
		"datacenters":  datacentersFunc(i.recaller),
		"key":          keyFunc(i.recaller),
		"keyExists":    keyExistsFunc(i.recaller),
		"keyOrDefault": keyWithDefaultFunc(i.recaller),
		"ls":           lsFunc(i.recaller, true),
		"safeLs":       safeLsFunc(i.recaller),
		"node":         nodeFunc(i.recaller),
		"nodes":        nodesFunc(i.recaller),
		"secret":       secretFunc(i.recaller),
		"secrets":      secretsFunc(i.recaller),
		"service":      serviceFunc(i.recaller),
		"connect":      connectFunc(i.recaller),
		"services":     servicesFunc(i.recaller),
		"tree":         treeFunc(i.recaller, true),
		"safeTree":     safeTreeFunc(i.recaller),
		"caRoots":      connectCARootsFunc(i.recaller),
		"caLeaf":       connectLeafFunc(i.recaller),
	}

	for k, v := range i.funcMapMerge {
		switch f := v.(type) {
		case func(Recaller) interface{}:
			r[k] = f(i.recaller)
		default:
			r[k] = v
		}
	}

	return r
}
