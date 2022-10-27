package hcat

import (
	"context"
	"sync"
	"time"
)

// timers is a threadsafe object to manage multiple timers that represent
// buffer periods for objects by their ID.
type timers struct {
	mux sync.RWMutex
	// timers is the map of IDs to their active buffer timers.
	timers   map[string]*timer
	buffered map[string]bool
	ch       chan string
}

// timer is an internal representation of a single buffer state.
type timer struct {
	mux sync.RWMutex

	id string
	ch chan string

	deadline time.Time
	min      time.Duration
	max      time.Duration

	timer      timerer
	newTimerer func(d time.Duration) timerer
	cancelTick context.CancelFunc
	isActive   bool
}

// time.Timer interface to allow mocking for testing without races
type timerer interface {
	Reset(time.Duration) bool
	GetC() <-chan time.Time
	Stop() bool
}

func newTimers() *timers {
	return &timers{
		timers:   make(map[string]*timer),
		buffered: make(map[string]bool),
		ch:       make(chan string, 10),
	}
}

// Run is a blocking function to monitor timers and notify the channel
// a buffer period has completed.
func (t *timers) Run(triggerCh chan string) {
	for {
		id, ok := <-t.ch
		if !ok {
			return
		}
		t.mux.Lock()
		t.buffered[id] = true
		t.mux.Unlock()
		triggerCh <- id
	}
}

// Stop sends a signal to halt monitoring of timers and clears out any active
// timers.
func (t *timers) Stop() {
	t.mux.Lock()
	defer t.mux.Unlock()

	for id, timer := range t.timers {
		timer.stop()
		delete(t.timers, id)
	}
}

// Add a new timer and returns if the timer was added.
func (t *timers) Add(min, max time.Duration, id string) bool {
	t.mux.Lock()
	defer t.mux.Unlock()

	if t.timers[id] != nil {
		return false
	}

	t.timers[id] = newTimer(t.ch, min, max, id)
	return true
}

// Buffered checks the cache of recently expired timers if the timer is done
// buffering. (used in testing)
func (t *timers) Buffered(id string) bool {
	t.mux.RLock()
	defer t.mux.RUnlock()
	return t.buffered[id]
}

// isBuffering tests whether buffing is currently in use
func (t *timers) isBuffering(id string) bool {
	_, ok := t.timers[id]
	return ok
}

// tick activates the buffer period and updates the timer.
// Returns false if no timer is found.
func (t *timers) tick(id string) bool {
	return t._tick(id, time.Now())
}

// tick with 'now' passed in to allow testing
func (t *timers) _tick(id string, now time.Time) bool {
	t.mux.Lock()
	defer t.mux.Unlock()

	timer, ok := t.timers[id]
	if ok {
		timer.tick(now)
	}
	return ok
}

// Reset resets an active timer
func (t *timers) Reset(id string) {
	t.mux.Lock()
	defer t.mux.Unlock()

	if timer, ok := t.timers[id]; ok {
		timer.reset()
		delete(t.buffered, id)
	}
}

// add timer using test version of time.Timer
func (t *timers) testAdd(min, max time.Duration, id string) bool {
	ok := t.Add(min, max, id)
	if ok {
		t.timers[id].newTimerer = NewTestTimer
	}
	return ok
}

// returns the timer for id
func (t *timers) get(id string) *timer {
	t.mux.Lock()
	defer t.mux.Unlock()

	if timer, ok := t.timers[id]; ok {
		return timer
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////
// newTimer creates a new buffer timer for the given template.
func newTimer(ch chan string, min, max time.Duration, id string) *timer {
	return &timer{
		id:  id,
		min: min,
		max: max,
		ch:  ch,
		// change to use test timer in tests
		newTimerer: NewRealTimer,
	}
}

func (t *timer) stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
}

// tick updates the minimum buffer timer
func (t *timer) tick(now time.Time) {
	if t.active() {
		t.activeTick(now)
		return
	}

	t.inactiveTick(now)
}

func (t *timer) active() bool {
	t.mux.RLock()
	defer t.mux.RUnlock()
	return t.isActive
}

// inactiveTick is the first tick of a buffer period, set up the timer and
// calculate the max deadline.
func (t *timer) inactiveTick(now time.Time) {
	if t.timer == nil {
		t.timer = t.newTimerer(t.min)
	} else {
		t.timer.Reset(t.min)
	}
	ctx, cancel := context.WithCancel(context.Background())

	t.mux.Lock()
	t.isActive = true
	t.deadline = now.Add(t.max) // reset the deadline ot the future
	t.cancelTick = cancel
	t.mux.Unlock()

	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		case <-t.timer.GetC():
			t.mux.Lock()
			t.ch <- t.id
			t.isActive = false
			t.mux.Unlock()
		}
	}(ctx)
}

// activeTick snoozes the timer for the min time, or snooze less if we are coming
// up against the max time.
func (t *timer) activeTick(now time.Time) {
	// Wait for the lock in case the go routine is updating the active state.
	// If the timer has already fired don't snooze and let the next tick reset
	// the buffer period to active
	t.mux.Lock()
	defer t.mux.Unlock()
	if !t.isActive {
		return
	}

	if now.Add(t.min).Before(t.deadline) {
		t.timer.Reset(t.min)
	} else if dur := t.deadline.Sub(now); dur > 0 {
		t.timer.Reset(dur)
	}
}

// reset resets the timer to inactive
func (t *timer) reset() {
	t.mux.Lock()
	defer t.mux.Unlock()
	if !t.isActive {
		return
	}

	t.cancelTick()
	t.isActive = false
}

// //////////////////////////////////////////////////////////////////////
// time.Timer wrapper and a mocked/test Timer implementation
// They both meet the `timerer` interface above

// time.Timer wrapped to fit the interface (needed a getter for the channel)
type realTimer struct {
	time.Timer
}

func NewRealTimer(d time.Duration) timerer {
	return &realTimer{*time.NewTimer(d)}
}

func (tt *realTimer) GetC() <-chan time.Time {
	return tt.C
}

// time.Timer for testing where it totals up the timer time for comparison
type testTimer struct {
	C         chan time.Time
	totalTime time.Duration
}

// testTimer
func NewTestTimer(d time.Duration) timerer {
	C := make(chan time.Time)
	return &testTimer{C: C, totalTime: d}
}

func (tt *testTimer) GetC() <-chan time.Time {
	return tt.C
}

func (tt *testTimer) Reset(d time.Duration) bool {
	tt.totalTime += d
	return true
}

func (tt *testTimer) Stop() bool {
	return true
}

func (tt *testTimer) send() {
	tt.C <- time.Now()
}
