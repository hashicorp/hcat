package hcat

import (
	"context"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
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
		if r.missing != true {
			t.Fatal("missing should be true")
		}
	})

	t.Run("no-changes", func(t *testing.T) {
		rv := NewResolver()
		tt := fooTemplate(t)
		tt.isDirty() // flush dirty mark set on new templates
		w := blindWatcher(t)
		d, _ := idep.NewKVGetQuery("foo")
		defer w.Stop()

		// seed the dependency tracking
		// otherwise it will trigger first run
		w.Register(tt, d)
		// set receivedData to true to make it think it has it already
		v := w.tracker.view(d.String())
		v.receivedData = true

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
		if r.NoChange != true {
			t.Fatal("NoChange should be true")
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
	})

	t.Run("complete-changes", func(t *testing.T) {
		rv := NewResolver()
		tt := fooTemplate(t)
		w := blindWatcher(t)
		defer w.Stop()
		d, _ := idep.NewKVGetQuery("foo")

		// seed the cache and the dependency tracking
		// maybe abstract out into separate function
		regSave := func(d dep.Dependency, value interface{}) {
			v := w.register(tt, d)      // register with watcher
			v.store(value)              // view received and recorded data
			w.cache.Save(v.ID(), value) // saves data to cache
		}

		regSave(d, "bar")
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
		if r.NoChange != false {
			t.Fatal("NoChange should be false")
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
		ctx := context.Background()
		w.Wait(ctx) // wait for (fake/instantaneous) dependency resolution

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
		if r.NoChange != false {
			t.Fatal("NoChange should be false")
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

		// Run 1, 'words' is registered
		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		ctx := context.Background()
		w.Wait(ctx)

		// Run 2, loops and registers both nested 'echo' calls
		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.missing == false {
			t.Fatal("missing should be true")
		}
		w.Wait(ctx)

		// Run 3-4, fetched 'echo' data comes in and completes the template.
		// Due to asynchronous nature of the test, it is indeterminate whether
		// the data will be received and used in 1 or 2 checks. So we need to
		// loop twice. At the end the 2nd loop the template will be complete or
		// something went wrong.
		for i := 0; i < 2; i++ {
			r, err = rv.Run(tt, w)
			if err != nil {
				t.Fatal("Run() error:", err)
			}
			if r.Complete {
				break // complete in 1 pass, break before Wait or it will hang
			}
			w.Wait(ctx)
		}
		if r.missing == true {
			t.Fatal("missing should be false")
		}
		if r.Complete == false {
			t.Fatal("Complete should be true")
		}
		if r.NoChange != false {
			t.Fatal("NoChange should be false")
		}
		if string(r.Contents) != "foobar" {
			t.Fatal("Wrong contents:", string(r.Contents))
		}
	})
}

//////////////////////////
// Helpers

func fooTemplate(t *testing.T) *Template {
	return NewTemplate(
		TemplateInput{
			Contents: `{{key "foo"}}`,
		})
}

func echoTemplate(t *testing.T, data string) *Template {
	return NewTemplate(
		TemplateInput{
			Contents:     `{{echo "` + data + `"}}`,
			FuncMapMerge: template.FuncMap{"echo": echoFunc},
		})
}

func echoFunc(recall Recaller) interface{} {
	return func(s string) (interface{}, error) {
		d := &idep.FakeDep{Name: s}
		if value, ok := recall(d); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}
		return "", nil
	}
}

func wordListFunc(recall Recaller) interface{} {
	return func(s ...string) interface{} {
		d := &idep.FakeListDep{
			Name: "words",
			Data: s,
		}
		if value, ok := recall(d); ok {
			if value == nil {
				return []string{}
			}
			return value.([]string)
		}
		return []string{}
	}
}

func echoListTemplate(t *testing.T, data ...string) *Template {
	list := strings.Join(data, `" "`)
	return NewTemplate(
		TemplateInput{
			Contents: `{{range words "` + list + `"}}{{echo .}}{{end}}`,
			FuncMapMerge: template.FuncMap{
				"echo":  echoFunc,
				"words": wordListFunc},
		})
}

// watcher with no Looker
func blindWatcher(t *testing.T) *Watcher {
	return NewWatcher(WatcherInput{Cache: NewStore()})
}
