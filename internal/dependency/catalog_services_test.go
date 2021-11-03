package dependency

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewCatalogServicesQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *CatalogServicesQuery
		err  bool
	}{
		{
			"empty",
			"",
			&CatalogServicesQuery{},
			false,
		},
		{
			"node",
			"node",
			nil,
			true,
		},
		{
			"dc",
			"@dc1",
			&CatalogServicesQuery{
				dc: "dc1",
			},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := NewCatalogServicesQuery(tc.i)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if act != nil {
				act.stopCh = nil
			}

			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestNewCatalogServicesQueryV1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts []string
		exp  *CatalogServicesQuery
		err  bool
	}{
		{
			"no opts",
			[]string{},
			&CatalogServicesQuery{},
			false,
		},
		{
			"dc",
			[]string{"dc=dc1"},
			&CatalogServicesQuery{
				dc: "dc1",
			},
			false,
		},
		{
			"ns",
			[]string{"ns=namespace"},
			&CatalogServicesQuery{
				ns: "namespace",
			},
			false,
		},
		{
			"node-meta",
			[]string{"node-meta=k:v", "node-meta=foo:bar"},
			&CatalogServicesQuery{
				nodeMeta: map[string]string{"k": "v", "foo": "bar"},
			},
			false,
		},
		{
			"multiple",
			[]string{"node-meta=k:v", "ns=namespace", "dc=dc1"},
			&CatalogServicesQuery{
				dc:       "dc1",
				ns:       "namespace",
				nodeMeta: map[string]string{"k": "v"},
			},
			false,
		},
		{
			"invalid query",
			[]string{"invalid=true"},
			nil,
			true,
		},
		{
			"invalid query format",
			[]string{"dc1"},
			nil,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewCatalogServicesQueryV1(tc.opts)
			if tc.err {
				assert.Error(t, err)
				return
			}

			if act != nil {
				act.stopCh = nil
			}

			assert.NoError(t, err, err)
			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestCatalogServicesQuery_Fetch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  []*dep.CatalogSnippet
	}{
		{
			"all",
			"",
			[]*dep.CatalogSnippet{
				{
					Name: "consul",
					Tags: dep.ServiceTags([]string{}),
				},
				{
					Name: "critical-service",
					Tags: dep.ServiceTags([]string{}),
				},
				{
					Name: "foo-sidecar-proxy",
					Tags: dep.ServiceTags([]string{}),
				},
				{
					Name: "service-meta",
					Tags: dep.ServiceTags([]string{"tag1"}),
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewCatalogServicesQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}

			act, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestCatalogServicesQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"empty",
			"",
			"catalog.services",
		},
		{
			"datacenter",
			"@dc1",
			"catalog.services(@dc1)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewCatalogServicesQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.ID())
		})
	}
}

func TestCatalogServicesQueryV1_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    []string
		exp  string
	}{
		{
			"empty",
			[]string{},
			"catalog.services",
		},
		{
			"datacenter",
			[]string{"dc=dc1"},
			"catalog.services(@dc1)",
		},
		{
			"namespace",
			[]string{"ns=namespace"},
			"catalog.services(ns=namespace)",
		},
		{
			"node-meta",
			[]string{"node-meta=k:v", "node-meta=foo:bar"},
			"catalog.services(node-meta=foo:bar&node-meta=k:v)",
		},
		{
			"multiple",
			[]string{"node-meta=k:v", "dc=dc1", "ns=namespace"},
			"catalog.services(@dc1&node-meta=k:v&ns=namespace)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewCatalogServicesQueryV1(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.ID())
		})
	}
}
