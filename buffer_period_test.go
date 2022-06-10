package hcat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufferPeriod(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		min            time.Duration
		max            time.Duration
		snoozeCount    int64
		snoozeAfter    time.Duration
		expectedWithin time.Duration
	}{
		{
			name: "one period",
			min:  time.Duration(2 * time.Millisecond),
			max:  time.Duration(10 * time.Millisecond),
		},
		{
			name:        "snooze once",
			min:         time.Duration(4 * time.Millisecond),
			max:         time.Duration(12 * time.Millisecond),
			snoozeCount: 1,
			snoozeAfter: time.Duration(1 * time.Millisecond),
		},
		{
			name:        "deadline",
			min:         time.Duration(4 * time.Millisecond),
			max:         time.Duration(6 * time.Millisecond),
			snoozeCount: 3,
			snoozeAfter: time.Duration(1 * time.Millisecond),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tc := testCase
			t.Parallel()

			triggerCh := make(chan string)
			bufferPeriods := newTimers()
			go bufferPeriods.Run(triggerCh)
			defer bufferPeriods.Stop()

			bufferPeriods.Add(tc.min, tc.max, tc.name)
			assert.False(t, bufferPeriods.Buffered(tc.name),
				"buffer isn't activated yet to be buffered")

			bufferPeriods.tick(tc.name)

			// Simulate consecutive calls to resolver.Run(template)
			go func() {
				for i := int64(0); i < tc.snoozeCount; i++ {
					<-time.After(tc.snoozeAfter)
					bufferPeriods.tick(tc.name)
				}
			}()

			// Test signal is received within expected duration
			expectedWithin := tc.min + time.Duration(tc.snoozeCount)*tc.snoozeAfter
			expectedWithin += 3 * time.Millisecond // add a bit of leniency
			select {
			case id := <-triggerCh:
				assert.Equal(t, tc.name, id, "unexpected id")
			case <-time.After(expectedWithin):
				assert.Fail(t, "buffer did not complete within expected period", expectedWithin)
			}

			assert.True(t, bufferPeriods.Buffered(tc.name), "id should be cached as buffered")
		})
	}

	t.Run("not configured", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		first := time.Duration(2 * time.Millisecond)
		second := time.Duration(4 * time.Millisecond)
		bufferPeriods.Add(first, first*2, "first")
		bufferPeriods.Add(second, second*2, "second")

		bufferPeriods.tick("first")
		bufferPeriods.tick("second")

		completed := make(chan struct{})
		go func() {
			assert.Equal(t, "first", <-triggerCh)
			assert.Equal(t, "second", <-triggerCh)
			completed <- struct{}{}
		}()

		select {
		case <-time.After(5900 * time.Microsecond):
			assert.Fail(t, "expected both buffer periods to send a signal")
		case <-completed:
		}
	})

	t.Run("stop unused timers", func(t *testing.T) {
		t.Parallel()

		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)

		bufferPeriods.Add(time.Millisecond, 2*time.Millisecond, "unused")
		bufferPeriods.Stop()
	})

	t.Run("reset", func(t *testing.T) {
		t.Parallel()

		triggerCh := make(chan string, 5)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		id := "foo"
		bufferPeriods.Add(2*time.Millisecond, 8*time.Millisecond, id)
		bufferPeriods.tick(id) // activate buffer
		assert.True(t, bufferPeriods.timers[id].active())

		bufferPeriods.Reset(id)

		select {
		case <-triggerCh:
			t.Fatalf("buffer was reset and should not have triggered")
		case <-time.After(2 * time.Millisecond):
			assert.False(t, bufferPeriods.timers[id].active())
		}
	})
}
