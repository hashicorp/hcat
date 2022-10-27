package hcat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestTimer(ts *timers, name string) *testTimer {
	timer, ok := ts.get(name).timer.(*testTimer)
	if ok {
		return timer
	}
	panic("should be *testTimer")
}

func TestBufferPeriod(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		min         time.Duration
		max         time.Duration
		snoozeCount int64
		expected    time.Duration // usually min + (snoozeCount * min)
	}{
		{
			name:     "no buffering",
			min:      time.Duration(2 * time.Millisecond),
			max:      time.Duration(10 * time.Millisecond),
			expected: time.Duration(2 * time.Millisecond),
		},
		{
			name:        "snooze one",
			min:         time.Duration(4 * time.Millisecond),
			max:         time.Duration(12 * time.Millisecond),
			snoozeCount: 1,
			expected:    time.Duration(8 * time.Millisecond),
		},
		{
			name:        "snooze many",
			min:         time.Duration(4 * time.Millisecond),
			max:         time.Duration(21 * time.Millisecond),
			snoozeCount: 3,
			expected:    time.Duration(16 * time.Millisecond),
		},
		{
			name:        "deadline",
			min:         time.Duration(4 * time.Millisecond),
			max:         time.Duration(6 * time.Millisecond),
			snoozeCount: 3,
			expected:    time.Duration(6 * time.Millisecond),
			// this is 6 as activeTick() is a noop if past the deadline
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tc := testCase

			triggerCh := make(chan string)
			bufferPeriods := newTimers()
			go bufferPeriods.Run(triggerCh)
			defer bufferPeriods.Stop()

			bufferPeriods.testAdd(tc.min, tc.max, tc.name)
			assert.False(t, bufferPeriods.Buffered(tc.name),
				"buffer isn't activated yet to be buffered")

			// manually run tick, 1 + snoozeCount
			// use static 'now' for testing
			now := time.Now()
			bufferPeriods._tick(tc.name, now)
			now = now.Add(tc.min) // fake time passes..

			for i := int64(0); i < tc.snoozeCount; i++ {
				bufferPeriods._tick(tc.name, now)
				now = now.Add(tc.min) // fake time passes..
			}

			// pull out the test timer for examination
			timer := getTestTimer(bufferPeriods, tc.name)

			// testTimer adds up ticks (time.Reset) into totalTime
			if timer.totalTime != tc.expected {
				t.Errorf("buffer time (%s) doesn't match expected (%s)",
					timer.totalTime, tc.expected)
			}

			timer.send()      // fake a time.Timer expiring
			id := <-triggerCh // previous send() should enable this
			if id != tc.name {
				t.Errorf("id (%s) should match name (%s)", id, tc.name)
			}
			// Test signal is received within expected duration
			assert.True(t, bufferPeriods.Buffered(tc.name), "id should be cached as buffered")
		})
	}

	t.Run("not configured", func(t *testing.T) {
		triggerCh := make(chan string)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		isBuffering := bufferPeriods.isBuffering("dne")
		assert.False(t, isBuffering, "buffer not configured, should not be buffering")

		select {
		case id := <-triggerCh:
			assert.Fail(t, "unexpected ID when no buffer period was added", id)
		case <-time.After(time.Millisecond):
			// test passes
		}
	})

	t.Run("multiple", func(t *testing.T) {
		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		first := time.Duration(3 * time.Millisecond)
		second := time.Duration(6 * time.Millisecond)
		bufferPeriods.testAdd(first, first*3, "first")
		bufferPeriods.testAdd(second, second*3, "second")

		bufferPeriods.tick("first")
		bufferPeriods.tick("second")

		if tmr := getTestTimer(bufferPeriods, "first"); tmr.totalTime != first {
			t.Error("first tick times don't match")
		} else { // times match, so send a time down the Timer channel
			tmr.send()
		}

		if tmr := getTestTimer(bufferPeriods, "second"); tmr.totalTime != second {
			t.Error("second tick times don't match")
		} else { // times match, so send a time down the Timer channel
			tmr.send()
		}

		completed := make(chan struct{})
		go func() {
			assert.Equal(t, "first", <-triggerCh)
			assert.Equal(t, "second", <-triggerCh)
			completed <- struct{}{}
		}()

		_, ok := bufferPeriods.get("first").timer.(*testTimer)
		if !ok {
			t.Error("not using fake timer")
		}

		select {
		case <-time.After(8800 * time.Microsecond):
			assert.Fail(t, "expected both buffer periods to send a signal")
		case <-completed:
		}
	})

	t.Run("stop unused timers", func(t *testing.T) {
		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)

		bufferPeriods.testAdd(time.Millisecond, 2*time.Millisecond, "unused")
		if _, ok := bufferPeriods.timers["unused"]; !ok {
			t.Error("timers entry should exist")
		}
		bufferPeriods.Stop()
		if _, ok := bufferPeriods.timers["unused"]; ok {
			t.Error("timers entry should *not* exist")
		}
	})

	t.Run("reset", func(t *testing.T) {
		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		id := "foo"
		bufferPeriods.testAdd(2*time.Millisecond, 8*time.Millisecond, id)
		bufferPeriods.tick(id) // activate buffer
		assert.True(t, bufferPeriods.timers[id].active())
		bufferPeriods.Reset(id)
		assert.False(t, bufferPeriods.timers[id].active())
	})
}
