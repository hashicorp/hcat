package dependency

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat/dep"
)

const (
	dcRe          = `(@(?P<dc>[[:word:]\.\-\_]+))?`
	keyRe         = `/?(?P<key>[^@]+)`
	filterRe      = `(\|(?P<filter>[[:word:]\,]+))?`
	serviceNameRe = `(?P<name>[[:word:]\-\_]+)`
	nodeNameRe    = `(?P<name>[[:word:]\.\-\_]+)`
	nearRe        = `(~(?P<near>[[:word:]\.\-\_]+))?`
	prefixRe      = `/?(?P<prefix>[^@]+)`
	tagRe         = `((?P<tag>[[:word:]=:\.\-\_]+)\.)?`
)

// Type aliases to simplify things as we refactor
//type QueryOptions = dep.QueryOptions
type ResponseMetadata = dep.ResponseMetadata

// Using interfaces for type annotations
// see hashicat/dep/ for interface definitions.
type BlockingQuery interface {
	blockingQuery()
}
type VaultType interface {
	Vault()
}
type ConsulType interface {
	Consul()
}
type isConsul struct{}
type isVault struct{}
type isBlocking struct{}

func (isConsul) Consul()          {}
func (isVault) Vault()            {}
func (isBlocking) blockingQuery() {}

// This specifies all the fields internally required by dependencies.
// The public ones + private ones used internally by hashicat.
// Used to validate interface implementations in each dependency file.
type isDependency interface {
	dep.Dependency
	QueryOptionsSetter
}

// used to help shoehorn the dependency setup into hashicat
// until I get a chance to rework it
// Used to assert/access option setting
type QueryOptionsSetter interface {
	SetOptions(QueryOptions)
}

// QueryOptions is a list of options to send with the query. These options are
// client-agnostic, and the dependency determines which, if any, of the options
// to use.
type QueryOptions struct {
	AllowStale        bool
	Datacenter        string
	Filter            string
	Namespace         string
	Near              string
	RequireConsistent bool
	VaultGrace        time.Duration
	WaitIndex         uint64
	WaitTime          time.Duration
	DefaultLease      time.Duration

	ctx context.Context
}

func (q *QueryOptions) Merge(o *QueryOptions) *QueryOptions {
	var r QueryOptions

	if q == nil {
		if o == nil {
			return &QueryOptions{}
		}
		r = *o
		return &r
	}

	r = *q

	if o == nil {
		return &r
	}

	if o.AllowStale != false {
		r.AllowStale = o.AllowStale
	}

	if o.Datacenter != "" {
		r.Datacenter = o.Datacenter
	}

	if o.Filter != "" {
		r.Filter = o.Filter
	}

	if o.Namespace != "" {
		r.Namespace = o.Namespace
	}

	if o.Near != "" {
		r.Near = o.Near
	}

	if o.RequireConsistent != false {
		r.RequireConsistent = o.RequireConsistent
	}

	if o.WaitIndex != 0 {
		r.WaitIndex = o.WaitIndex
	}

	if o.WaitTime != 0 {
		r.WaitTime = o.WaitTime
	}

	return &r
}

func (q *QueryOptions) SetContext(ctx context.Context) QueryOptions {
	var q2 QueryOptions
	if q != nil {
		q2 = *q
	}
	q2.ctx = ctx
	return q2
}

func (q *QueryOptions) ToConsulOpts() *consulapi.QueryOptions {
	cq := consulapi.QueryOptions{
		AllowStale:        q.AllowStale,
		Datacenter:        q.Datacenter,
		Filter:            q.Filter,
		Namespace:         q.Namespace,
		Near:              q.Near,
		RequireConsistent: q.RequireConsistent,
		WaitIndex:         q.WaitIndex,
		WaitTime:          q.WaitTime,
	}

	if q.ctx != nil {
		return cq.WithContext(q.ctx)
	}
	return &cq
}

func (q *QueryOptions) String() string {
	u := &url.Values{}

	if q.AllowStale {
		u.Add("stale", strconv.FormatBool(q.AllowStale))
	}

	if q.Datacenter != "" {
		u.Add("dc", q.Datacenter)
	}

	if q.Filter != "" {
		u.Add("filter", q.Filter)
	}

	if q.Namespace != "" {
		u.Add("ns", q.Namespace)
	}

	if q.Near != "" {
		u.Add("near", q.Near)
	}

	if q.RequireConsistent {
		u.Add("consistent", strconv.FormatBool(q.RequireConsistent))
	}

	if q.WaitIndex != 0 {
		u.Add("index", strconv.FormatUint(q.WaitIndex, 10))
	}

	if q.WaitTime != 0 {
		u.Add("wait", q.WaitTime.String())
	}

	return u.Encode()
}

// deepCopyAndSortTags deep copies the tags in the given string slice and then
// sorts and returns the copied result.
func deepCopyAndSortTags(tags []string) []string {
	newTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		newTags = append(newTags, tag)
	}
	sort.Strings(newTags)
	return newTags
}

// respWithMetadata is a short wrapper to return the given interface with fake
// response metadata for non-Consul dependencies.
func respWithMetadata(i interface{}) (interface{}, *dep.ResponseMetadata, error) {
	return i, &dep.ResponseMetadata{
		LastContact: 0,
		LastIndex:   uint64(time.Now().Unix()),
	}, nil
}

// regexpMatch matches the given regexp and extracts the match groups into a
// named map.
func regexpMatch(re *regexp.Regexp, q string) map[string]string {
	names := re.SubexpNames()
	match := re.FindAllStringSubmatch(q, -1)

	if len(match) == 0 {
		return map[string]string{}
	}

	m := map[string]string{}
	for i, n := range match[0] {
		if names[i] != "" {
			m[names[i]] = n
		}
	}

	return m
}
