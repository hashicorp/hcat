package events

import "time"

// EventHandler is the interface of the call back function for receiveing events.
type EventHandler func(Event)

// Event is used to type restrict the Events
type Event interface {
	isEvent()
}

// Trace is useful to see some details of what's going on
type Trace struct {
	ID      string
	Message string
	event
}

// BlockingWait means a blocking query was made
type BlockingWait struct {
	ID string
	event
}

// ServerContacted indicates that the tracked service has been successfully
// contacted (received a non-error response).
type ServerContacted struct {
	ID string
	event
}

// ServerError indicates that an tracked service has been contacted but with
// an error returned.
type ServerError struct {
	ID    string
	Error error
	event
}

// ServerTimeout indicates that a call to the server timed out.
type ServerTimeout struct {
	ID string
	event
}

// RetryAttempt indicates that a tracked call is being retried.
type RetryAttempt struct {
	ID      string
	Attempt int
	Sleep   time.Duration
	Error   error
	event
}

// MaxRetries indicates that the maximum number of retries has been reached
// (and failed).
type MaxRetries struct {
	ID    string
	Count int
	event
}

// NewData indicates that fresh/new data has been retrieved from the service.
type NewData struct {
	ID   string
	Data interface{}
	event
}

// StaleData indicates that the service returned stale (possibly old) data.
type StaleData struct {
	ID          string
	LastContant time.Duration
	event
}

// NoNewData indicates that data was retrieved from the service, but that it
// matches the current data so no change would be triggered.
type NoNewData struct {
	ID string
	event
}

// TrackStart indicates that a new data point is being tracked.
type TrackStart struct {
	ID string
	event
}

// TrackStop indicates that a data point is no longer being tracked.
type TrackStop struct {
	ID string
	event
}

// Not used yet, need an PolllingQuery interface to match on
// see BlockingQuery for how it should work
type PollingWait struct {
	ID       string
	Duration time.Duration
	event
}

// Event interface type fulfillment
type event struct{}

func (event) isEvent() {}
