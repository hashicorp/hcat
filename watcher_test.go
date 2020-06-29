package hcat

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	dep "github.com/hashicorp/hcat/internal/dependency"
	"github.com/pkg/errors"
)

func TestWatcherAdd(t *testing.T) {
	t.Run("updates-map", func(t *testing.T) {
		w := newWatcher(t)

		d := &dep.FakeDep{}
		if added := w.Add(d); !added {
			t.Fatal("expected add to return true")
		}

		_, exists := w.depViewMap[d.String()]
		if !exists {
			t.Errorf("expected add to append to map")
		}
	})
	t.Run("exists", func(t *testing.T) {
		w := newWatcher(t)

		d := &dep.FakeDep{}
		w.depViewMap[d.String()] = &view{}

		if added := w.Add(d); added {
			t.Errorf("expected add to return false")
		}
	})
	t.Run("startsViewPoll", func(t *testing.T) {
		w := newWatcher(t)

		if added := w.Add(&dep.FakeDep{}); !added {
			t.Errorf("expected add to return true")
		}

		select {
		case err := <-w.errCh:
			t.Fatal(err)
		case <-w.dataCh:
			// Got data, which means the poll was started
		}
	})
}

func TestWatcherWatching(t *testing.T) {
	t.Run("not-exists", func(t *testing.T) {
		w := newWatcher(t)

		d := &dep.FakeDep{}
		if w.watching(d.String()) == true {
			t.Errorf("expected to not be watching")
		}
	})

	t.Run("exists", func(t *testing.T) {
		w := newWatcher(t)

		d := &dep.FakeDep{}
		w.Add(d)

		if w.watching(d.String()) == false {
			t.Errorf("expected to be watching")
		}
	})
}

func TestWatcherRemove(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		w := newWatcher(t)

		d := &dep.FakeDep{}
		w.Add(d)

		removed := w.remove(d.String())
		if removed != true {
			t.Error("expected Remove to return true")
		}

		if _, ok := w.depViewMap[d.String()]; ok {
			t.Error("expected dependency to be removed")
		}
	})

	t.Run("does-not-exist", func(t *testing.T) {
		w := newWatcher(t)

		var fd dep.FakeDep
		removed := w.remove(fd.String())
		if removed != false {
			t.Fatal("expected Remove to return false")
		}
	})
}

func TestWatcherSize(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		w := newWatcher(t)

		if w.Size() != 0 {
			t.Errorf("expected %d to be %d", w.Size(), 0)
		}
	})

	t.Run("returns-num-views", func(t *testing.T) {
		w := newWatcher(t)

		for i := 0; i < 10; i++ {
			d := &dep.FakeDep{Name: fmt.Sprintf("%d", i)}
			w.Add(d)
		}

		if w.Size() != 10 {
			t.Errorf("expected %d to be %d", w.Size(), 10)
		}
	})
}

func TestWatcherWait(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		t1 := time.Now()
		err := w.Wait(time.Millisecond)
		if err != nil {
			t.Fatal("Error not expected")
		}
		dur := time.Now().Sub(t1)
		if dur < time.Millisecond || dur > time.Millisecond*2 {
			t.Fatal("Wait call was off;", dur)
		}
	})
	t.Run("0-timeout", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		t1 := time.Now()
		testerr := errors.New("test")
		go func() {
			time.Sleep(time.Millisecond)
			w.errCh <- testerr
		}()
		w.Wait(0)
		dur := time.Now().Sub(t1)
		if dur < time.Millisecond || dur > time.Millisecond*2 {
			t.Fatal("Wait call was off;", dur)
		}
	})
	t.Run("error", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		testerr := errors.New("test")
		go func() {
			w.errCh <- testerr
		}()
		err := w.Wait(0)
		if err != testerr {
			t.Fatal("None or Unexpected Error;", err)
		}
	})
	t.Run("remove-old-dependency", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		d := &dep.FakeDep{}
		w.Add(d)
		if _, ok := w.depViewMap[d.String()]; !ok {
			t.Error("expected dependency to be present")
		}
		w.cleanDeps(nil)
		if _, ok := w.depViewMap[d.String()]; ok {
			t.Error("expected dependency to be removed")
		}
	})
	// Test cache updates
	t.Run("simple-update", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		foodep := &dep.FakeDep{Name: "foo"}
		view := newView(&newViewInput{
			Dependency: foodep,
		})
		w.dataCh <- view
		w.Wait(0)
		store := w.cache.(*Store)
		if _, ok := store.data[foodep.String()]; !ok {
			t.Fatal("failed update")
		}
	})
	t.Run("multi-update", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		deps := make([]Dependency, 5)
		views := make([]*view, 5)
		for i := 0; i < 5; i++ {
			deps[i] = &dep.FakeDep{Name: strconv.Itoa(i)}
			views[i] = newView(&newViewInput{
				Dependency: deps[i],
			})
		}
		go func() {
			for _, v := range views {
				w.dataCh <- v
			}
		}()
		w.Wait(0)
		store := w.cache.(*Store)
		if len(store.data) != 5 {
			t.Fatal("failed update")
		}
		if _, ok := store.data[deps[3].String()]; !ok {
			t.Fatal("failed update")
		}
	})
	// test tracking of updated dependencies
	t.Run("simple-updated-tracking", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		foodep := &dep.FakeDep{Name: "foo"}
		view := newView(&newViewInput{
			Dependency: foodep,
		})
		w.dataCh <- view
		w.Wait(0)
		if w.changed.Len() != 1 {
			t.Fatal("failed to track updated dependency")
		}
	})
	t.Run("multi-updated-tracking", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		deps := make([]Dependency, 5)
		views := make([]*view, 5)
		for i := 0; i < 5; i++ {
			deps[i] = &dep.FakeDep{Name: strconv.Itoa(i)}
			views[i] = newView(&newViewInput{
				Dependency: deps[i],
			})
		}
		go func() {
			for _, v := range views {
				w.dataCh <- v
			}
		}()
		w.Wait(0)
		if w.changed.Len() != 5 {
			t.Fatal("failed to track updated dependency")
		}
	})
	t.Run("duplicate-updated-tracking", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Stop()
		for i := 0; i < 2; i++ {
			foodep := &dep.FakeDep{Name: "foo"}
			view := newView(&newViewInput{
				Dependency: foodep,
			})
			w.dataCh <- view
		}
		w.Wait(0)
		if w.changed.Len() != 1 {
			t.Fatal("failed to track updated dependency")
		}
	})

}

func newWatcher(t *testing.T) *Watcher {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
		Cache:   NewStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
