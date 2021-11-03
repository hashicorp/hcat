package dependency

import (
	"fmt"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency  = (*ConnectLeafQuery)(nil)
	_ BlockingQuery = (*ConnectLeafQuery)(nil)
)

type ConnectLeafQuery struct {
	isConsul
	isBlocking
	stopCh chan struct{}

	service string
	opts    QueryOptions
}

func NewConnectLeafQuery(service string) *ConnectLeafQuery {
	return &ConnectLeafQuery{
		stopCh:  make(chan struct{}, 1),
		service: service,
	}
}

func (d *ConnectLeafQuery) Fetch(clients dep.Clients) (
	interface{}, *dep.ResponseMetadata, error,
) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}
	opts := d.opts.Merge(nil)

	cert, md, err := clients.Consul().Agent().ConnectCALeaf(d.service,
		opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   md.LastIndex,
		LastContact: md.LastContact,
	}

	return cert, rm, nil
}

func (d *ConnectLeafQuery) Stop() {
	close(d.stopCh)
}

func (d *ConnectLeafQuery) CanShare() bool {
	return false
}

// ID returns the human-friendly version of this dependency.
func (d *ConnectLeafQuery) ID() string {
	if d.service != "" {
		return fmt.Sprintf("connect.caleaf(%s)", d.service)
	}
	return "connect.caleaf"
}

// Stringer interface reuses ID
func (d *ConnectLeafQuery) String() string {
	return d.ID()
}

func (d *ConnectLeafQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
