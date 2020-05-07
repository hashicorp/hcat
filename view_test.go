package hat

import (
	"reflect"
	"testing"
	"time"
)

func TestPoll_returnsViewCh(t *testing.T) {
	vw, err := newView(&newViewInput{
		Dependency: &TestDep{},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	vw, err := newView(&newViewInput{
		Dependency: &TestDepFetchError{},
	})
	if err != nil {
		t.Fatal(err)
	}

	viewCh := make(chan *view)
	errCh := make(chan error)

	go vw.poll(viewCh, errCh)
	defer vw.stop()

	select {
	case data := <-viewCh:
		t.Errorf("expected no data, but got %+v", data)
	case err := <-errCh:
		expected := "failed to contact server"
		if err.Error() != expected {
			t.Errorf("expected %q to be %q", err.Error(), expected)
		}
	case <-vw.stopCh:
		t.Errorf("poll received premature stop")
	}
}

func TestPoll_stopsViewStopCh(t *testing.T) {
	vw, err := newView(&newViewInput{
		Dependency: &TestDep{},
	})
	if err != nil {
		t.Fatal(err)
	}

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

func TestPoll_once(t *testing.T) {
	vw, err := newView(&newViewInput{
		Dependency: &TestDep{},
	})
	if err != nil {
		t.Fatal(err)
	}

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

	select {
	case <-viewCh:
		t.Errorf("expected no data (should have stopped), but received view data")
	case err := <-errCh:
		t.Errorf("error while polling: %s", err)
	case <-vw.stopCh:
		t.Errorf("poll received premature stop")
	case <-time.After(20 * time.Millisecond):
		// No data in 0.2s, so the test passes
	}
}

func TestPoll_retries(t *testing.T) {
	vw, err := newView(&newViewInput{
		Dependency: &TestDepRetry{},
		RetryFunc: func(retry int) (bool, time.Duration) {
			return retry < 1, 250 * time.Millisecond
		},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	view, err := newView(&newViewInput{
		Dependency: &TestDepSameIndex{},
	})
	if err != nil {
		t.Fatal(err)
	}

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

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
	view, err := newView(&newViewInput{
		Dependency: &TestDepStale{},
		MaxStale:   10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

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
	view, err := newView(&newViewInput{
		Dependency: &TestDep{},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	view, err := newView(&newViewInput{
		Dependency: &TestDepFetchError{},
	})
	if err != nil {
		t.Fatal(err)
	}

	doneCh := make(chan struct{})
	successCh := make(chan struct{})
	errCh := make(chan error)

	go view.fetch(doneCh, successCh, errCh)

	select {
	case <-doneCh:
		t.Errorf("expected error, but received doneCh")
	case err := <-errCh:
		expected := "failed to contact server"
		if err.Error() != expected {
			t.Fatalf("expected error %q to be %q", err.Error(), expected)
		}
	}
}

func TestStop_stopsPolling(t *testing.T) {
	vw, err := newView(&newViewInput{
		Dependency: &TestDep{},
	})
	if err != nil {
		t.Fatal(err)
	}

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
