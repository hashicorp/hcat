// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	event
	ID      string
	Message string
}

// BlockingWait means a blocking query was made
type BlockingWait struct {
	event
	ID string
}

// ServerContacted indicates that the tracked service has been successfully
// contacted (received a non-error response).
type ServerContacted struct {
	event
	ID string
}

// ServerError indicates that an tracked service has been contacted but with
// an error returned.
type ServerError struct {
	event
	Error error
	ID    string
}

// ServerTimeout indicates that a call to the server timed out.
type ServerTimeout struct {
	event
	ID string
}

// RetryAttempt indicates that a tracked call is being retried.
type RetryAttempt struct {
	event
	Error   error
	ID      string
	Attempt int
	Sleep   time.Duration
}

// MaxRetries indicates that the maximum number of retries has been reached
// (and failed).
type MaxRetries struct {
	event
	ID    string
	Count int
}

// NewData indicates that fresh/new data has been retrieved from the service.
type NewData struct {
	event
	Data interface{}
	ID   string
}

// StaleData indicates that the service returned stale (possibly old) data.
type StaleData struct {
	event
	ID          string
	LastContant time.Duration
}

// NoNewData indicates that data was retrieved from the service, but that it
// matches the current data so no change would be triggered.
type NoNewData struct {
	event
	ID string
}

// TrackStart indicates that a new data point is being tracked.
type TrackStart struct {
	event
	ID string
}

// TrackStop indicates that a data point is no longer being tracked.
type TrackStop struct {
	event
	ID string
}

// Not used yet, need an PolllingQuery interface to match on
// see BlockingQuery for how it should work
type PollingWait struct {
	event
	ID       string
	Duration time.Duration
}

// Event interface type fulfillment
type event struct{}

func (event) isEvent() {}
