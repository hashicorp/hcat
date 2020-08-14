package dependency

import (
	"fmt"

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

func (d *ConnectLeafQuery) Fetch(clients Clients) (
	interface{}, *ResponseMetadata, error,
) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}
	opts := d.opts.Merge(nil)
	//log.Printf("[TRACE] %s: GET %s", d, &url.URL{
	//	Path:     "/v1/agent/connect/ca/leaf/" + d.service,
	//	RawQuery: opts.String(),
	//})

	cert, md, err := clients.Consul().Agent().ConnectCALeaf(d.service,
		opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	//log.Printf("[TRACE] %s: returned response", d)

	rm := &ResponseMetadata{
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

func (d *ConnectLeafQuery) String() string {
	if d.service != "" {
		return fmt.Sprintf("connect.caleaf(%s)", d.service)
	}
	return "connect.caleaf"
}

func (d *ConnectLeafQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}
