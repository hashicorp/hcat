package hcat

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/hcat/events"
	dep "github.com/hashicorp/hcat/internal/dependency"
)

func TestPoll_returnsViewCh(t *testing.T) {
	vw := newView(&newViewInput{
		Dependency: &dep.FakeDep{},
	})

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	defer vw.stop()

	select {
	case <-viewCh:
		// Got this far, so the test passes
	case err := <-errCh:
		t.Errorf("error while polling: %s", err)
	case <-vw.stopCh:
		t.Errorf("poll received premature stop")
	}
}

func TestPoll_returnsErrCh(t *testing.T) {
	vw := newView(&newViewInput{
		Dependency: &dep.FakeDepFetchError{},
	})
	vw.lastIndex = 15 // test that it will be reset to 0

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	defer vw.stop()

	select {
	case data := <-viewCh:
		t.Errorf("expected no data, but got %+v", data)
	case err := <-errCh:
		expected := "failed to contact server: connection refused"
		if err.Error() != expected {
			t.Errorf("expected %q to be %q", err.Error(), expected)
		}
		if vw.lastIndex != 0 {
			t.Errorf("expected last index to be 0 but %q", vw.lastIndex)
		}
	case <-vw.stopCh:
		t.Errorf("poll received premature stop")
	}
}

func TestPoll_stopsViewStopCh(t *testing.T) {
	vw := newView(&newViewInput{
		Dependency: &dep.FakeDep{},
	})

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	vw.stop()

	select {
	case <-viewCh:
		t.Errorf("expected no data, but received view data")
	case err := <-errCh:
		t.Errorf("error while polling: %s", err)
	case <-time.After(20 * time.Millisecond):
		// No data was received, test passes
	}
}

func TestPoll_retries(t *testing.T) {
	vw := newView(&newViewInput{
		Dependency: &dep.FakeDepRetry{},
		RetryFunc: func(retry int) (bool, time.Duration) {
			return retry < 1, 250 * time.Millisecond
		},
	})

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	defer vw.stop()

	select {
	case <-viewCh:
		t.Errorf("should not have gotten data yet")
	case <-time.After(100 * time.Millisecond):
	}

	select {
	case <-viewCh:
		// Got this far, so the test passes
	case err := <-errCh:
		t.Errorf("error while polling: %s", err)
	case <-vw.stopCh:
		t.Errorf("poll received premature stop")
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestFetch_resetRetries(t *testing.T) {
	view := newView(&newViewInput{
		Dependency: &dep.FakeDepSameIndex{},
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go view.fetch(doneCh, successCh, errCh)

	select {
	case <-successCh:
	case <-doneCh:
		t.Error("should not be done")
	case err := <-errCh:
		t.Errorf("error while fetching: %s", err)
	}
}

func TestFetch_maxStale(t *testing.T) {
	view := newView(&newViewInput{
		Dependency: &dep.FakeDepStale{},
		MaxStale:   10 * time.Millisecond,
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	go view.fetch(doneCh, successCh, errCh)

	select {
	case <-doneCh:
		expected := "this is some fresh data"
		if !reflect.DeepEqual(view.Data(), expected) {
			t.Errorf("expected %q to be %q", view.Data(), expected)
		}
	case err := <-errCh:
		t.Errorf("error while fetching: %s", err)
	}
}

func TestFetch_savesView(t *testing.T) {
	view := newView(&newViewInput{
		Dependency: &dep.FakeDep{Name: "this is some data"},
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	go view.fetch(doneCh, successCh, errCh)

	select {
	case <-doneCh:
		expected := "this is some data"
		if !reflect.DeepEqual(view.Data(), expected) {
			t.Errorf("expected %q to be %q", view.Data(), expected)
		}
	case err := <-errCh:
		t.Errorf("error while fetching: %s", err)
	}
}

func TestFetch_returnsErrCh(t *testing.T) {
	view := newView(&newViewInput{
		Dependency: &dep.FakeDepFetchError{},
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	go view.fetch(doneCh, successCh, errCh)

	select {
	case <-doneCh:
		t.Errorf("expected error, but received doneCh")
	case err := <-errCh:
		expected := "failed to contact server: connection refused"
		if err.Error() != expected {
			t.Fatalf("expected error %q to be %q", err.Error(), expected)
		}
	}
}

func TestFetch_ctxCancel(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	view := newView(&newViewInput{
		Dependency: &dep.FakeDepBlockingQuery{
			Name:          "ctxCancel",
			BlockDuration: time.Duration(5 * time.Minute),
			Ctx:           ctx,
		},
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		view.fetch(doneCh, successCh, errCh)
		wg.Done()
	}()

	select {
	case <-doneCh:
		t.Errorf("unexpected doneCh")
	case <-successCh:
		t.Errorf("unexpected successCh")
	case <-errCh:
		t.Errorf("unexpected errCh")
	case <-time.After(5 * time.Millisecond):
		// Nothing expected in any of these channels
		ctxCancel()
	}

	// Successfully stopped by context
	wg.Wait()
}

func TestStop_stopsPolling(t *testing.T) {
	vw := newView(&newViewInput{
		Dependency: &dep.FakeDep{},
	})

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	vw.stop()

	select {
	case v := <-viewCh:
		t.Errorf("got unexpected view: %#v", v)
	case err := <-errCh:
		t.Error(err)
	case <-vw.stopCh:
		// Successfully stopped
	}
}

func TestStop_stopsFetchWithCancel(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	view := newView(&newViewInput{
		Dependency: &dep.FakeDepBlockingQuery{
			Name:          "ctxCancel",
			BlockDuration: time.Duration(5 * time.Minute),
			Ctx:           ctx,
		},
	})
	view.ctxCancel = ctxCancel

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		view.fetch(doneCh, successCh, errCh)
		wg.Done()
	}()

	view.stop()

	select {
	case <-doneCh:
		t.Errorf("unexpected doneCh")
	case <-successCh:
		t.Errorf("unexpected successCh")
	case <-errCh:
		t.Errorf("unexpected errCh")
	case <-time.After(5 * time.Millisecond):
		// Nothing expected in any of these channels
	}

	// Successfully stopped by context
	wg.Wait()
}

func TestRateLimiter(t *testing.T) {
	// test for rate limiting delay working
	elapsed := minDelayBetweenUpdates / 2 // simulate time passing
	start := time.Now().Add(-elapsed)     // add negative to subtract
	dur := rateLimiter(start)             // should close to elapsed
	if !(dur > 0) {
		t.Errorf("rate limiting duration should be > 0, found: %v", dur)
	}
	if dur > minDelayBetweenUpdates {
		t.Errorf("rate limiting duration extected to be < %v, found %v",
			minDelayBetweenUpdates, dur)
	}
	// test that you get 0 when enough time is past
	elapsed = minDelayBetweenUpdates // simulate time passing
	start = time.Now().Add(-elapsed) // add negative to subtract
	dur = rateLimiter(start)         // should be 0
	if dur != 0 {
		t.Errorf("rate limiting duration should be 0, found: %v", dur)
	}
}

func TestFetchEvents(t *testing.T) {
	data := "event test data"
	fdep := &dep.FakeDep{Name: data}
	vw := newView(&newViewInput{
		Dependency: fdep,
		EventHandler: func(e events.Event) {
			switch v := e.(type) {
			case events.Trace:
				if v.ID != fdep.ID() {
					t.Errorf("bad ID, wanted: '%v', got '%v'", fdep.ID(), v.ID)
				}
			case events.NewData:
				if v.Data != data {
					t.Errorf("bad data, wanted: '%v', got '%v'", v.Data, data)
				}
			default:
				t.Errorf("bad event: %#v", e)
			}
		},
	})

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	go vw.fetch(doneCh, successCh, errCh)

	select {
	case <-doneCh:
	case err := <-errCh:
		t.Errorf("error while fetching: %s", err)
	}
}

func TestPollingEvents(t *testing.T) {
	data := "event test data"
	fdep := &dep.FakeDep{Name: data}
	vw := newView(&newViewInput{
		Dependency: fdep,
		EventHandler: func(e events.Event) {
			switch v := e.(type) {
			case events.Trace, events.ServerContacted, events.TrackStart:
			case events.TrackStop: // only get this sometimes, race to exit test
			case events.NewData:
				if v.ID != fdep.ID() {
					t.Errorf("bad ID, wanted: '%v', got '%v'", fdep.ID(), v.ID)
				}
				if v.Data != data {
					t.Errorf("bad data, wanted: '%v', got '%v'", v.Data, data)
				}
			default:
				t.Errorf("bad event: %#v", e)
			}
		},
	})

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	defer vw.stop()

	select {
	case <-viewCh:
	case err := <-errCh:
		t.Error(err)
	case <-vw.stopCh:
		t.Errorf("got unexpected stop")
	}
}
