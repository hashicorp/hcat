package hat

import (
	"strings"
	"testing"
	"text/template"

	dep "github.com/hashicorp/hat/internal/dependency"
)

func TestResolverRun(t *testing.T) {
	t.Parallel()
	t.Run("first-run", func(t *testing.T) {
		rv := NewResolver()
		tt := fooTemplate(t)
		w := blindWatcher(t)
		defer w.Stop()

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
	})

	t.Run("skip-dueto-no-changes", func(t *testing.T) {
		rv := NewResolver()
		tt := fooTemplate(t)
		w := blindWatcher(t)
		defer w.Stop()

		// seed the dependency tracking
		// otherwise it will trigger first run
		w.TemplateDeps(tt.ID(), &dep.FakeDep{Name: "foo"})
		// basically this is what we're testing

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete != false {
			t.Fatal("Complete should be false")
		}
		if string(r.Contents) != "" {
			t.Fatal("bad contents")
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
	})

	t.Run("complete", func(t *testing.T) {
		rv := NewResolver()
		tt := fooTemplate(t)
		w := blindWatcher(t)
		defer w.Stop()

		d, _ := dep.NewKVGetQuery("foo")
		d.EnableBlocking()
		// seed the cache and the dependency tracking
		w.cache.Save(d.String(), "bar")
		r, err := rv.Run(tt, w)

		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete == false {
			t.Fatal("Complete should be true")
		}
		if string(r.Contents) != "bar" {
			t.Fatal("bad contents")
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
	})

	// actually run using an injected fake dependency
	// test dependency echo's back the string arg
	t.Run("single-pass-run", func(t *testing.T) {
		rv := NewResolver()
		tt := echoTemplate(t, "foo")
		w := blindWatcher(t)
		defer w.Stop()

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		w.Wait(0) // wait for (fake/instantaneous) dependency resolution

		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
		if r.Complete == false {
			t.Fatal("Complete should be true")
		}
		if string(r.Contents) != "foo" {
			t.Fatal("Wrong contents:", string(r.Contents))
		}
	})

	// actually run using injected fake dependencies
	// dep1 returns a list of words where dep2 echos each
	t.Run("multi-pass-run", func(t *testing.T) {
		rv := NewResolver()
		tt := echoListTemplate(t, "foo", "bar")
		w := blindWatcher(t)
		defer w.Stop()

		// Run 1, 'words' is missing
		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		w.Wait(0) // wait for (fake/instantaneous) dependency resolution

		// Run 2, 'echo foo' is missing
		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		w.Wait(0)

		// Run 3, 'echo bar' is missing
		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		w.Wait(0)

		// Run 4, complete
		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
		if r.Complete == false {
			t.Fatal("Complete should be true")
		}
		if string(r.Contents) != "foobar" {
			t.Fatal("Wrong contents:", string(r.Contents))
		}
	})
}

//////////////////////////
// Helpers

func fooTemplate(t *testing.T) *Template {
	tm, err := NewTemplate(
		&NewTemplateInput{
			Contents: `{{key "foo"}}`,
		})
	if err != nil {
		t.Fatal("new template error:", err)
	}
	return tm
}

func echoTemplate(t *testing.T, data string) *Template {
	tm, err := NewTemplate(
		&NewTemplateInput{
			Contents:     `{{echo "` + data + `"}}`,
			FuncMapMerge: template.FuncMap{"echo": echoFunc},
		})
	if err != nil {
		t.Fatal("new template error:", err)
	}
	return tm
}

func echoFunc(r Recaller, used, missing *DepSet) interface{} {
	return func(s string) (interface{}, error) {
		d := &dep.FakeDep{Name: s}
		used.Add(d)
		if value, ok := r.Recall(d.String()); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}
		missing.Add(d)
		return "", nil
	}
}

func wordListFunc(r Recaller, used, missing *DepSet) interface{} {
	return func(s ...string) interface{} {
		d := &dep.FakeListDep{
			Name: "words",
			Data: s,
		}
		used.Add(d)
		if value, ok := r.Recall(d.String()); ok {
			if value == nil {
				return []string{}
			}
			return value.([]string)
		}
		missing.Add(d)
		return []string{}
	}
}

func echoListTemplate(t *testing.T, data ...string) *Template {
	list := strings.Join(data, `" "`)
	tm, err := NewTemplate(
		&NewTemplateInput{
			Contents: `{{range words "` + list + `"}}{{echo .}}{{end}}`,
			FuncMapMerge: template.FuncMap{
				"echo":  echoFunc,
				"words": wordListFunc},
		})
	if err != nil {
		t.Fatal("new template error:", err)
	}
	return tm
}

// watcher with no Looker
func blindWatcher(t *testing.T) *Watcher {
	w, err := NewWatcher(&NewWatcherInput{Cache: NewStore()})
	if err != nil {
		t.Fatal("new watcher error:", err)
	}
	return w
}
