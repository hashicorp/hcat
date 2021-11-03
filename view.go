package hcat

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/hcat/events"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

// Temporarily raise these types to the top level via aliasing.
// This is to address a bug in the short term and this should be refactored
// when thinking of how to modularlize the dependencies.
type QueryOptionsSetter = idep.QueryOptionsSetter
type QueryOptions = idep.QueryOptions

// view is a representation of a Dependency and the most recent data it has
// received from Consul.
type view struct {
	// dependency is the dependency that is associated with this view
	dependency dep.Dependency

	// clients is the list of clients to communicate upstream. This is passed
	// directly to the dependency.
	clients Looker

	// event holds the callback for event processing
	event events.EventHandler

	// data is the most-recently-received data from Consul for this view. It is
	// accompanied by a series of locks and booleans to ensure consistency.
	dataLock     sync.RWMutex
	data         interface{}
	receivedData bool
	lastIndex    uint64

	// flag to denote that polling is active
	isPolling bool

	// blockWaitTime is amount of time in seconds to do a blocking query for
	blockWaitTime time.Duration

	// maxStale is the maximum amount of time to allow a query to be stale.
	maxStale time.Duration

	// defaultLease is used for non-renewable leases when secret has no lease
	defaultLease time.Duration

	// retryFunc is the function to invoke on failure to determine if a retry
	// should be attempted.
	retryFunc RetryFunc

	// stopCh is used to stop polling on this view
	stopCh chan struct{}

	// Each view has a context used to cancel an in-flight HTTP request. This is
	// a no-op if there is not an active request. Canceling is required to release
	// the underlying TCP connections used by Consul blocking queries that are
	// waiting for changes from the server. It allows for the http.Transport to
	// change the TCP connection status from active to idle, to then be reaped.
	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewViewInput is used as input to the NewView function.
type newViewInput struct {
	// Dependency is the dependency to associate with the new view.
	Dependency dep.Dependency

	// Clients is the list of clients to communicate upstream. This is passed
	// directly to the dependency.
	Clients Looker

	// EventHandler takes the callback for event processing
	EventHandler events.EventHandler

	// BlockWaitTime is amount of time in seconds to do a blocking query for
	BlockWaitTime time.Duration

	// MaxStale is the maximum amount a time a query response is allowed to be
	// stale before forcing a read from the leader.
	MaxStale time.Duration

	// RetryFunc is a function which dictates how this view should retry on
	// upstream errors.
	RetryFunc RetryFunc
}

// NewView constructs a new view with the given inputs.
func newView(i *newViewInput) *view {
	ctx, cancel := context.WithCancel(context.Background())
	eventHandler := i.EventHandler
	if eventHandler == nil {
		eventHandler = func(events.Event) {}
	}
	return &view{
		dependency:    i.Dependency,
		clients:       i.Clients,
		event:         eventHandler,
		blockWaitTime: i.BlockWaitTime,
		maxStale:      i.MaxStale,
		retryFunc:     i.RetryFunc,
		stopCh:        make(chan struct{}, 1),
		ctx:           ctx,
		ctxCancel:     cancel,
	}
}

// Dependency returns the dependency attached to this view.
func (v *view) Dependency() dep.Dependency {
	return v.dependency
}

// Data returns the most-recently-received data from Consul for this view.
func (v *view) Data() interface{} {
	v.dataLock.RLock()
	defer v.dataLock.RUnlock()
	return v.data
}

// DataAndLastIndex returns the most-recently-received data from Consul for
// this view, along with the last index. This is atomic so you will get the
// index that goes with the data you are fetching.
func (v *view) DataAndLastIndex() (interface{}, uint64) {
	v.dataLock.RLock()
	defer v.dataLock.RUnlock()
	return v.data, v.lastIndex
}

// ID outputs a unique string identifier for the view
// It is identical to it's contained Dependency ID.
func (v *view) ID() string {
	return v.dependency.ID()
}

// pollingFlag handles setting and clearing the flag to indicate active polling
// Returned function needs to be called (usually w/ defer) to clear the flag.
func (v *view) pollingFlag() (alreadyPolling bool, unflag func()) {
	v.dataLock.Lock()
	defer v.dataLock.Unlock()

	if v.isPolling {
		return true, func() {}
	}

	v.isPolling = true
	return false, func() {
		v.dataLock.Lock()
		defer v.dataLock.Unlock()
		v.isPolling = false
	}
}

// poll queries the Consul instance for data using the fetch function, but also
// accounts for interrupts on the interrupt channel. This allows the poll
// function to be fired in a goroutine, but then halted even if the fetch
// function is in the middle of a blocking query.
func (v *view) poll(viewCh chan<- *view, errCh chan<- error) {
	var retries int
	v.event(events.TrackStart{ID: v.ID()})

	alreadyPolling, stoppedPolling := v.pollingFlag()
	if alreadyPolling {
		return
	}
	defer func() {
		stoppedPolling()
		v.event(events.TrackStop{ID: v.ID()})
	}()

	for {
		doneCh := make(chan struct{}, 1)
		successCh := make(chan struct{}, 1)
		fetchErrCh := make(chan error, 1)
		go v.fetch(doneCh, successCh, fetchErrCh)

	WAIT:
		select {
		case <-doneCh:
			// Reset the retry to avoid exponentially incrementing retries when we
			// have some successful requests
			retries = 0

			select {
			case <-v.stopCh:
				return
			case viewCh <- v:
			}

		case <-successCh:
			// We successfully received a non-error response from the server.
			// This does not mean we have data (that's dataCh's job), but
			// rather this just resets the counter indicating we communicated
			// successfully. For example, Consul make have an outage, but when
			// it returns, the view is unchanged. We have to reset the counter
			// retries, but not update the actual template.
			v.event(events.ServerContacted{ID: v.ID()})
			retries = 0
			goto WAIT
		case err := <-fetchErrCh:
			v.event(events.ServerError{ID: v.ID(), Error: err})
			var skipRetry bool
			if strings.Contains(err.Error(), "Unexpected response code: 400") {
				// 400 is not useful to retry
				skipRetry = true
			}

			if strings.Contains(err.Error(), "connection refused") {
				// This indicates that Consul may have restarted. If Consul
				// restarted, the current lastIndex will be stale and cause the
				// next blocking query to hang until the wait time expires. To
				// be safe, reset the lastIndex=0 so that the next query will not
				// block and retrieve the latest lastIndex
				v.dataLock.Lock()
				v.lastIndex = 0
				v.dataLock.Unlock()
			}

			if v.retryFunc != nil && !skipRetry {
				retry, sleep := v.retryFunc(retries)
				if retry {
					v.event(events.RetryAttempt{
						ID:      v.ID(),
						Attempt: retries + 1,
						Sleep:   sleep,
						Error:   err,
					})
					select {
					case <-time.After(sleep):
						retries++
						continue
					case <-v.stopCh:
						return
					}
				}
				v.event(events.MaxRetries{ID: v.ID(), Count: retries})
			}

			// Push the error back up to the watcher
			select {
			case <-v.stopCh:
				return
			case errCh <- err:
				return
			}
		case <-v.stopCh:
			return
		}
	}
}

// fetch queries the Consul instance for the attached dependency. This API
// promises that either data will be written to doneCh or an error will be
// written to errCh. It is designed to be run in a goroutine that selects the
// result of doneCh and errCh. It is assumed that only one instance of fetch
// is running per view and therefore no locking or mutexes are used.
func (v *view) fetch(doneCh, successCh chan<- struct{}, errCh chan<- error) {
	v.event(events.Trace{ID: v.ID(), Message: "starting fetch"})

	var allowStale bool
	if v.maxStale != 0 {
		allowStale = true
	}

	for {
		// If the view was stopped, short-circuit this loop. This prevents a bug
		// where a view can get "lost" in the event Consul Template is reloaded.
		select {
		case <-v.stopCh:
			return
		case <-v.ctx.Done():
			return
		default:
		}

		start := time.Now() // for rateLimiter below

		if d, ok := v.dependency.(QueryOptionsSetter); ok {
			opts := QueryOptions{
				AllowStale:   allowStale,
				WaitTime:     v.blockWaitTime,
				WaitIndex:    v.lastIndex,
				DefaultLease: v.defaultLease,
			}
			opts = opts.SetContext(v.ctx)
			d.SetOptions(opts)
		}
		v.event(events.Trace{ID: v.ID(), Message: "fetching value"})
		data, rm, err := v.dependency.Fetch(v.clients)
		if err != nil {
			switch {
			case err == dep.ErrStopped:
				v.event(events.Trace{ID: v.ID(), Message: err.Error()})
			case strings.Contains(err.Error(), context.Canceled.Error()):
				// This is a wrapped error so relying on string matching
				v.event(events.Trace{ID: v.ID(), Message: err.Error()})
			default:
				errCh <- err
			}
			return
		}

		if rm == nil {
			errCh <- fmt.Errorf("received nil response metadata - this is a bug " +
				"and should be reported")
			return
		}

		// If we got this far, we received data successfully. That data might not
		// trigger a data update (because we could continue below), but we need to
		// inform the poller to reset the retry count.
		v.event(events.Trace{ID: v.ID(), Message: "successful data response"})
		select {
		case successCh <- struct{}{}:
		default:
		}

		if allowStale && rm.LastContact > v.maxStale {
			allowStale = false
			v.event(events.StaleData{ID: v.ID(), LastContant: rm.LastContact})
			continue
		}

		if v.maxStale != 0 {
			allowStale = true
		}

		if dur := rateLimiter(start); dur > 1 {
			time.Sleep(dur)
		}

		if rm.LastIndex == v.lastIndex {
			v.event(events.Trace{ID: v.ID(), Message: "same index, no new data"})
			continue
		}

		v.dataLock.Lock()
		if rm.LastIndex < v.lastIndex {
			v.event(events.Trace{ID: v.ID(),
				Message: "wrong index order, resetting"})
			v.lastIndex = 0
			v.dataLock.Unlock()
			continue
		}
		v.lastIndex = rm.LastIndex

		if v.receivedData && reflect.DeepEqual(data, v.data) {
			v.event(events.NoNewData{ID: v.ID()})
			v.dataLock.Unlock()
			continue
		}

		if _, ok := v.dependency.(idep.BlockingQuery); ok && data == nil {
			v.event(events.BlockingWait{ID: v.ID()})
			v.dataLock.Unlock()
			continue
		}
		v.dataLock.Unlock()

		v.event(events.NewData{ID: v.ID(), Data: data})
		v.store(data)

		close(doneCh)
		return
	}
}

// Store-s the data and marks that it was received
func (v *view) store(data interface{}) {
	v.dataLock.Lock()
	defer v.dataLock.Unlock()
	v.data = data
	if !v.receivedData {
		v.receivedData = true
	}
}

const minDelayBetweenUpdates = time.Millisecond * 100

// return a duration to sleep to limit the frequency of upstream calls
func rateLimiter(start time.Time) time.Duration {
	remaining := minDelayBetweenUpdates - time.Since(start)
	if remaining > 0 {
		dither := time.Duration(rand.Int63n(20000000)) // 0-20ms
		return remaining + dither
	}
	return 0
}

// stop halts polling of this view.
func (v *view) stop() {
	v.dependency.Stop()
	close(v.stopCh)
	v.ctxCancel()
}
