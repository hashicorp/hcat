package tfunc

import (
	"fmt"
	"testing"
	"text/template"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func testHealthServiceQueryID(service string) string {
	return fmt.Sprintf("health.service(%s|passing)", service)
}

func testKVListQueryID(prefix string) string {
	return fmt.Sprintf("kv.list(%s)", prefix)
}

// simple check for duplicate names for template functions
func TestAllForDups(t *testing.T) {
	all := make(template.FuncMap)
	allfuncs := []func() template.FuncMap{
		ConsulFilters, Env, Control, Helpers, Math, Sprig}
	for _, f := range allfuncs {
		for k, v := range f() {
			if _, ok := all[k]; ok {
				t.Fatal("duplicate entry")
			}
			all[k] = v
		}
	}
}

// Return a new template with all unversioned and V0 template functions.
func newTemplate(ti hcat.TemplateInput) *hcat.Template {
	funcMap := AllUnversioned()
	// use vault v0 api as that is all that is currently supported
	for k, v := range VaultV0() {
		funcMap[k] = v
	}
	// use consul v0 api as default for now as most tests use it
	for k, v := range ConsulV0() {
		funcMap[k] = v
	}
	switch ti.FuncMapMerge {
	case nil:
	default:
		// allow passed in option to override defaults
		for k, v := range ti.FuncMapMerge {
			funcMap[k] = v
		}
	}
	ti.FuncMapMerge = funcMap
	return hcat.NewTemplate(ti)
}

// fake/stub Watcherer for tests
type fakeWatcher struct {
	*hcat.Store
}

func (fakeWatcher) Buffering(hcat.Notifier) bool  { return false }
func (f fakeWatcher) Complete(hcat.Notifier) bool { return true }
func (f fakeWatcher) Recaller(hcat.Notifier) hcat.Recaller {
	return func(d dep.Dependency) (value interface{}, found bool) {
		return f.Store.Recall(d.ID())
	}
}
