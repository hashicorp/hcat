package hat

import (
	"fmt"
	"sync"
	"time"

	dep "github.com/hashicorp/hat/internal/dependency"
)

// FakeDep is a fake dependency that does not actually speaks to a server.
type FakeDep struct {
	name string
}

func (d *FakeDep) Fetch(clients dep.Clients, opts *dep.QueryOptions) (interface{}, *dep.ResponseMetadata, error) {
	time.Sleep(time.Millisecond)
	data := "this is some data"
	rm := &dep.ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *FakeDep) CanShare() bool {
	return true
}

func (d *FakeDep) String() string {
	return fmt.Sprintf("test_dep(%s)", d.name)
}

func (d *FakeDep) Stop() {}

func (d *FakeDep) Type() dep.Type {
	return dep.TypeLocal
}

// FakeDepStale is a fake dependency that can be used to test what happens
// when stale data is permitted.
type FakeDepStale struct {
	name string
}

// Fetch is used to implement the dependency interface.
func (d *FakeDepStale) Fetch(clients dep.Clients, opts *dep.QueryOptions) (interface{}, *dep.ResponseMetadata, error) {
	time.Sleep(time.Millisecond)

	if opts == nil {
		opts = &dep.QueryOptions{}
	}

	if opts.AllowStale {
		data := "this is some stale data"
		rm := &dep.ResponseMetadata{LastIndex: 1, LastContact: 50 * time.Millisecond}
		return data, rm, nil
	}

	data := "this is some fresh data"
	rm := &dep.ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *FakeDepStale) CanShare() bool {
	return true
}

func (d *FakeDepStale) String() string {
	return fmt.Sprintf("test_dep_stale(%s)", d.name)
}

func (d *FakeDepStale) Stop() {}

func (d *FakeDepStale) Type() dep.Type {
	return dep.TypeLocal
}

// FakeDepFetchError is a fake dependency that returns an error while fetching.
type FakeDepFetchError struct {
	name string
}

func (d *FakeDepFetchError) Fetch(clients dep.Clients, opts *dep.QueryOptions) (interface{}, *dep.ResponseMetadata, error) {
	time.Sleep(time.Millisecond)
	return nil, nil, fmt.Errorf("failed to contact server")
}

func (d *FakeDepFetchError) CanShare() bool {
	return true
}

func (d *FakeDepFetchError) String() string {
	return fmt.Sprintf("test_dep_fetch_error(%s)", d.name)
}

func (d *FakeDepFetchError) Stop() {}

func (d *FakeDepFetchError) Type() dep.Type {
	return dep.TypeLocal
}

var _ dep.Dependency = (*FakeDepSameIndex)(nil)

type FakeDepSameIndex struct{}

func (d *FakeDepSameIndex) Fetch(clients dep.Clients, opts *dep.QueryOptions) (interface{}, *dep.ResponseMetadata, error) {
	meta := &dep.ResponseMetadata{LastIndex: 100}
	return nil, meta, nil
}

func (d *FakeDepSameIndex) CanShare() bool {
	return true
}

func (d *FakeDepSameIndex) Stop() {}

func (d *FakeDepSameIndex) String() string {
	return "test_dep_same_index"
}

func (d *FakeDepSameIndex) Type() dep.Type {
	return dep.TypeLocal
}

// FakeDepRetry is a fake dependency that errors on the first fetch and
// succeeds on subsequent fetches.
type FakeDepRetry struct {
	sync.Mutex
	name    string
	retried bool
}

func (d *FakeDepRetry) Fetch(clients dep.Clients, opts *dep.QueryOptions) (interface{}, *dep.ResponseMetadata, error) {
	time.Sleep(time.Millisecond)

	d.Lock()
	defer d.Unlock()

	if d.retried {
		data := "this is some data"
		rm := &dep.ResponseMetadata{LastIndex: 1}
		return data, rm, nil
	}

	d.retried = true
	return nil, nil, fmt.Errorf("failed to contact server (try again)")
}

func (d *FakeDepRetry) CanShare() bool {
	return true
}

func (d *FakeDepRetry) String() string {
	return fmt.Sprintf("test_dep_retry(%s)", d.name)
}

func (d *FakeDepRetry) Stop() {}

func (d *FakeDepRetry) Type() dep.Type {
	return dep.TypeLocal
}
