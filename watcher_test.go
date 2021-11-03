package hcat

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
	"github.com/pkg/errors"
)

func TestWatcherAdd(t *testing.T) {
	t.Run("updates-tracker", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Register(n)
		if added := w.track(n, d); added == nil {
			t.Fatal("Register returned nil")
		}

		if !w.Watching(d.ID()) {
			t.Errorf("expected add to append to map")
		}
	})
	t.Run("exists", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Register(n)
		var added *view
		if added = w.track(n, d); added == nil {
			t.Fatal("Register returned nil")
		}
		if readded := w.track(n, d); readded != added {
			t.Fatal("Register should have returned the already created"+
				"view, instead got:", added)
		}
	})
	t.Run("startsViewPoll", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Register(n)
		if added := w.track(n, d); added == nil {
			t.Fatal("Register returned nil")
		}
		w.Poll(d)

		select {
		case err := <-w.errCh:
			t.Fatal(err)
		case <-w.dataCh:
			// Got data, which means the poll was started
		}
	})
	t.Run("consul-retry-func", func(t *testing.T) {
		w := newWatcher()
		w.retryFuncConsul = func(n int) (bool, time.Duration) {
			return false, 0 * time.Second
		}
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Register(n)
		added := w.track(n, d)
		if added == nil {
			t.Fatal("Register returned nil")
		}
		if added.retryFunc == nil {
			t.Fatal("Retry func was nil")
		}
	})
}

func TestWatcherRegisty(t *testing.T) {

	t.Run("base", func(t *testing.T) {
		w := blindWatcher()
		tt := echoTemplate("foo")
		if err := w.Register(tt); err != nil {
			t.Fatal("error should be nil, got:", err)
		}
		if _, ok := w.tracker.notifiers[tt.ID()]; !ok {
			t.Fatal("registered template not tracked")
		}
	})

	t.Run("duplicate-template-error", func(t *testing.T) {
		w := blindWatcher()
		tt := echoTemplate("foo")
		if err := w.Register(tt); err != nil {
			t.Fatal("error should be nil, got:", err)
		}
		if err := w.Register(tt); err != RegistryErr {
			t.Fatal("should have errored")
		}
	})
}

func TestWatcherWatching(t *testing.T) {
	t.Run("not-exists", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		if w.Watching(d.ID()) == true {
			t.Errorf("expected to not be Watching")
		}
	})

	t.Run("exists", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Track(n, d)

		if w.Watching(d.ID()) == false {
			t.Errorf("expected to be Watching")
		}
	})
	// below are tracking related
	t.Run("ignore-duplicates", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n := fakeNotifier("foo")
		w.Track(n, d)

		if w.Watching(d.ID()) == false {
			t.Errorf("expected to be Watching")
		}
		if w.Size() != 1 {
			t.Errorf("should ignore duplicate entries")
		}
	})
	t.Run("multi-notifiers-same-dep", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d := &idep.FakeDep{}
		n0 := fakeNotifier("foo")
		n1 := fakeNotifier("bar")
		w.Track(n0, d)
		v0 := w.tracker.view(d.ID())
		w.Track(n1, d)
		v1 := w.tracker.view(d.ID())

		// be sure view created for dependency is reused
		if v0 != v1 {
			t.Errorf("previous view overwritten, should reuse first one")
		}

		if w.Watching(d.ID()) == false {
			t.Errorf("expected to be Watching")
		}
		if len(w.tracker.tracked) != 2 {
			t.Errorf("should have 2 entries")
		}

		if w.Complete(n0) {
			t.Errorf("dep has not received data, should not be completed: %s", n0.ID())
		}
		if w.Complete(n1) {
			t.Errorf("dep has not received data, should not be completed: %s", n1.ID())
		}

		if notifiers := w.tracker.notifiersFor(v0); len(notifiers) != 2 {
			t.Errorf("unexpected number of notifiers for view: %s %v", d.ID(), notifiers)
		}
	})
	t.Run("same-notifier-multiple-deps", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d0 := &idep.FakeDep{Name: "foo"}
		d1 := &idep.FakeDep{Name: "bar"}
		n := fakeNotifier("foo")
		w.Track(n, d0)
		w.Track(n, d1)

		if w.Watching(d0.ID()) == false {
			t.Errorf("expected to be Watching")
		}
		if w.Watching(d1.ID()) == false {
			t.Errorf("expected to be Watching")
		}
		if len(w.tracker.tracked) != 2 {
			t.Errorf("should have 2 entries")
		}
	})

	t.Run("multi-notifiers-multi-dep", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		d0 := &idep.FakeDep{Name: "taco"}    // dep for foo and bar
		d1 := &idep.FakeDep{Name: "burrito"} // dep for bar
		nFoo := fakeNotifier("foo")
		nBar := fakeNotifier("bar")
		w.Track(nFoo, d0)
		w.Track(nBar, d0)
		w.Track(nBar, d1)

		// Test that 2 views were created for the 2 dependencies
		if w.tracker.viewCount() != 2 {
			t.Errorf("unexpected number of views, expected 2: %d", w.tracker.viewCount())
		}

		if w.Watching(d0.ID()) == false {
			t.Errorf("expected to be Watching: %s", d0.ID())
		}
		if w.Watching(d1.ID()) == false {
			t.Errorf("expected to be Watching: %s", d1.ID())
		}

		// 2 tracked pairs for foo (taco and burrito), 1 tracked pair for bar (burrito)
		if len(w.tracker.tracked) != 3 {
			t.Errorf("should have 3 entries")
		}

		if w.Complete(nFoo) {
			t.Fatalf("dep has not received data, should not be completed: %s", nFoo.ID())
		}
		if w.Complete(nBar) {
			t.Fatalf("dep has not received data, should not be completed: %s", nBar.ID())
		}

		v0 := w.tracker.view(d0.ID())
		if v0 == nil || w.Watching(d0.ID()) == false {
			t.Errorf("expected to be Watching after Complete: %s", d0.ID())
		}
		if notifiers := w.tracker.notifiersFor(v0); len(notifiers) != 2 {
			t.Errorf("unexpected number of notifiers for view: %s %v", d0.ID(), notifiers)
		}

		v1 := w.tracker.view(d1.ID())
		if v1 == nil || w.Watching(d1.ID()) == false {
			t.Errorf("expected to be Watching after Complete: %s", d1.ID())
		}
		if notifiers := w.tracker.notifiersFor(v1); len(notifiers) != 1 {
			t.Errorf("unexpected number of notifiers for view: %s %v", d1.ID(), notifiers)
		}
	})

	// GH-44
	t.Run("register-complete-race", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		// fake template/notifier and dependencies (fields in template)
		n := fakeNotifier("foo") // template stand-in
		w.Register(n)
		d0 := &idep.FakeDep{Name: "taco"}
		d1 := &idep.FakeDep{Name: "burrito"}

		// The race is between template's Execute and watcher's Complete.
		// First Execute renders the template, registering dependencies
		// and starting the polling. The polling then returns before the
		// Complete call marking everything as retrieved and Complete
		// erroneously returns true. It should require a second pass of the
		// template to render the values into the form before being Complete.
		//
		// To replicate we want to manually force the condition instead of
		// relying on the race, so we're testing by replicating the
		// corresponding behavior sans some timing aspects (no rate limiting,
		// etc.)

		// First template Execute call..
		// 1. each dependency gets registered
		v0 := w.track(n, d0)
		v1 := w.track(n, d1)
		// 2. polling should start, but we'll simulate that manually below
		// Template Execute is now done.

		// Polling now returns data, stores it on the view and marks the
		// view as having received the data (view.receivedData = true)
		fakePollReturn := func(v *view, d dep.Dependency) {
			v.store(d.ID())
		}
		fakePollReturn(v0, d0)
		fakePollReturn(v1, d1)

		// Then the Complete check happens. Should return false as the data
		// hasn't been rendered into the template yet.
		if w.Complete(n) {
			t.Fatalf("Complete should return false until the notifier says it's done.")
		}
	})
}

func TestWatcherVaultToken(t *testing.T) {
	t.Run("empty-token", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		err := w.WatchVaultToken("")
		if err != nil {
			t.Fatal("Didn't expect and error:", err)
		}
		if w.Size() > 0 {
			t.Fatal("dependency should not have been added")
		}
	})
	t.Run("token-added", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		err := w.WatchVaultToken("fake-token")
		if err != nil {
			t.Fatal("Didn't expect and error:", err)
		}
		test_id := (&idep.VaultTokenQuery{}).ID()

		if !w.Watching(test_id) {
			t.Fatal("token dep not added to watcher")
		}
	})
	t.Run("not-cleaned", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		err := w.WatchVaultToken("fake-token")
		if err != nil {
			t.Fatal("Didn't expect and error:", err)
		}
		test_id := (&idep.VaultTokenQuery{}).ID()
		if !w.Watching(test_id) {
			t.Fatal("token dep not added to watcher")
		}
		n := fakeNotifier("some-random-notifier")
		w.Complete(n)
		if !w.Watching(test_id) {
			t.Fatal("token dep should not have been cleaned")
		}
	})
}

func TestWatcherSize(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		if w.Size() != 0 {
			t.Errorf("expected %d to be %d", w.Size(), 0)
		}
	})

	t.Run("returns-num-views", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		for i := 0; i < 10; i++ {
			d := &idep.FakeDep{Name: fmt.Sprintf("%d", i)}
			n := fakeNotifier("foo")
			w.Track(n, d)
		}

		if w.Size() != 10 {
			t.Errorf("expected %d to be %d", w.Size(), 10)
		}
	})
}

func TestWatcherWait(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		t1 := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond*100)
		defer cancel()
		err := w.Wait(ctx)
		if err != nil {
			t.Fatal("Error not expected")
		}
		dur := time.Since(t1)
		if dur < time.Microsecond*100 || dur > time.Millisecond*10 {
			t.Fatal("Wait call was off;", dur)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		t1 := time.Now()
		ctx, cancel := context.WithDeadline(context.Background(),
			time.Now().Add(time.Microsecond*100))
		defer cancel()
		err := w.Wait(ctx)
		if err != nil {
			t.Fatal("Error not expected")
		}
		dur := time.Since(t1)
		if dur < time.Microsecond*100 || dur > time.Millisecond*10 {
			t.Fatal("Wait call was off;", dur)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		errCh := make(chan error)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		go func() {
			err := w.Wait(ctx)
			if err != nil {
				errCh <- err
			}
		}()
		cancel()
		err := <-errCh
		if ctx.Err() != context.Canceled {
			t.Fatal("unexpected context error:", ctx.Err())
		}
		if err != ctx.Err() {
			t.Fatal("unexpected wait error:", err)
		}
	})
	t.Run("0-timeout", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		t1 := time.Now()
		testerr := errors.New("test")
		go func() {
			time.Sleep(time.Microsecond * 100)
			w.errCh <- testerr
		}()
		w.Wait(context.Background())
		dur := time.Since(t1)
		if dur < time.Microsecond*100 || dur > time.Millisecond*10 {
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
		err := w.Wait(context.Background())
		if err != testerr {
			t.Fatal("None or Unexpected Error;", err)
		}
	})
	// Test cache updates
	t.Run("simple-update", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		n := fakeNotifier("foo")
		w.Register(n)
		w.dataCh <- w.track(n, foodep)
		w.Wait(context.Background())
		store := w.cache.(*Store)
		if _, ok := store.data[foodep.ID()]; !ok {
			t.Fatal("failed update")
		}
	})
	t.Run("multi-update", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		n := fakeNotifier("foo")
		w.Register(n)
		deps := make([]dep.Dependency, 5)
		for i := 0; i < 5; i++ {
			deps[i] = &idep.FakeDep{Name: strconv.Itoa(i)}
			// doesn't need goroutine as dataCh has a large buffer
			w.dataCh <- w.track(n, deps[i])
		}
		w.Wait(context.Background())
		store := w.cache.(*Store)
		if len(store.data) != 5 {
			t.Fatal("failed update")
		}
		if _, ok := store.data[deps[3].ID()]; !ok {
			t.Fatal("failed update")
		}
	})
	// test tracking of updated dependencies
	t.Run("simple-updated-tracking", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		n := fakeNotifier("foo")
		w.Register(n)
		w.dataCh <- w.track(n, foodep)
		w.Wait(context.Background())

		if len(w.tracker.tracked) != 1 {
			fmt.Printf("%#v\n", w.tracker)
			t.Fatal("failed to track updated dependency")
		}

		if _, found := w.cache.Recall(foodep.ID()); !found {
			fmt.Printf("%#v\n", w.cache)
			t.Fatal("failed to update cache")
		}
	})
	t.Run("multi-updated-tracking", func(t *testing.T) {
		w := newWatcher()
		n := fakeNotifier("multi")
		w.Register(n)
		defer w.Stop()
		deps := make([]dep.Dependency, 5)
		for i := 0; i < 5; i++ {
			deps[i] = &idep.FakeDep{Name: strconv.Itoa(i)}
			w.dataCh <- w.track(n, deps[i])
			w.Wait(context.Background())
		}
		if n.count() != len(deps) {
			t.Fatal("failed to track updated dependency")
		}
	})
	t.Run("duplicate-updated-tracking", func(t *testing.T) {
		w := newWatcher()
		n := fakeNotifier("dup")
		w.Register(n)
		defer w.Stop()
		for i := 0; i < 2; i++ {
			foodep := &idep.FakeDep{Name: "foo"}
			w.dataCh <- w.track(n, foodep)
		}
		w.Wait(context.Background())
		if n.count() != 2 {
			t.Fatal("didn't receive all notifications")
		}
		if len(w.tracker.views) != 1 {
			t.Fatal("duplicate views for same dependency")
		}
	})
	t.Run("wait-channel", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		n := fakeNotifier("foo")
		w.Register(n)
		foodep := &idep.FakeDep{Name: "foo"}
		w.dataCh <- w.track(n, foodep)
		err := <-w.WaitCh(context.Background())
		if err != nil {
			t.Fatal("wait error:", err)
		}
		store := w.cache.(*Store)
		if _, ok := store.data[foodep.ID()]; !ok {
			t.Fatal("failed update")
		}
	})
	t.Run("wait-channel-cancel", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()

		errCh := make(chan error)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		go func() {
			errCh <- <-w.WaitCh(ctx)
		}()
		cancel()
		err := <-errCh
		if ctx.Err() != context.Canceled {
			t.Fatal("unexpected context error:", ctx.Err())
		}
		if err != ctx.Err() {
			t.Fatal("unexpected wait error:", err)
		}
	})
	t.Run("wait-stop-leak", func(t *testing.T) {
		w := newWatcher()
		errCh := make(chan error)
		go func() {
			errCh <- w.Wait(context.Background())
		}()
		leaked := make(chan bool, 1)
		defer close(leaked)
		<-w.waitingCh
		w.Stop()
		select {
		case <-errCh:
			leaked <- false
		case <-time.After(time.Millisecond):
			leaked <- true
		}
		if ok := <-leaked; ok {
			t.Fatal("goroutine leak")
		}
	})
	t.Run("wait-stop-order", func(t *testing.T) {
		w := newWatcher()
		// can Stop can be run before Wait and have Wait work correctly
		w.Stop()
		errCh := make(chan error)
		go func() {
			errCh <- w.Wait(context.Background())
		}()
		bad_stop := make(chan bool, 1)
		defer close(bad_stop)
		<-w.waitingCh
		select {
		case <-errCh:
			bad_stop <- true
		case <-time.After(time.Millisecond):
			bad_stop <- false
		}
		if ok := <-bad_stop; ok {
			t.Fatal("Stop->Wait shouldn't stop Wait")
		}
		w.Stop()
	})
}

func TestWatcherNotify(t *testing.T) {
	t.Run("single-notify-true", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		n := fakeNotifier("foo")
		w.Register(n)
		w.dataCh <- w.track(n, foodep)
		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()
		if err := w.Wait(ctx); err != nil {
			t.Fatalf("wait should have returned nil, got: %v\n", err)
		}
	})
	t.Run("single-notify-false", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		n := fakeNotifier("foo")
		w.Register(n)
		w.dataCh <- w.track(n, foodep)
		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()
		n.notify = false
		if err := w.Wait(ctx); err != context.Canceled {
			t.Fatalf("wait should have returned context.Canceled, got: %v", err)
		}
	})
	t.Run("multi-notify-true", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		bardep := &idep.FakeDep{Name: "bar"}
		n := fakeNotifier("foo")
		w.Register(n)
		w.dataCh <- w.track(n, foodep)
		w.dataCh <- w.track(n, bardep)
		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()
		if err := w.Wait(ctx); err != nil {
			t.Fatalf("wait should have returned nil, got: %v\n", err)
		}
	})
	t.Run("multi-notify-false", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		bardep := &idep.FakeDep{Name: "bar"}
		n := fakeNotifier("foo")
		w.Register(n)
		n.notify = false
		w.dataCh <- w.track(n, foodep)
		w.dataCh <- w.track(n, bardep)
		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()
		if err := w.Wait(ctx); err != context.Canceled {
			t.Fatalf("wait should have returned context.Canceled, got: %v", err)
		}
	})
	t.Run("notify-true-then-false", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		nf := fakeNotifier("foo")
		bardep := &idep.FakeDep{Name: "bar"}
		nb := fakeNotifier("bar")
		w.Register(nf, nb)
		nb.notify = false
		w.dataCh <- w.track(nf, foodep)
		w.dataCh <- w.track(nb, bardep)
		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()
		if err := w.Wait(ctx); err != nil {
			t.Fatalf("wait should have returned nil, got: %v\n", err)
		}
	})
	t.Run("notify-false-then-true", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		nf := fakeNotifier("foo")
		nf.notify = false
		bardep := &idep.FakeDep{Name: "bar"}
		nb := fakeNotifier("bar")
		w.Register(nf, nb)
		w.dataCh <- w.track(nf, foodep)
		w.dataCh <- w.track(nb, bardep)

		ctx, cc := context.WithCancel(context.Background())
		go func() { time.Sleep(time.Millisecond); cc() }()

		if err := w.Wait(ctx); err != nil {
			t.Fatalf("wait should have returned nil, got: %v\n", err)
		}
	})
	t.Run("notify-assert", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		foodep := &idep.FakeDep{Name: "foo"}
		bardep := &idep.FakeListDep{Name: "bar"}
		n := fakeNotifier("foo")
		w.Register(n)
		fooview := w.track(n, foodep)
		fooview.store("foo")
		barview := w.track(n, bardep)
		barview.store([]string{"bar", "zed"})
		w.dataCh <- fooview
		w.dataCh <- barview

		w.Wait(context.Background())
		for i := 0; i < 2; i++ {
			d := <-n.deps
			switch d.(type) {
			case string, []string:
			default:
				t.Fatalf("Bad type of test data: %T\n", d)
			}
		}
	})
}

func TestWatcherMarkSweep(t *testing.T) {
	t.Run("remove-old-dependency", func(t *testing.T) {
		w := newWatcher()
		defer w.Stop()
		fdep := &idep.FakeDep{Name: "foo"}
		bdep := &idep.FakeDep{Name: "bar"}
		n := fakeNotifier("zed")
		w.Register(n)
		w.track(n, fdep).store(fdep.Name)
		w.track(n, bdep).store(bdep.Name)
		w.cache.Save(fdep.ID(), fdep.Name)
		w.cache.Save(bdep.ID(), bdep.Name)

		// checks that dependencies are watched and have active views
		checkDeps := func(deps ...*idep.FakeDep) {
			t.Helper() // fixes line numbers
			for _, d := range deps {
				if !w.Watching(d.ID()) {
					t.Errorf("expected dependency to be present (%s)", d)
				}
				if v := w.view(d.ID()); v == nil {
					t.Errorf("expected dependency '%v' to be present", d)
				}
				if _, found := w.cache.Recall(d.ID()); !found {
					t.Errorf("expected to find cache for '%v'", d.ID())
				}
			}
		}
		// everything watched
		checkDeps(fdep, bdep)
		// marks all dependencies of this notifier as being unused
		w.Mark(n)
		// everything still watched
		checkDeps(fdep, bdep)

		// simulate recaller calling register
		w.track(n, fdep)

		// everything still here
		checkDeps(fdep, bdep)

		// should delete dep that did *not* register (bdep)
		w.Sweep(n)
		// fdep was registered, should still be present
		checkDeps(fdep)
		// bdep was un-registered, should be gone
		if w.Watching(bdep.ID()) {
			t.Error("expected dependency bar to no longer be watched")
		}
		if v := w.view(bdep.ID()); v != nil {
			t.Error("expected dependency bar to be removed")
		}
		if _, found := w.cache.Recall(bdep.ID()); found {
			t.Errorf("expected *no* cache for '%v'", bdep.ID())
		}
	})
}

func newWatcher() *Watcher {
	return NewWatcher(WatcherInput{
		Clients: NewClientSet(),
		Cache:   NewStore(),
	})
}
