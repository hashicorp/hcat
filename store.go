package hat

import (
	"sync"

	dep "github.com/hashicorp/hat/internal/dependency"
)

// Store is what Template uses to determine the values that are
// available for template parsing.
type Store struct {
	sync.RWMutex

	// data is the map of individual dependencies and the most recent data for
	// that dependency.
	data map[string]interface{}

	// receivedData is an internal tracker of which dependencies have stored
	// data in the Store.
	receivedData map[string]struct{}
}

// NewStore creates a new Store with empty values for each
// of the key structs.
func NewStore() *Store {
	return &Store{
		data:         make(map[string]interface{}),
		receivedData: make(map[string]struct{}),
	}
}

// Save accepts a dependency and the data to store associated with that
// dep. This function converts the given data to a proper type and stores
// it interally.
func (b *Store) Save(d dep.Dependency, data interface{}) {
	b.Lock()
	defer b.Unlock()

	b.data[d.String()] = data
	b.receivedData[d.String()] = struct{}{}
}

// Recall gets the current value for the given dependency in the Store.
func (b *Store) Recall(d dep.Dependency) (interface{}, bool) {
	b.RLock()
	defer b.RUnlock()

	// If we have not received data for this dependency, return now.
	if _, ok := b.receivedData[d.String()]; !ok {
		return nil, false
	}

	return b.data[d.String()], true
}

// ForceSet is used to force set the value of a dependency
// for a given hash code
func (b *Store) ForceSet(hashCode string, data interface{}) {
	b.Lock()
	defer b.Unlock()

	b.data[hashCode] = data
	b.receivedData[hashCode] = struct{}{}
}

// Forget accepts a dependency and removes all associated data with this
// dependency. It also resets the "receivedData" internal map.
func (b *Store) Delete(d dep.Dependency) {
	b.Lock()
	defer b.Unlock()

	delete(b.data, d.String())
	delete(b.receivedData, d.String())
}
