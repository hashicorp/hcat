package hat

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	dep "github.com/hashicorp/hat/internal/dependency"
	"github.com/pkg/errors"
)

func TestAdd_updatesMap(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	d := &dep.FakeDep{}
	if _, err := w.add(d); err != nil {
		t.Fatal(err)
	}

	_, exists := w.depViewMap[d.String()]
	if !exists {
		t.Errorf("expected add to append to map")
	}
}

func TestAdd_exists(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	d := &dep.FakeDep{}
	w.depViewMap[d.String()] = &view{}

	added, err := w.add(d)
	if err != nil {
		t.Fatal(err)
	}

	if added != false {
		t.Errorf("expected add to return false")
	}
}

func TestAdd_startsViewPoll(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	added, err := w.add(&dep.FakeDep{})
	if err != nil {
		t.Fatal(err)
	}

	if added != true {
		t.Errorf("expected add to return true")
	}

	select {
	case err := <-w.errCh:
		t.Fatal(err)
	case <-w.dataCh:
		// Got data, which means the poll was started
	}
}

func TestWatching_notExists(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	d := &dep.FakeDep{}
	if w.Watching(d) == true {
		t.Errorf("expected to not be watching")
	}
}

func TestWatching_exists(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	d := &dep.FakeDep{}
	if _, err := w.add(d); err != nil {
		t.Fatal(err)
	}

	if w.Watching(d) == false {
		t.Errorf("expected to be watching")
	}
}

func TestRemove_exists(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	d := &dep.FakeDep{}
	if _, err := w.add(d); err != nil {
		t.Fatal(err)
	}

	removed := w.remove(d)
	if removed != true {
		t.Error("expected Remove to return true")
	}

	if _, ok := w.depViewMap[d.String()]; ok {
		t.Error("expected dependency to be removed")
	}
}

func TestRemove_doesNotExist(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	removed := w.remove(&dep.FakeDep{})
	if removed != false {
		t.Fatal("expected Remove to return false")
	}
}

func TestSize_empty(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if w.Size() != 0 {
		t.Errorf("expected %d to be %d", w.Size(), 0)
	}
}

func TestSize_returnsNumViews(t *testing.T) {
	w, err := NewWatcher(&NewWatcherInput{
		Clients: &clientSet{},
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		d := &dep.FakeDep{Name: fmt.Sprintf("%d", i)}
		if _, err := w.add(d); err != nil {
			t.Fatal(err)
		}
	}

	if w.Size() != 10 {
		t.Errorf("expected %d to be %d", w.Size(), 10)
	}
}

func TestWait(t *testing.T) {
	newWatcher := func() *Watcher {
		w, err := NewWatcher(&NewWatcherInput{
			Clients: &clientSet{},
			Cache:   NewStore(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	t.Run("timeout", func(t *testing.T) {
		w := newWatcher()
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
		w := newWatcher()
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
		w := newWatcher()
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
		w := newWatcher()
		defer w.Stop()
		d := &dep.FakeDep{}
		if _, err := w.add(d); err != nil {
			t.Fatal(err)
		}
		w.olddepCh <- d
		// use timeout to get it to return and give remove time to run
		w.Wait(time.Millisecond)
		if _, ok := w.depViewMap[d.String()]; ok {
			t.Error("expected dependency to be removed")
		}
	})
	t.Run("simple-update", func(t *testing.T) {
		w := newWatcher()
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
		w := newWatcher()
		defer w.Stop()
		deps := make([]dep.Dependency, 5)
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
}
