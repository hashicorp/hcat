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
	// olddepCh is the chan where no longer used dependencies are sent.
	olddepCh chan string

	// changed is a list of deps that have been changed since last check
	changed stringSet
	// tracker tracks template<->dependencies (see bottom of this file)
	depTracker *tracker

	// depViewMap is a map of Dependency-IDs to Views.
	depViewMap   map[string]*view
	depViewMapMx sync.Mutex

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

	w := &Watcher{
		clients:         clients,
		cache:           cache,
		dataCh:          make(chan *view, dataBufferSize),
		errCh:           make(chan error),
		olddepCh:        make(chan string, dataBufferSize),
		changed:         newStringSet(),
		depTracker:      newTracker(),
		depViewMap:      make(map[string]*view),
		retryFuncConsul: i.ConsulRetryFunc,
		maxStale:        i.ConsulMaxStale,
		blockWaitTime:   i.ConsulBlockWait,
		retryFuncVault:  i.VaultRetryFunc,
		defaultLease:    i.VaultDefaultLease,
	}

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
		w.Add(vt)
		// prevent cleanDeps from removing it
		w.Register(vaultTokenDummyTemplateID, vt)
	}
	return nil
}

// WaitCh returns an error channel and runs Wait sending the result down
// the channel. Sugur for when you need to use Wait in a select block.
func (w *Watcher) WaitCh(ctx context.Context, timeout time.Duration) <-chan error {
	errCh := make(chan error)
	go func() {
		errCh <- w.Wait(ctx, timeout)
	}()
	return errCh
}

// Wait blocks until new a watched value changes
func (w *Watcher) Wait(ctx context.Context, timeout time.Duration) error {
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}
	w.changed.Clear() // clear old updates before waiting on new ones

	cleanStop := make(chan struct{})
	defer close(cleanStop) // only run while waiting
	go func() {
		w.cleanDeps(cleanStop)
	}()

	// combine cache and changed updates so we don't forget one
	dataUpdate := func(v *view) {
		id := v.Dependency().String()
		w.cache.Save(id, v.Data())
		w.changed.Add(id)
	}
	for {
		select {
		case <-timer:
			return nil
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

		// olddepCh outputs deps that need to be removed
		case d := <-w.olddepCh:
			w.remove(d)

		case err := <-w.errCh:
			// Push the error back up the stack
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// cleanDeps scans all current dependencies and removes any unused ones.
// This can happen in, for example, cases of a loop that gets service
// information for all services and the 'all services' list changes over time.
func (w *Watcher) cleanDeps(done chan struct{}) {
	// get all dependencies
	w.depViewMapMx.Lock()
	deps := make([]string, 0, len(w.depViewMap))
	for k := range w.depViewMap {
		deps = append(deps, k)
	}
	w.depViewMapMx.Unlock()
	// remove any no longer used
	for k := range w.depTracker.findUnused(deps) {
		w.remove(k)
		select {
		case <-done:
			return
		default:
		}
	}
}

// Register is used to tell the Watcher which dependencies are used by
// which templates. This is used to enable a you to check to see if a template
// needs to be updated by checking if any of its dependencies have Changed().
func (w *Watcher) Register(tmplID string, deps ...dep.Dependency) {
	if len(deps) > 0 {
		w.depTracker.update(tmplID, deps...)
	}
}

// Changed is used to check a template to see if any of its dependencies
// have been updated (changed).
// Returns True if template dependencies have changed.
func (w *Watcher) Changed(tmplID string) bool {
	deps, initialized := w.depTracker.templateDeps(tmplID)
	if !initialized { // first pass, always return true
		return true
	}
	for depID := range w.changed.Map() {
		if _, ok := deps[depID]; ok {
			return true
		}
	}
	return false
}

// Add the given dependency to the list of monitored dependencies and start the
// associated view. If the dependency already exists, no action is taken.
//
// If the Dependency already existed, it this function will return false. If the
// view was successfully created, it will return true.
func (w *Watcher) Add(d dep.Dependency) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	//log.Printf("[DEBUG] (watcher) adding %s", d)

	if _, ok := w.depViewMap[d.String()]; ok {
		//log.Printf("[TRACE] (watcher) %s already exists, skipping", d)
		return false
	}

	// Choose the correct retry function based off of the dependency's type.
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

	//log.Printf("[TRACE] (watcher) %s starting", d)

	w.depViewMap[d.String()] = v
	go v.poll(w.dataCh, w.errCh)

	return true
}

// Wrap embedded cache's Recaller interface
func (w *Watcher) Recall(id string) (interface{}, bool) {
	return w.cache.Recall(id)
}

// Stop halts this watcher and any currently polling views immediately. If a
// view was in the middle of a poll, no data will be returned.
func (w *Watcher) Stop() {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	//log.Printf("[DEBUG] (watcher) stopping all views")

	for _, view := range w.depViewMap {
		if view == nil {
			continue
		}
		//log.Printf("[TRACE] (watcher) stopping %s", view.Dependency())
		view.stop()
	}

	// Reset the map to have no views
	w.depViewMap = make(map[string]*view)

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
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()
	return len(w.depViewMap)
}

// Remove-s the given dependency from the list and stops the
// associated view. If a view for the given dependency does not exist, this
// function will return false. If the view does exist, this function will return
// true upon successful deletion.
func (w *Watcher) remove(id string) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	//log.Printf("[DEBUG] (watcher) removing %s", id)

	defer w.cache.Delete(id)

	if view, ok := w.depViewMap[id]; ok {
		//log.Printf("[TRACE] (watcher) actually removing %s", id)
		view.stop()
		delete(w.depViewMap, id)
		return true
	}

	//log.Printf("[TRACE] (watcher) %s did not exist, skipping", id)
	return false
}

// Watching determines if the given dependency (id) is being watched.
func (w *Watcher) Watching(id string) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	_, ok := w.depViewMap[id]
	return ok
}

///////////
// internal structure used to track template <-> dependencies relationships
type depmap map[string]map[string]struct{}

func (dm depmap) add(k, k1 string) {
	if _, ok := dm[k]; !ok {
		dm[k] = make(map[string]struct{})
	}
	dm[k][k1] = struct{}{}
}

type tracker struct {
	sync.RWMutex
	deps depmap
	tpls depmap
}

func newTracker() *tracker {
	return &tracker{
		deps: make(depmap),
		tpls: make(depmap),
	}
}

func (t *tracker) update(tmplID string, deps ...dep.Dependency) {
	t.clear(tmplID)
	t.Lock()
	defer t.Unlock()
	for _, d := range deps {
		t.deps.add(d.String(), tmplID)
		t.tpls.add(tmplID, d.String())
	}
}

func (t *tracker) clear(tmplID string) {
	t.Lock()
	defer t.Unlock()
	for d := range t.tpls[tmplID] {
		delete(t.deps[d], tmplID)
		delete(t.tpls[tmplID], d)
	}
}

func (t *tracker) findUnused(depIDs []string) map[string]struct{} {
	t.RLock()
	defer t.RUnlock()
	result := make(map[string]struct{})
	for _, depID := range depIDs {
		if len(t.deps[depID]) == 0 {
			result[depID] = struct{}{}
		}
	}
	return result
}

func (t *tracker) templateDeps(tmplID string) (map[string]struct{}, bool) {
	t.RLock()
	defer t.RUnlock()
	deps, ok := t.tpls[tmplID]
	return deps, ok
}
