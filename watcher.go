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
	Save(IDer, interface{})
	Recall(IDer) (interface{}, bool)
	Delete(IDer)
}

// Watcher is a top-level manager for views that poll Consul for data.
type Watcher struct {
	sync.Mutex
	// clients is the collection of API clients to talk to upstreams.
	clients Looker
	// dataCh is the chan where Views will be published.
	dataCh chan *view
	// errCh is the chan where any errors will be published.
	errCh chan error
	// depViewMap is a map of Template-IDs to Views.
	depViewMap map[string]*view

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
		depViewMap:      make(map[string]*view),
		dataCh:          make(chan *view, dataBufferSize),
		errCh:           make(chan error),
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
		if _, err := w.add(vt); err != nil {
			return nil, errors.Wrap(err, "watcher")
		}
	}

	return w, nil
}

// add adds the given dependency to the list of monitored dependencies
// and start the associated view. If the dependency already exists, no action is
// taken.
//
// If the Dependency already existed, it this function will return false. If the
// view was successfully created, it will return true. If an error occurs while
// creating the view, it will be returned here (but future errors returned by
// the view will happen on the channel).
func (w *Watcher) add(d dep.Dependency) (bool, error) {
	w.Lock()
	defer w.Unlock()

	log.Printf("[DEBUG] (watcher) adding %s", d)

	if _, ok := w.depViewMap[d.String()]; ok {
		log.Printf("[TRACE] (watcher) %s already exists, skipping", d)
		return false, nil
	}

	// Choose the correct retry function based off of the dependency's type.
	var retryFunc RetryFunc
	switch d.Type() {
	case dep.TypeConsul:
		retryFunc = w.retryFuncConsul
	case dep.TypeVault:
		retryFunc = w.retryFuncVault
	}

	v, err := newView(&newViewInput{
		Dependency:    d,
		Clients:       w.clients,
		MaxStale:      w.maxStale,
		BlockWaitTime: w.blockWaitTime,
		RetryFunc:     retryFunc,
	})
	if err != nil {
		return false, errors.Wrap(err, "watcher")
	}

	log.Printf("[TRACE] (watcher) %s starting", d)

	w.depViewMap[d.String()] = v
	go v.poll(w.dataCh, w.errCh)

	return true, nil
}

// Watching determines if the given dependency is being watched.
func (w *Watcher) Watching(d dep.Dependency) bool {
	w.Lock()
	defer w.Unlock()

	_, ok := w.depViewMap[d.String()]
	return ok
}

// ForceWatching is used to force setting the internal state of watching
// a dependency. This is only used for unit testing purposes.
func (w *Watcher) forceWatching(d dep.Dependency, enabled bool) {
	w.Lock()
	defer w.Unlock()

	if enabled {
		w.depViewMap[d.String()] = nil
	} else {
		delete(w.depViewMap, d.String())
	}
}

// Remove removes the given dependency from the list and stops the
// associated view. If a view for the given dependency does not exist, this
// function will return false. If the view does exist, this function will return
// true upon successful deletion.
func (w *Watcher) remove(d dep.Dependency) bool {
	w.Lock()
	defer w.Unlock()

	log.Printf("[DEBUG] (watcher) removing %s", d)

	if view, ok := w.depViewMap[d.String()]; ok {
		log.Printf("[TRACE] (watcher) actually removing %s", d)
		view.stop()
		delete(w.depViewMap, d.String())
		return true
	}

	log.Printf("[TRACE] (watcher) %s did not exist, skipping", d)
	return false
}

// Size returns the number of views this watcher is watching.
func (w *Watcher) Size() int {
	w.Lock()
	defer w.Unlock()
	return len(w.depViewMap)
}

// Stop halts this watcher and any currently polling views immediately. If a
// view was in the middle of a poll, no data will be returned.
func (w *Watcher) Stop() {
	w.Lock()
	defer w.Unlock()

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

	// Close any idle TCP connections
	w.clients.Stop()
}
