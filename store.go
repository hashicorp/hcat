package hcat

import (
	r "reflect"
	"sync"
)

// Store is what Template uses to determine the values that are
// available for template parsing.
type Store struct {
	sync.RWMutex

	// data is the map of individual dependencies and the most recent data for
	// that dependency.
	data map[string]interface{}
}

// NewStore creates a new Store with empty values for each
// of the key structs.
func NewStore() *Store {
	return &Store{
		data: make(map[string]interface{}),
	}
}

// Save accepts a dependency and the data to store associated with that
// dep. This function converts the given data to a proper type and stores
// it interally.
func (s *Store) Save(id string, data interface{}) {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.data[id]; ok {
		s.data[id] = data
		return
	}
	// only write initial value if valid/non-empty/non-nil
	v := r.ValueOf(data)
	if !v.IsValid() || v.IsZero() {
		return
	}
	switch v.Kind() {
	case r.Chan, r.Func, r.Interface, r.Ptr, r.Slice:
		if v.IsNil() {
			return
		}
	}
	s.data[id] = data
}

// Recall gets the current value for the given dependency in the Store.
func (s *Store) Recall(id string) (interface{}, bool) {
	s.RLock()
	defer s.RUnlock()

	data, ok := s.data[id]
	return data, ok
}

// Forget accepts a dependency and removes all associated data with this
// dependency.
func (s *Store) Delete(id string) {
	s.Lock()
	defer s.Unlock()

	delete(s.data, id)
}

// Reset clears all stored data.
func (s *Store) Reset() {
	s.Lock()
	defer s.Unlock()

	for k := range s.data {
		delete(s.data, k)
	}
}

// forceSet is used to force set the value of a dependency for a given hash
// code. Used in testing.
func (s *Store) forceSet(hashCode string, data interface{}) {
	s.Lock()
	defer s.Unlock()

	s.data[hashCode] = data
}
