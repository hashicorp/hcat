package dependency

import (
	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ isDependency  = (*ConnectCAQuery)(nil)
	_ BlockingQuery = (*ConnectCAQuery)(nil)
)

type ConnectCAQuery struct {
	isConsul
	isBlocking
	stopCh chan struct{}
	opts   QueryOptions
}

func NewConnectCAQuery() *ConnectCAQuery {
	return &ConnectCAQuery{
		stopCh: make(chan struct{}, 1),
	}
}

func (d *ConnectCAQuery) Fetch(clients dep.Clients) (
	interface{}, *dep.ResponseMetadata, error,
) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(nil)
	certs, md, err := clients.Consul().Agent().ConnectCARoots(
		opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.ID())
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   md.LastIndex,
		LastContact: md.LastContact,
	}

	return certs.Roots, rm, nil
}

func (d *ConnectCAQuery) Stop() {
	close(d.stopCh)
}

func (d *ConnectCAQuery) CanShare() bool {
	return false
}

// ID returns the human-friendly version of this dependency.
func (d *ConnectCAQuery) ID() string {
	return "connect.caroots"
}

// Stringer interface reuses ID
func (d *ConnectCAQuery) String() string {
	return d.ID()
}

func (d *ConnectCAQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
