package hcat

import (
	"context"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestResolverRun(t *testing.T) {
	t.Parallel()
	t.Run("first-run", func(t *testing.T) {
		rv := NewResolver()
		tt := echoTemplate("foo")
		w := blindWatcher()
		defer w.Stop()
		w.Register(tt)

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete != false {
			t.Fatal("Complete should be false")
		}
		if string(r.Contents) != "" {
			t.Error("bad contents")
		}
	})

	t.Run("no-changes", func(t *testing.T) {
		rv := NewResolver()
		tt := echoTemplate("foo")
		w := blindWatcher()
		defer w.Stop()
		w.Register(tt)

		// Run/Wait/Run to get it completed once
		_, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		w.Wait(context.Background())
		_, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}

		// This run should have no changes
		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}

		if string(r.Contents) != "foo" {
			t.Error("bad contents")
		}
		if r.NoChange != true {
			t.Error("NoChange should be true")
		}
		if r.Complete != true {
			t.Error("Complete should be true")
		}
	})

	t.Run("complete-changes", func(t *testing.T) {
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := echoTemplate("foo")
		w.Register(tt)
		d := &idep.FakeDep{Name: "foo"}

		// seed the cache and the dependency tracking
		// maybe abstract out into separate function
		regSave := func(d dep.Dependency, value interface{}) {
			v := w.track(tt, d)         // register with watcher
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
	})

	t.Run("not-dirty-should-not-mean-complete", func(t *testing.T) {
		// Tests a situation where template has unresolved dependencies
		// but they don't count against complete as they haven't been
		// marked in use yet.
		// Basically this tests a bug where the mark-n-sweep information was
		// being used in the complete check erroneously.
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := echoTemplate("foo")
		w.Register(tt)

		// Run it once to get everything registered
		_, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}

		// Flush the dirty flat and re-run
		// pre-fix it would return Complete=true w/ no Contents
		tt.isDirty()
		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete == true {
			t.Fatal("Complete should be false")
		}
		if string(r.Contents) != "" {
			t.Fatal("bad contents")
		}
		if r.NoChange != true {
			t.Fatal("NoChange should be false")
		}
	})

	// actually run using an injected fake dependency
	// test dependency echo's back the string arg
	t.Run("single-pass-run", func(t *testing.T) {
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := echoTemplate("foo")
		w.Register(tt)

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete != false {
			t.Fatal("Complete should be false")
		}
		ctx := context.Background()
		w.Wait(ctx) // wait for (fake/instantaneous) dependency resolution

		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
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

	// same as above, but with buffering enabled to verify things work with it
	t.Run("buffered-single-pass-run", func(t *testing.T) {
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := echoTemplate("foo")
		w.Register(tt)
		w.SetBufferPeriod(time.Millisecond, time.Second, tt.ID())

		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		ctx := context.Background()
		w.Wait(ctx) // wait for (fake/instantaneous) dependency resolution

		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
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
		w := blindWatcher()
		defer w.Stop()
		tt := echoListTemplate("foo", "bar")
		w.Register(tt)

		// Run 1, 'words' is registered
		r, err := rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete != false {
			t.Fatal("Complete should be flase")
		}
		ctx := context.Background()
		w.Wait(ctx)

		// Run 2, loops and registers both nested 'echo' calls
		r, err = rv.Run(tt, w)
		if err != nil {
			t.Fatal("Run() error:", err)
		}
		if r.Complete != false {
			t.Fatal("Complete should be false")
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
		if r.Complete != true {
			t.Fatal("Complete should be true")
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

	// Test issue where Run/Wait would loop forever as each Run would set the
	// buffering timer, triggering a bufferTrigger which exited Wait..
	// This shouldn't ever be an issue again with the new implementation.
	t.Run("buffered-resolve-loop-hang-check", func(t *testing.T) {
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := echoTemplate("foo")
		w.Register(tt)
		w.SetBufferPeriod(time.Millisecond, time.Second, tt.ID())

		ctx, cancel := context.WithTimeout(context.Background(),
			time.Millisecond*30)
		defer cancel()

		// should stop on second loop
		for i := 0; i < 4; i++ {
			_, err := rv.Run(tt, w)
			if err != nil {
				t.Fatal("Run() error:", err)
			}
			select {
			case <-w.WaitCh(ctx):
			case <-ctx.Done():
				return
			}
		}
		t.Error("bad behavior")
	})

	// Tests that the buffer actually buffers. Uses timed dependencies that
	// return once fast and once after a delay. This way it does the initial
	// fast run and we can check the contents, then the second batch comes in
	// spread out in time but they are should all be included in the second
	// update due to buffering. Without buffering it would return after the
	// first timed one returns.
	t.Run("buffer-timing-test", func(t *testing.T) {
		rv := NewResolver()
		w := blindWatcher()
		defer w.Stop()
		tt := timedEchoTemplate("foo")
		w.Register(tt)

		// Disabling this should fail the test
		// Buffer minumum value is used to reset the timer, so to fail with
		// buffering on you'll need to set it to <10ms (by a bit to avoid racing)
		w.SetBufferPeriod(time.Millisecond*30, time.Millisecond*500, tt.ID())
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Millisecond*300)
		defer cancel()

		stageOneComplete := false
		stageOneTarget := "foo_0s foo_0s foo_0s"
		stageTwoTarget := "foo_10ms foo_20ms foo_30ms"
		// run 0 initializes the template and variables, triggering lookups
		// run 1 gets the fast responses version
		// run 2 gets the delayed responses version
		for i := 0; i < 3; i++ {
			r, err := rv.Run(tt, w)
			if err != nil {
				t.Fatal("Run() error:", err)
			}
			if !r.Complete {
				w.Wait(ctx)
				continue
			}
			switch stageOneComplete {
			case false:
				if string(r.Contents) != stageOneTarget {
					t.Errorf("first pass contents wrong. want: '%s', got '%s'",
						stageOneTarget, r.Contents)
				}
				stageOneComplete = true
			case true:
				if string(r.Contents) != stageTwoTarget {
					t.Errorf("second pass contents wrong. want: '%s', got '%s'",
						stageTwoTarget, r.Contents)
				}
				return
			}
			w.Wait(ctx)
		}
		t.Error("Updating data failed?!?")
	})
}

//////////////////////////
// Helpers

func echoTemplate(data string) *Template {
	return NewTemplate(
		TemplateInput{
			Contents:     `{{echo "` + data + `"}}`,
			FuncMapMerge: template.FuncMap{"echo": echoFunc},
		})
}

func echoFunc(recall Recaller) interface{} {
	return func(s string) interface{} {
		d := &idep.FakeDep{Name: s}
		if value, ok := recall(d); ok {
			if value == nil {
				return ""
			}
			return value.(string)
		}
		return ""
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

func echoListTemplate(data ...string) *Template {
	list := strings.Join(data, `" "`)
	return NewTemplate(
		TemplateInput{
			Contents: `{{range words "` + list + `"}}{{echo .}}{{end}}`,
			FuncMapMerge: template.FuncMap{
				"echo":  echoFunc,
				"words": wordListFunc},
		})
}

func timedEchoFunc(delay time.Duration) interface{} {
	return func(recall Recaller) interface{} {
		return func(s string) interface{} {
			d := &idep.FakeTimedUpdateDep{Name: s, Delay: delay}
			if value, ok := recall(d); ok {
				if value == nil {
					return ""
				}
				return value.(string)
			}
			return ""
		}
	}
}

func timedEchoTemplate(data string) *Template {
	return NewTemplate(
		TemplateInput{
			Contents: `{{echo1 "` + data + `"}} ` +
				`{{echo2 "` + data + `"}} ` +
				`{{echo3 "` + data + `"}}`,
			FuncMapMerge: template.FuncMap{
				"echo1": timedEchoFunc(time.Millisecond * 10),
				"echo2": timedEchoFunc(time.Millisecond * 20),
				"echo3": timedEchoFunc(time.Millisecond * 30),
			},
		})
}

// watcher with no Looker
func blindWatcher() *Watcher {
	return NewWatcher(WatcherInput{Cache: NewStore()})
}
