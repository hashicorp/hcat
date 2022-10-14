package test

import (
	"sync"

	"github.com/hashicorp/consul/sdk/testutil"
)

// Meets consul/sdk/testutil/TestingTB interface
var _ testutil.TestingTB = (*TestingTB)(nil)

type TestingTB struct {
	sync.Mutex
	cleanup func()
}

func (t *TestingTB) DoCleanup() {
	t.Lock()
	defer t.Unlock()
	t.cleanup()
}

func (*TestingTB) Failed() bool                  { return false }
func (*TestingTB) Logf(string, ...interface{})   {}
func (*TestingTB) Fatalf(string, ...interface{}) {}
func (*TestingTB) Name() string                  { return "TestingTB" }
func (*TestingTB) Helper()                       {}
func (t *TestingTB) Cleanup(f func()) {
	t.Lock()
	defer t.Unlock()
	prev := t.cleanup
	t.cleanup = func() {
		f()
		if prev != nil {
			prev()
		}
	}
}
