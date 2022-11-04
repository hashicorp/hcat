package vaulttoken

import (
	"github.com/hashicorp/hcat/dep"
)

type callback func(any) bool

// dep Notifier for use by vault token above and in tests
type callbackNotifier struct {
	dep dep.Dependency
	fun callback
}

// returned boolean controls watcher.Watch channel output
// ie. returning false will skip sending it on that channel.
func (n callbackNotifier) Notify(d any) (ok bool) {
	if n.fun != nil {
		return n.fun(d)
	} else {
		return true
	}
}

// unique ID for this notifier (amoung the pop of notifiers)
func (n callbackNotifier) ID() string {
	return n.dep.ID()
}
