package hcat

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
	"github.com/pkg/errors"
)

// dataBufferSize is the default number of views to process in a batch.
const dataBufferSize = 2048

// RetryFunc defines the function type used to determine how many and how often
// to retry calls to the external services.
type RetryFunc func(int) (bool, time.Duration)

// Cacher defines the interface required by the watcher for caching data
// retreived from external services. It is implemented by Store.
type Cacher interface {
	Save(key string, value interface{})
	Recall(key string) (value interface{}, found bool)
	Delete(key string)
	Reset()
}

// Watcher is a manager for views that poll external sources for data.
type Watcher struct {
	// clients is the collection of API clients to talk to upstreams.
	clients Looker
	// cache stores the data fetched from remote sources
	cache Cacher

	// dataCh is the chan where Views will be published.
	dataCh chan *view
	// errCh is the chan where any errors will be published.
	errCh chan error

	// stopCh is the chan used internally to notify of Stop calls
	stopCh drainableChan
	// waitingCh is used internally to test when Wait is waiting
	waitingCh chan struct{}

	// tracker tracks template<->dependencies (see bottom of this file)
	tracker *tracker

	// bufferTemplates manages the buffer period per template to accumulate
	// dependency changes.
	bufferTemplates *timers
	// bufferTrigger is the notification channel for template IDs that have
	// completed their active buffer period.
	bufferTrigger chan string

	// Consul related
	retryFuncConsul RetryFunc
	// blockWaitTime is how long to block on consul's blocking queries
	blockWaitTime time.Duration
	// maxStale passed to consul to control staleness
	maxStale time.Duration

	// Vault related
	retryFuncVault RetryFunc
	// defaultLease is used for non-renewable leases when secret has no lease
	defaultLease time.Duration
}

type WatcherInput struct {
	// Clients is the client set to communicate with upstreams.
	Clients Looker
	// Cache is the Cacher for caching watched values
	Cache Cacher

	// Optional Vault specific parameters
	// Default non-renewable secret duration
	VaultDefaultLease time.Duration
	// RetryFun for Vault
	VaultRetryFunc RetryFunc

	// Optional Consul specific parameters
	// MaxStale is the max time Consul will return a stale value.
	ConsulMaxStale time.Duration
	// BlockWait is amount of time Consul will block on a query.
	ConsulBlockWait time.Duration
	// RetryFun for Consul
	ConsulRetryFunc RetryFunc
}

type drainableChan chan struct{}

func (s drainableChan) drain() {
	for {
		select {
		case <-s:
		default:
			return
		}
	}
}

// NewWatcher creates a new watcher using the given API client.
func NewWatcher(i WatcherInput) *Watcher {
	cache := i.Cache
	if cache == nil {
		cache = NewStore()
	}
	clients := i.Clients
	if clients == nil {
		clients = NewClientSet()
	}

	bufferTriggerCh := make(chan string, dataBufferSize/2)
	w := &Watcher{
		clients:         clients,
		cache:           cache,
		dataCh:          make(chan *view, dataBufferSize),
		errCh:           make(chan error),
		waitingCh:       make(chan struct{}),
		stopCh:          make(chan struct{}, 1),
		tracker:         newTracker(),
		bufferTrigger:   bufferTriggerCh,
		bufferTemplates: newTimers(),
		retryFuncConsul: i.ConsulRetryFunc,
		maxStale:        i.ConsulMaxStale,
		blockWaitTime:   i.ConsulBlockWait,
		retryFuncVault:  i.VaultRetryFunc,
		defaultLease:    i.VaultDefaultLease,
	}

	go w.bufferTemplates.Run(bufferTriggerCh)

	return w
}

const vaultTokenDummyTemplateID = "dummy.watcher.vault-token.id"

// WatchVaultToken takes a vault token and watches it to keep it updated.
// This is a specialized method as this token can be required without being in
// a template. I hope to generalize this idea so you can watch arbitrary
// dependencies in the future.
func (w *Watcher) WatchVaultToken(token string) error {
	// Start a watcher for the Vault renew if that config was specified
	if token != "" {
		vt, err := idep.NewVaultTokenQuery(token)
		if err != nil {
			return errors.Wrap(err, "watcher")
		}
		// fakeNotifier is defined near end of file
		w.Register(fakeNotifier(vaultTokenDummyTemplateID), vt)
		w.Poll(vt)
	}
	return nil
}

// WaitCh returns an error channel and runs Wait sending the result down
// the channel. Useful for when you need to use Wait in a select block.
func (w *Watcher) WaitCh(ctx context.Context) <-chan error {
	errCh := make(chan error)
	go func() {
		errCh <- w.Wait(ctx)
	}()
	return errCh
}

// Wait blocks until new a watched value changes or until context is closed
// or exceeds its deadline.
func (w *Watcher) Wait(ctx context.Context) error {
	w.stopCh.drain() // in case Stop was already called

	// send waiting notification, only used for testing
	select {
	case w.waitingCh <- struct{}{}:
	default:
	}

	// combine cache and changed updates so we don't forget one
	dataUpdate := func(v *view) {
		id := v.Dependency().String()
		w.cache.Save(id, v.Data())
		for _, n := range w.tracker.notifiersFor(v) {
			n.Notify(v.Dependency())
		}
	}
	for {
		select {
		case view := <-w.dataCh:
			dataUpdate(view)
			// Drain all dependency data. Prevents re-rendering templates over
			// and over when a large batch of dependencies are updated.
			// See consul-template GH-168 for background.
			for {
				select {
				case view := <-w.dataCh:
					dataUpdate(view)
				case <-time.After(time.Microsecond):
					return nil
				}
			}

		case <-w.bufferTrigger:
			// A template is now ready to be rendered, though there might be a
			// few ready around the same time if they have the same dependencies.
			// Drain the channel similar for the dataCh above.
			for {
				select {
				case <-w.bufferTrigger:
				case <-time.After(time.Microsecond):
					return nil
				}
			}

		case <-w.stopCh:
			return nil

		case err := <-w.errCh:
			// Push the error back up the stack
			return err

		case <-ctx.Done():
			// No changes detected is not considered an error when deadline passes or
			// timeout is reached
			if ctx.Err() == context.DeadlineExceeded {
				return nil
			}
			return ctx.Err()
		}
	}
}

// Buffer sets the template to activate buffer and accumulate changes for a
// period. If the template has not been initalized or a buffer period is not
// configured for the template, it will skip the buffering.
// period.
func (w *Watcher) Buffer(tmplID string) bool {
	// first pass skips buffering.
	_, initialized := w.tracker.notifiers[tmplID]
	if !initialized {
		return false
	}

	return w.bufferTemplates.Buffer(tmplID)
}

// Register is used to add dependencies to be monitored by the watcher. It sets
// everything up but stops short of running the polling, waiting for an
// explicit start.
// If the dependency is already registered, no action is taken.
func (w *Watcher) Register(n Notifier, d dep.Dependency) {
	w.register(n, d)
}

// register is private form of Register that returns the new view.
// Returned view is useful internally and for testing.
// Private as we don't want `view` public at this point.
func (w *Watcher) register(n Notifier, d dep.Dependency) *view {
	if v, ok := w.tracker.lookup(n, d); ok {
		w.tracker.usedID(v.ID())
		return v
	}
	// Choose the correct retry function based off of the dependency's type.
	// NOTE: I would like to abstract this part out to not have type specific
	//       things embedded in general code.
	var retryFunc RetryFunc
	switch d.(type) {
	case idep.ConsulType:
		retryFunc = w.retryFuncConsul
	case idep.VaultType:
		retryFunc = w.retryFuncVault
	}

	v := newView(&newViewInput{
		Dependency:    d,
		Clients:       w.clients,
		MaxStale:      w.maxStale,
		BlockWaitTime: w.blockWaitTime,
		RetryFunc:     retryFunc,
	})
	w.tracker.add(v, n)
	w.tracker.usedID(v.ID())
	return v
}

// Poll starts any/all polling as needed.
// It is idepotent.
// If nothing is passed it checks all views (dependencies).
func (w *Watcher) Poll(deps ...dep.Dependency) {
	if len(deps) == 0 {
		for _, v := range w.tracker.views {
			deps = append(deps, v.Dependency())
		}
	}
	for _, d := range deps {
		if v := w.tracker.view(d.String()); v != nil {
			//log.Printf("[TRACE] (watcher) %s starting", d)
			go v.poll(w.dataCh, w.errCh)
		}
	}
}

// Recaller returns a Recaller (function) that wraps the Store (cache)
// to enable tracking dependencies on the Watcher.
func (w *Watcher) Recaller(n Notifier) Recaller {
	return func(dep dep.Dependency) (interface{}, bool) {
		w.register(n, dep)
		data, ok := w.cache.Recall(dep.String())
		if !ok {
			w.Poll(dep)
		}
		return data, ok
	}
}

// Complete checks if all values in use have been fetched.
// ..also cleans out data no longer used.
func (w *Watcher) Complete(n Notifier) bool {
	defer w.tracker.sweep(n)
	return w.tracker.complete(n)
}

// SetBufferPeriod sets a buffer period to accumulate dependency changes for
// a template.
func (w *Watcher) SetBufferPeriod(min, max time.Duration, tmplIDs ...string) {
	for _, id := range tmplIDs {
		w.bufferTemplates.Add(min, max, id)
	}
}

// Stop halts this watcher and any currently polling views immediately. If a
// view was in the middle of a poll, no data will be returned.
func (w *Watcher) Stop() {
	w.bufferTemplates.Stop()

	//log.Printf("[DEBUG] (watcher) stopping all views")
	w.tracker.stopViews()

	w.stopCh.drain() // So calling Stop twice doesn't block
	w.stopCh <- struct{}{}

	// Empty cache
	if w.cache != nil {
		w.cache.Reset()
	}

	// Close any idle TCP connections
	if w.clients != nil {
		w.clients.Stop()
	}
}

// Size returns the number of views this watcher is watching.
func (w *Watcher) Size() int {
	return w.tracker.viewCount()
}

// Remove-s the given dependency from the list and stops the
// associated view. If a view for the given dependency does not exist, this
// function will return false. If the view does exist, this function will return
// true upon successful deletion.
func (w *Watcher) remove(id string) bool {
	//log.Printf("[DEBUG] (watcher) removing %s", id)

	defer w.cache.Delete(id)
	return w.tracker.remove(id)
}

// Watching determines if the given dependency (id) is being watched.
func (w *Watcher) Watching(id string) bool {
	v := w.tracker.view(id)
	return (v != nil)
}

// view is a convenience function for accessing stored views by id
// note that dependency IDs and their corresponding view IDs are identical
func (w *Watcher) view(id string) *view {
	return w.tracker.view(id)
}

///////////////////////////////////////////////////////////////////////////
// internal structure used to track template <-> dependencies relationships

func newTracker() *tracker {
	return &tracker{
		tracked:   make([]trackedPair, 0, 8),
		views:     make(map[string]*view),
		notifiers: make(map[string]Notifier),
	}
}

// 1 view/notifier pair. Think many-2-many RDBMS table with annotations.
type trackedPair struct {
	// viewed: id of view watched, notified: id of external thing
	// notified, client, consumer
	view, notify string
	// inUse flag gets off pre-render and back on at use
	inUse bool
}

// returns new pair to keep as value
func (tp trackedPair) used() trackedPair {
	tp.inUse = true
	return tp
}

// returns new pair to keep as value
func (tp trackedPair) refresh() trackedPair {
	tp.inUse = false
	return tp
}

type Notifier interface {
	Notify(dep.Dependency)
	ID() string
}

// If performance of looping through tracked gets to be to much build 2 indexes
// of views/notifiers to their trackedPair entries and use that to accel lookups.
// It will require updating though, and complicates things. So wait.

type tracker struct {
	sync.Mutex
	// think in terms of a many-2-many DB relationship
	tracked []trackedPair
	// viewID -> view
	views map[string]*view
	// stringID -> Notifier (stringID is usually template-id)
	notifiers map[string]Notifier
}

// viewCount returns the number of views watched
func (t *tracker) viewCount() int {
	t.Lock()
	defer t.Unlock()
	return len(t.views)
}

// notUsed clears the inUse flag, for testing
func (t *tracker) notUsed(notifierID, viewID string) bool {
	for i, tp := range t.tracked {
		if tp.view == viewID && tp.notify == notifierID {
			t.tracked[i] = tp.refresh()
			return true
		}
	}
	return false
}

// lookup returns the view and true, or nil and false
// true is returned if the notifier and depencency match a tracked pair
// returns the view as it is the 1 thing that you don't have yet
// note that a view's and dependency's IDs are interchangeable (identical)
func (t *tracker) lookup(n Notifier, d dep.Dependency) (*view, bool) {
	notifierID, depID := n.ID(), d.String()
	t.Lock()
	defer t.Unlock()
	for _, tp := range t.tracked {
		if tp.view == depID && tp.notify == notifierID {
			return t.views[tp.view], true
		}
	}
	return nil, false
}

// view returns the view (or nil)
// note that a view's and dependency's IDs are interchangeable (identical)
func (t *tracker) view(viewID string) *view {
	t.Lock()
	defer t.Unlock()
	return t.views[viewID]
}

// adds new tracked entry
func (t *tracker) add(v *view, n Notifier) {
	t.Lock()
	defer t.Unlock()
	t.views[v.ID()] = v
	t.notifiers[n.ID()] = n
	t.tracked = append(t.tracked,
		trackedPair{view: v.ID(), notify: n.ID(), inUse: true})
}

// Marks all trackedPairs w/ a view as having been used
func (t *tracker) usedID(viewID string) {
	t.Lock()
	defer t.Unlock()
	for idx, tp := range t.tracked {
		if tp.view == viewID {
			t.tracked[idx] = tp.used()
		}
	}
}

// Remove view and all trackedPairs that contained it
func (t *tracker) remove(viewID string) bool {
	t.Lock()
	defer t.Unlock()
	delete(t.views, viewID)
	tmp := t.tracked[:0]
	var removed bool
	for _, tp := range t.tracked {
		if tp.view != viewID {
			tmp = append(tmp, tp)
		} else {
			removed = true
		}
	}
	t.tracked = tmp
	return removed
}

// stop all view from polling/watching
func (t *tracker) stopViews() {
	t.Lock()
	defer t.Unlock()
	for id, view := range t.views {
		delete(t.views, id)
		if view == nil {
			continue
		}
		view.stop()
	}
}

// Return all views for a notifier
func (t *tracker) notifiersFor(v *view) []Notifier {
	viewID := v.ID()
	results := make([]Notifier, 0, 8)
	for _, tp := range t.tracked {
		if tp.view == viewID {
			results = append(results, t.notifiers[tp.notify])
		}
	}
	return results
}

// initialized returns true if the view has had its data fetched at least once
func (t *tracker) initialized(viewID string) bool {
	if v, ok := t.views[viewID]; ok {
		return v.receivedData
	}
	return false
}

// complete returns true if every dependency used has been initialized
// ie. it returns true if all values have been fetched
func (t *tracker) complete(n Notifier) bool {
	for _, tp := range t.tracked {
		thisNotifier := tp.notify == n.ID()
		if thisNotifier && tp.inUse && !t.initialized(tp.view) {
			return false
		}
	}
	return true
}

// Clean out un-used trackedPair entries, views and notifiers
// Checks based on passed in notifier, ignores others.
func (t *tracker) sweep(n Notifier) {
	t.Lock()
	defer t.Unlock()
	used := make(map[string]struct{})
	// remove tracked that were used
	tmp := t.tracked[:0]
	for _, tp := range t.tracked {
		otherNotifier := tp.notify != n.ID()
		if tp.inUse || otherNotifier {
			tmp = append(tmp, tp.refresh())
			used[tp.view] = struct{}{}
			used[tp.notify] = struct{}{}
		}
	}
	t.tracked = tmp
	// remove views/notifiers no longer referenced
	for v := range t.views {
		if _, ok := used[v]; !ok {
			delete(t.views, v)
		}
	}
	for n := range t.notifiers {
		if _, ok := used[n]; !ok {
			delete(t.views, n)
		}
	}
}

// dummy Notifier for use by vault token above and in tests
type dummyNotifier struct {
	name string
	deps chan dep.Dependency
}

func fakeNotifier(name string) *dummyNotifier {
	return &dummyNotifier{name: name, deps: make(chan dep.Dependency, 100)}
}
func (n *dummyNotifier) Notify(d dep.Dependency) {
	n.deps <- d
}
func (n *dummyNotifier) ID() string {
	return string(n.name)
}
func (n *dummyNotifier) count() int {
	return len(n.deps)
}
func (n *dummyNotifier) ids() []string {
	results := make([]string, len(n.deps))
	for i := 0; i < len(n.deps); i++ {
		d := <-n.deps
		results[i] = d.String()
		n.deps <- d
	}
	return results
}
