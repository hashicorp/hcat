package hcat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufferPeriod(t *testing.T) {
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
			min:  time.Duration(2 * time.Second),
			max:  time.Duration(10 * time.Second),
		},
		{
			name:        "snooze once",
			min:         time.Duration(4 * time.Second),
			max:         time.Duration(12 * time.Second),
			snoozeCount: 1,
			snoozeAfter: time.Duration(1 * time.Second),
		},
		{
			name:        "deadline",
			min:         time.Duration(4 * time.Second),
			max:         time.Duration(6 * time.Second),
			snoozeCount: 3,
			snoozeAfter: time.Duration(1 * time.Second),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tc := testCase
			t.Parallel()
			expectedWithin := tc.min + time.Duration(tc.snoozeCount)*tc.snoozeAfter + time.Second

			triggerCh := make(chan string)
			bufferPeriods := newTimers()
			go bufferPeriods.Run(triggerCh)
			defer bufferPeriods.Stop()

			bufferPeriods.Add(tc.min, tc.max, tc.name)
			assert.False(t, bufferPeriods.Buffered(tc.name), "buffer isn't activated yet to be buffered")

			active := bufferPeriods.Buffer(tc.name)
			assert.False(t, active, "buffer is unexpectedly already active")

			// Simulate consecutive calls to resolver.Run(template)
			go func() {
				for i := int64(0); i < tc.snoozeCount; i++ {
					<-time.After(tc.snoozeAfter * time.Second)
					bufferPeriods.Buffer(tc.name)
				}
			}()

			// Test signal is received within expected duration
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
		triggerCh := make(chan string)
		bufferPeriods := newTimers()
		go bufferPeriods.Run(triggerCh)
		defer bufferPeriods.Stop()

		isBuffering := bufferPeriods.Buffer("dne")
		assert.False(t, isBuffering, "buffer not configured, should not be buffering")

		select {
		case id := <-triggerCh:
			assert.Fail(t, "unexpected ID when no buffer period was added", id)
		case <-time.After(time.Millisecond):
			// test passes
		}
	})
}
