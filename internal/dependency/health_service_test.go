package dependency

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthServiceQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *HealthServiceQuery
		err  bool
	}{
		{
			"empty",
			"",
			nil,
			true,
		},
		{
			"dc_only",
			"@dc1",
			nil,
			true,
		},
		{
			"near_only",
			"~near",
			nil,
			true,
		},
		{
			"tag_only",
			"tag.",
			nil,
			true,
		},
		{
			"name",
			"name",
			&HealthServiceQuery{
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				passingOnly:             true,
			},
			false,
		},
		{
			"name_dc",
			"name@dc1",
			&HealthServiceQuery{
				dc:                      "dc1",
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				passingOnly:             true,
			},
			false,
		},
		{
			"name_dc_near",
			"name@dc1~near",
			&HealthServiceQuery{
				dc:                      "dc1",
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				near:                    "near",
				passingOnly:             true,
			},
			false,
		},
		{
			"name_near",
			"name~near",
			&HealthServiceQuery{
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				near:                    "near",
				passingOnly:             true,
			},
			false,
		},
		{
			"tag_name",
			"tag.name",
			&HealthServiceQuery{
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				deprecatedTag:           "tag",
				passingOnly:             true,
			},
			false,
		},
		{
			"tag_name_dc",
			"tag.name@dc",
			&HealthServiceQuery{
				dc:                      "dc",
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				deprecatedTag:           "tag",
				passingOnly:             true,
			},
			false,
		},
		{
			"tag_name_near",
			"tag.name~near",
			&HealthServiceQuery{
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				near:                    "near",
				deprecatedTag:           "tag",
				passingOnly:             true,
			},
			false,
		},
		{
			"tag_name_dc_near",
			"tag.name@dc~near",
			&HealthServiceQuery{
				dc:                      "dc",
				deprecatedStatusFilters: []string{"passing"},
				name:                    "name",
				near:                    "near",
				deprecatedTag:           "tag",
				passingOnly:             true,
			},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := NewHealthServiceQuery(tc.i)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if act != nil {
				act.stopCh = nil
			}

			assert.Equal(t, tc.exp, act)
		})
	}
	// Connect
	// all tests above also test connect, just need to check enabling it
	t.Run("connect_query", func(t *testing.T) {
		act, err := NewHealthConnectQuery("name")
		if err != nil {
			t.Fatal(err)
		}
		if act != nil {
			act.stopCh = nil
		}
		exp := &HealthServiceQuery{
			deprecatedStatusFilters: []string{"passing"},
			name:                    "name",
			passingOnly:             true,
			connect:                 true,
		}

		assert.Equal(t, exp, act)
	})
}

func TestHealthConnectServiceQuery_Fetch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		exp  []*dep.HealthService
	}{
		{
			"connect-service",
			"foo",
			[]*dep.HealthService{
				&dep.HealthService{
					Name:           "foo-sidecar-proxy",
					ID:             "foo",
					Kind:           "connect-proxy",
					Port:           21999,
					Status:         "passing",
					Address:        "127.0.0.1",
					NodeAddress:    "127.0.0.1",
					NodeDatacenter: "dc1",
					Tags:           dep.ServiceTags([]string{}),
					NodeMeta: map[string]string{
						"consul-network-segment": ""},
					Weights: api.AgentWeights{
						Passing: 1,
						Warning: 1,
					},
					Namespace: "",
				},
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewHealthConnectQuery(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				d.Stop()
			}()
			res, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}
			var act []*dep.HealthService
			if act = res.([]*dep.HealthService); len(act) != 1 {
				t.Fatal("Expected 1 result, got ", len(act))
			}
			// blank out fields we don't want to test
			inst := act[0]
			inst.Node, inst.NodeID = "", ""
			inst.Checks = nil
			inst.NodeTaggedAddresses = nil

			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestHealthServiceQuery_Fetch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  []*dep.HealthService
	}{
		{
			"consul",
			"consul",
			[]*dep.HealthService{
				&dep.HealthService{
					Node:           testConsul.Config.NodeName,
					NodeAddress:    testConsul.Config.Bind,
					NodeDatacenter: "dc1",
					NodeTaggedAddresses: map[string]string{
						"lan": "127.0.0.1",
						"wan": "127.0.0.1",
					},
					NodeMeta: map[string]string{
						"consul-network-segment": "",
					},
					ServiceMeta: map[string]string{},
					Address:     testConsul.Config.Bind,
					ID:          "consul",
					Name:        "consul",
					Tags:        []string{},
					Status:      "passing",
					Port:        testConsul.Config.Ports.Server,
					Weights: api.AgentWeights{
						Passing: 1,
						Warning: 1,
					},
					Namespace: "",
				},
			},
		},
		{
			"filters",
			"consul|warning",
			[]*dep.HealthService{},
		},
		{
			"multifilter",
			"consul|warning,passing",
			[]*dep.HealthService{
				&dep.HealthService{
					Node:           testConsul.Config.NodeName,
					NodeAddress:    testConsul.Config.Bind,
					NodeDatacenter: "dc1",
					NodeTaggedAddresses: map[string]string{
						"lan": "127.0.0.1",
						"wan": "127.0.0.1",
					},
					NodeMeta: map[string]string{
						"consul-network-segment": "",
					},
					ServiceMeta: map[string]string{},
					Address:     testConsul.Config.Bind,
					ID:          "consul",
					Name:        "consul",
					Tags:        []string{},
					Status:      "passing",
					Port:        testConsul.Config.Ports.Server,
					Weights: api.AgentWeights{
						Passing: 1,
						Warning: 1,
					},
					Namespace: "",
				},
			},
		},
		{
			"service-meta",
			"service-meta",
			[]*dep.HealthService{
				&dep.HealthService{
					Node:           testConsul.Config.NodeName,
					NodeAddress:    testConsul.Config.Bind,
					NodeDatacenter: "dc1",
					NodeTaggedAddresses: map[string]string{
						"lan": "127.0.0.1",
						"wan": "127.0.0.1",
					},
					NodeMeta: map[string]string{
						"consul-network-segment": "",
					},
					ServiceMeta: map[string]string{
						"meta1": "value1",
					},
					Address: testConsul.Config.Bind,
					ID:      "service-meta",
					Name:    "service-meta",
					Tags:    []string{"tag1"},
					Status:  "passing",
					Weights: api.AgentWeights{
						Passing: 1,
						Warning: 1,
					},
					Namespace: "",
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewHealthServiceQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}

			act, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}

			if act != nil {
				for _, v := range act.([]*dep.HealthService) {
					v.NodeID = ""
					v.Checks = nil
					// delete any version data from ServiceMeta
					v.ServiceMeta = filterMeta(v.ServiceMeta)
					v.NodeTaggedAddresses = filterAddresses(
						v.NodeTaggedAddresses)
				}
			}

			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestHealthServiceQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"name",
			"name",
			"health.service(name|passing)",
		},
		{
			"name_dc",
			"name@dc",
			"health.service(name@dc|passing)",
		},
		{
			"name_filter",
			"name|any",
			"health.service(name|any)",
		},
		{
			"name_multifilter",
			"name|warning,passing",
			"health.service(name|passing,warning)",
		},
		{
			"name_near",
			"name~near",
			"health.service(name~near|passing)",
		},
		{
			"name_near_filter",
			"name~near|any",
			"health.service(name~near|any)",
		},
		{
			"name_dc_near",
			"name@dc~near",
			"health.service(name@dc~near|passing)",
		},
		{
			"name_dc_near_filter",
			"name@dc~near|any",
			"health.service(name@dc~near|any)",
		},
		{
			"tag_name",
			"tag.name",
			"health.service(tag.name|passing)",
		},
		{
			"tag_name_dc",
			"tag.name@dc",
			"health.service(tag.name@dc|passing)",
		},
		{
			"tag_name_near",
			"tag.name~near",
			"health.service(tag.name~near|passing)",
		},
		{
			"tag_name_dc_near",
			"tag.name@dc~near",
			"health.service(tag.name@dc~near|passing)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewHealthServiceQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}

func TestNewHealthServiceQueryV1(t *testing.T) {
	t.Parallel()

	t.Run("empty service name", func(t *testing.T) {
		act, err := NewHealthServiceQueryV1("", []string{})
		assert.Error(t, err)
		assert.Nil(t, act)
	})

	cases := []struct {
		name string
		opts []string
		exp  *HealthServiceQuery
		err  bool
	}{
		{
			"no opts",
			[]string{},
			&HealthServiceQuery{
				name:        "name",
				filter:      "Status == passing",
				passingOnly: true,
			},
			false,
		}, {
			"dc",
			[]string{"dc=dc"},
			&HealthServiceQuery{
				name:        "name",
				dc:          "dc",
				filter:      "Status == passing",
				passingOnly: true,
			},
			false,
		}, {
			"near",
			[]string{"near=near"},
			&HealthServiceQuery{
				name:        "name",
				near:        "near",
				filter:      "Status == passing",
				passingOnly: true,
			},
			false,
		}, {
			"namespace",
			[]string{"ns=ns"},
			&HealthServiceQuery{
				name:        "name",
				ns:          "ns",
				filter:      "Status == passing",
				passingOnly: true,
			},
			false,
		}, {
			"multiple queries",
			[]string{"ns=ns", "dc=dc", "near=near"},
			&HealthServiceQuery{
				name:        "name",
				dc:          "dc",
				near:        "near",
				ns:          "ns",
				filter:      "Status == passing",
				passingOnly: true,
			},
			false,
		}, {
			"filters",
			[]string{"Status != passing", "\"my-tag\" in ServiceTags"},
			&HealthServiceQuery{
				name:   "name",
				filter: "Status != passing and \"my-tag\" in ServiceTags",
			},
			false,
		}, {
			"query and filter",
			[]string{"dc=dc", "\"my-tag\" in ServiceTags", "\"another-tag\" in ServiceTags"},
			&HealthServiceQuery{
				name:   "name",
				dc:     "dc",
				filter: "\"my-tag\" in ServiceTags and \"another-tag\" in ServiceTags",
			},
			false,
		}, {
			"invalid query",
			[]string{"dne=dne"},
			nil,
			true,
		}, {
			"invalid filter grammar",
			[]string{"ServiceTags === tag"},
			nil,
			true,
		}, {
			"invalid filter grammar",
			[]string{"Grammer is not empty", "Grammar is very bad"},
			nil,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewHealthServiceQueryV1("name", tc.opts)
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

	// Connect
	// all tests above also test connect, just need to check enabling it
	t.Run("connect_query", func(t *testing.T) {
		act, err := NewHealthConnectQueryV1("name", nil)
		if act != nil {
			act.stopCh = nil
		}
		exp := &HealthServiceQuery{
			filter:      "Status == passing",
			name:        "name",
			passingOnly: true,
			connect:     true,
		}

		assert.NoError(t, err)
		assert.Equal(t, exp, act)
	})
}

func TestHealthServiceQueryV1_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    []string
		exp  string
	}{
		{
			"name",
			[]string{},
			`health.service(name?filter="Status == passing")`,
		}, {
			"dc",
			[]string{"dc=dc"},
			`health.service(name@dc?filter="Status == passing")`,
		}, {
			"near",
			[]string{"near=agent"},
			`health.service(name~agent?filter="Status == passing")`,
		}, {
			"ns",
			[]string{"ns=ns"},
			`health.service(name?ns=ns&filter="Status == passing")`,
		}, {
			"multifilter",
			[]string{"Status contains passing", "mytag in ServiceTags"},
			`health.service(name?filter="Status contains passing and mytag in ServiceTags")`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewHealthServiceQueryV1("name", tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
