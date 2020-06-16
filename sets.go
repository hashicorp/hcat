package hat

import (
	"fmt"
	"sync"

	dep "github.com/hashicorp/hat/internal/dependency"
)

// stringSet is a simple string set implementation used
type stringSet struct {
	*sync.RWMutex
	set map[string]struct{}
}

func newStringSet() stringSet {
	return stringSet{
		RWMutex: &sync.RWMutex{},
		set:     make(map[string]struct{}),
	}
}

// Len(gth) or size of set
func (s stringSet) Len() int {
	return len(s.set)
}

// Add and entry to the set
func (s stringSet) add(k string) {
	s.set[k] = struct{}{}
}
func (s stringSet) Add(k string) {
	s.Lock()
	defer s.Unlock()
	s.add(k)
}

// Map returns a copy of the underlying map used by the set
func (s stringSet) Map() map[string]struct{} {
	s.RLock()
	defer s.RUnlock()
	newmap := make(map[string]struct{}, len(s.set))
	for k, v := range s.set {
		newmap[k] = v
	}
	return newmap
}

// Clear deletes all entries from set
func (s stringSet) Clear() {
	s.Lock()
	defer s.Unlock()
	for k := range s.set {
		delete(s.set, k)
	}
}

// depSet is a dependency-specific set implementation.
// Relative ordering is preserved.
type depSet struct {
	stringSet
	list []dep.Dependency
}

func newDepSet() depSet {
	return depSet{
		list:      make([]dep.Dependency, 0, 8),
		stringSet: newStringSet(),
	}
}

// Add adds a new element to the set if it does not already exist.
func (s depSet) Add(d dep.Dependency) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.stringSet.set[d.String()]; !ok {
		s.list = append(s.list, d)
		s.stringSet.add(d.String())
		return true
	}
	return false
}

// List returns the insertion-ordered list of dependencies.
func (s depSet) List() []dep.Dependency {
	s.RLock()
	defer s.RUnlock()
	return s.list[:]
}

// String is a string representation of the set.
func (s depSet) String() string {
	s.RLock()
	defer s.RUnlock()
	return fmt.Sprint(s.list)
}
