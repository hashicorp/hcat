package events

import (
	"testing"
)

var (
	_ Event = (*Trace)(nil)
	_ Event = (*BlockingWait)(nil)
	_ Event = (*ServerContacted)(nil)
	_ Event = (*ServerError)(nil)
	_ Event = (*ServerTimeout)(nil)
	_ Event = (*RetryAttempt)(nil)
	_ Event = (*MaxRetries)(nil)
	_ Event = (*NewData)(nil)
	_ Event = (*StaleData)(nil)
	_ Event = (*NoNewData)(nil)
	_ Event = (*TrackStart)(nil)
	_ Event = (*TrackStop)(nil)
	_ Event = (*PollingWait)(nil)
)

func TestEvents(t *testing.T) {
	var event EventHandler
	event = func(e Event) {
		switch e.(type) {
		case Trace, BlockingWait, ServerContacted, ServerError,
			ServerTimeout, RetryAttempt, MaxRetries, NewData, StaleData,
			NoNewData, TrackStart, TrackStop, PollingWait:
		default:
			t.Errorf("Bad event type: %T", e)
		}
	}
	event(Trace{})
	event(MaxRetries{})
	event(TrackStop{})
}
