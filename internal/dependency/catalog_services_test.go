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
				&dep.CatalogSnippet{
					Name: "consul",
					Tags: dep.ServiceTags([]string{}),
				},
				&dep.CatalogSnippet{
					Name: "foo-sidecar-proxy",
					Tags: dep.ServiceTags([]string{}),
				},
				&dep.CatalogSnippet{
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
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
