package hat

import (
	"log"
	"sync"
	"time"

	dep "github.com/hashicorp/hat/internal/dependency"
	"github.com/pkg/errors"
)

// dataBufferSize is the default number of views to process in a batch.
const dataBufferSize = 2048

type RetryFunc func(int) (bool, time.Duration)

type Cacher interface {
	Save(string, interface{})
	Recall(string) (interface{}, bool)
	Delete(string)
	Reset()
}

// Watcher is a top-level manager for views that poll Consul for data.
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

type NewWatcherInput struct {
	// Clients is the client set to communicate with upstreams.
	Clients Looker
	// Cache is the Cacher for caching watched values
	Cache Cacher

	// Optional Vault specific parameters
	// VaultToken is a Vault token used to access Vault.
	VaultToken string
	// VaultRenewToken indicates if this watcher should renew VaultToken.
	VaultRenewToken bool
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
func NewWatcher(i *NewWatcherInput) (*Watcher, error) {
	w := &Watcher{
		clients:         i.Clients,
		cache:           i.Cache,
		depViewMap:      make(map[string]*view),
		dataCh:          make(chan *view, dataBufferSize),
		errCh:           make(chan error),
		olddepCh:        make(chan string, dataBufferSize),
		retryFuncConsul: i.ConsulRetryFunc,
		maxStale:        i.ConsulMaxStale,
		blockWaitTime:   i.ConsulBlockWait,
		retryFuncVault:  i.VaultRetryFunc,
		defaultLease:    i.VaultDefaultLease,
	}

	// Start a watcher for the Vault renew if that config was specified
	if i.VaultRenewToken && i.VaultToken != "" {
		vt, err := dep.NewVaultTokenQuery(i.VaultToken)
		if err != nil {
			return nil, errors.Wrap(err, "watcher")
		}
		w.add(vt)
	}

	return w, nil
}

func (w *Watcher) Wait(timeout time.Duration) error {
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}
	for {
		select {
		case <-timer:
			return nil
		case view := <-w.dataCh:
			w.cache.Save(view.Dependency(), view.Data())

			// Drain all dependency data. Prevents re-rendering templates over
			// and over when a large batch of dependencies are updated.
			// See GH-168 for background.
			for {
				select {
				case view := <-w.dataCh:
					w.cache.Save(view.Dependency(), view.Data())
				case <-time.After(time.Microsecond):
					return nil
				}
			}

		// XXX the resolver code will write to this
		// olddepCh outputs deps that need to be removed
		// should it drain, like above for dataCh?
		case d := <-w.olddepCh:
			w.remove(d)

		case err := <-w.errCh:
			// Push the error back up the stack
			return err

		}
	}
}

// Stop halts this watcher and any currently polling views immediately. If a
// view was in the middle of a poll, no data will be returned.
func (w *Watcher) Stop() {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	log.Printf("[DEBUG] (watcher) stopping all views")

	for _, view := range w.depViewMap {
		if view == nil {
			continue
		}
		log.Printf("[TRACE] (watcher) stopping %s", view.Dependency())
		view.stop()
	}

	// Reset the map to have no views
	w.depViewMap = make(map[string]*view)

	// Empty cache
	w.cache.Reset()

	// Close any idle TCP connections
	w.clients.Stop()
}

// Wrap embedded cache's Recaller interface
func (w *Watcher) Recall(id string) (interface{}, bool) {
	return w.cache.Recall(id)
}

// Watching determines if the given dependency is being watched.
func (w *Watcher) Watching(id string) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	_, ok := w.depViewMap[id]
	return ok
}

// Size returns the number of views this watcher is watching.
func (w *Watcher) Size() int {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()
	return len(w.depViewMap)
}

// add adds the given dependency to the list of monitored dependencies
// and start the associated view. If the dependency already exists, no action is
// taken.
//
// If the Dependency already existed, it this function will return false. If the
// view was successfully created, it will return true.
func (w *Watcher) add(d dep.Dependency) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	log.Printf("[DEBUG] (watcher) adding %s", d)

	if _, ok := w.depViewMap[d.String()]; ok {
		log.Printf("[TRACE] (watcher) %s already exists, skipping", d)
		return false
	}

	// Choose the correct retry function based off of the dependency's type.
	var retryFunc RetryFunc
	switch d.Type() {
	case dep.TypeConsul:
		retryFunc = w.retryFuncConsul
	case dep.TypeVault:
		retryFunc = w.retryFuncVault
	}

	v := newView(&newViewInput{
		Dependency:    d,
		Clients:       w.clients,
		MaxStale:      w.maxStale,
		BlockWaitTime: w.blockWaitTime,
		RetryFunc:     retryFunc,
	})

	log.Printf("[TRACE] (watcher) %s starting", d)

	w.depViewMap[d.String()] = v
	go v.poll(w.dataCh, w.errCh)

	return true
}

// Remove-s the given dependency from the list and stops the
// associated view. If a view for the given dependency does not exist, this
// function will return false. If the view does exist, this function will return
// true upon successful deletion.
func (w *Watcher) remove(id string) bool {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	log.Printf("[DEBUG] (watcher) removing %s", id)

	defer w.cache.Delete(id)

	if view, ok := w.depViewMap[id]; ok {
		log.Printf("[TRACE] (watcher) actually removing %s", id)
		view.stop()
		delete(w.depViewMap, id)
		return true
	}

	log.Printf("[TRACE] (watcher) %s did not exist, skipping", id)
	return false
}

// ForceWatching is used to force setting the internal state of watching
// a dependency. This is only used for unit testing purposes.
func (w *Watcher) forceWatching(d dep.Dependency, enabled bool) {
	w.depViewMapMx.Lock()
	defer w.depViewMapMx.Unlock()

	if enabled {
		w.depViewMap[d.String()] = nil
	} else {
		delete(w.depViewMap, d.String())
	}
}
