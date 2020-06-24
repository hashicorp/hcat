package dependency

import (
	"fmt"
	"sync"
	"time"
)

////////////
// FakeDep is a fake dependency that does not actually speaks to a server.
type FakeDep struct {
	Name string
}

func (d *FakeDep) Fetch(Clients, *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(time.Microsecond)
	data := d.Name
	rm := &ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *FakeDep) CanShare() bool {
	return true
}

func (d *FakeDep) String() string {
	return fmt.Sprintf("test_dep(%s)", d.Name)
}

func (d *FakeDep) Stop() {}

func (d *FakeDep) Type() Type {
	return TypeLocal
}

////////////
// FakeListDep is a fake dependency that does not actually speaks to a server.
// Returns a list, to allow for multi-pass template tests
type FakeListDep struct {
	Name string
	Data []string
}

func (d *FakeListDep) Fetch(Clients, *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(time.Microsecond)
	data := d.Data
	rm := &ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *FakeListDep) CanShare() bool {
	return true
}

func (d *FakeListDep) String() string {
	return fmt.Sprintf("test_list_dep(%s)", d.Name)
}

func (d *FakeListDep) Stop() {}

func (d *FakeListDep) Type() Type {
	return TypeLocal
}

////////////
// FakeDepStale is a fake dependency that can be used to test what happens
// when stale data is permitted.
type FakeDepStale struct {
	Name string
}

// Fetch is used to implement the dependency interface.
func (d *FakeDepStale) Fetch(clients Clients, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(time.Microsecond)

	if opts == nil {
		opts = &QueryOptions{}
	}

	if opts.AllowStale {
		data := "this is some stale data"
		rm := &ResponseMetadata{LastIndex: 1, LastContact: 50 * time.Millisecond}
		return data, rm, nil
	}

	data := "this is some fresh data"
	rm := &ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *FakeDepStale) CanShare() bool {
	return true
}

func (d *FakeDepStale) String() string {
	return fmt.Sprintf("test_dep_stale(%s)", d.Name)
}

func (d *FakeDepStale) Stop() {}

func (d *FakeDepStale) Type() Type {
	return TypeLocal
}

////////////
// FakeDepFetchError is a fake dependency that returns an error while fetching.
type FakeDepFetchError struct {
	Name string
}

func (d *FakeDepFetchError) Fetch(Clients, *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(time.Microsecond)
	return nil, nil, fmt.Errorf("failed to contact server")
}

func (d *FakeDepFetchError) CanShare() bool {
	return true
}

func (d *FakeDepFetchError) String() string {
	return fmt.Sprintf("test_dep_fetch_error(%s)", d.Name)
}

func (d *FakeDepFetchError) Stop() {}

func (d *FakeDepFetchError) Type() Type {
	return TypeLocal
}

var _ Dependency = (*FakeDepSameIndex)(nil)

////////////
type FakeDepSameIndex struct{}

func (d *FakeDepSameIndex) Fetch(Clients, *QueryOptions) (interface{}, *ResponseMetadata, error) {
	meta := &ResponseMetadata{LastIndex: 100}
	return nil, meta, nil
}

func (d *FakeDepSameIndex) CanShare() bool {
	return true
}

func (d *FakeDepSameIndex) Stop() {}

func (d *FakeDepSameIndex) String() string {
	return "test_dep_same_index"
}

func (d *FakeDepSameIndex) Type() Type {
	return TypeLocal
}

////////////
// FakeDepRetry is a fake dependency that errors on the first fetch and
// succeeds on subsequent fetches.
type FakeDepRetry struct {
	sync.Mutex
	Name    string
	retried bool
}

func (d *FakeDepRetry) Fetch(Clients, *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(time.Microsecond)

	d.Lock()
	defer d.Unlock()

	if d.retried {
		data := "this is some data"
		rm := &ResponseMetadata{LastIndex: 1}
		return data, rm, nil
	}

	d.retried = true
	return nil, nil, fmt.Errorf("failed to contact server (try again)")
}

func (d *FakeDepRetry) CanShare() bool {
	return true
}

func (d *FakeDepRetry) String() string {
	return fmt.Sprintf("test_dep_retry(%s)", d.Name)
}

func (d *FakeDepRetry) Stop() {}

func (d *FakeDepRetry) Type() Type {
	return TypeLocal
}
