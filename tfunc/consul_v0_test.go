package tfunc

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestConsulV0Execute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}

	testFunc := func(tc testCase) func(*testing.T) {
		return func(t *testing.T) {
			tpl := NewTemplate(tc.ti)

			a, err := tpl.Execute(tc.i.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if tc.err && errors.Is(err, errFuncNotImplemented) {
				t.Errorf("bad error: %v", err)
			}

			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		}
	}

	cases := []testCase{
		{
			"missing_deps",
			hcat.TemplateInput{
				Contents: `{{ key "foo" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"",
			false,
		},
		{
			"func_datacenters",
			hcat.TemplateInput{
				Contents: `{{ datacenters }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogDatacentersQuery(false)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []string{"dc1", "dc2"})
				return fakeWatcher{st}
			}(),
			"[dc1 dc2]",
			false,
		},
		{
			"func_datacenters_ignore",
			hcat.TemplateInput{
				Contents: `{{ datacenters true }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogDatacentersQuery(true)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []string{"dc1", "dc2"})
				return fakeWatcher{st}
			}(),
			"[dc1 dc2]",
			false,
		},
		{
			"func_key",
			hcat.TemplateInput{
				Contents: `{{ key "key" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), "5")
				return fakeWatcher{st}
			}(),
			"5",
			false,
		},
		{
			"func_keyExists",
			hcat.TemplateInput{
				Contents: `{{ keyExists "key" }} {{ keyExists "no_key" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), true)
				return fakeWatcher{st}
			}(),
			"true false",
			false,
		},
		{
			"func_keyOrDefault",
			hcat.TemplateInput{
				Contents: `{{ keyOrDefault "key" "100" }} {{ keyOrDefault "no_key" "200" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), "150")
				return fakeWatcher{st}
			}(),
			"150 200",
			false,
		},
		{
			"func_ls",
			hcat.TemplateInput{
				Contents: `{{ range ls "list" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVListQuery("list")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.KeyPair{
					{Key: "", Value: ""},
					{Key: "foo", Value: "bar"},
					{Key: "foo/zip", Value: "zap"},
				})
				return fakeWatcher{st}
			}(),
			"foo=bar",
			false,
		},
		{
			"func_node",
			hcat.TemplateInput{
				Contents: `{{ with node }}{{ .Node.Node }}{{ range .Services }}{{ .Service }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogNodeQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.CatalogNode{
					Node: &dep.Node{Node: "node1"},
					Services: []*dep.CatalogNodeService{
						{
							Service: "service1",
						},
					},
				})
				return fakeWatcher{st}
			}(),
			"node1service1",
			false,
		},
		{
			"func_nodes",
			hcat.TemplateInput{
				Contents: `{{ range nodes }}{{ .Node }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogNodesQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.Node{
					{Node: "node1"},
					{Node: "node2"},
				})
				return fakeWatcher{st}
			}(),
			"node1node2",
			false,
		},
		{
			"func_service",
			hcat.TemplateInput{
				Contents: `{{ range service "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.HealthService{
					{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					{
						Node:    "node2",
						Address: "5.6.7.8",
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"func_service_filter",
			hcat.TemplateInput{
				Contents: `{{ range service "webapp" "passing,any" }}{{ .Address }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewHealthServiceQuery("webapp|passing,any")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.HealthService{
					{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					{
						Node:    "node2",
						Address: "5.6.7.8",
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"func_services",
			hcat.TemplateInput{
				Contents: `{{ range services }}{{ .Name }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogServicesQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.CatalogSnippet{
					{
						Name: "service1",
					},
					{
						Name: "service2",
					},
				})
				return fakeWatcher{st}
			}(),
			"service1service2",
			false,
		},
		{
			"func_tree",
			hcat.TemplateInput{
				Contents: `{{ range tree "key" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVListQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.KeyPair{
					{Key: "", Value: ""},
					{Key: "admin/port", Value: "1134"},
					{Key: "maxconns", Value: "5"},
					{Key: "minconns", Value: "2"},
				})
				return fakeWatcher{st}
			}(),
			"admin/port=1134maxconns=5minconns=2",
			false,
		},
		{
			"leaf_cert",
			hcat.TemplateInput{
				Contents: `{{with caLeaf "foo"}}` +
					`{{.CertPEM}}{{.PrivateKeyPEM}}{{end}}`,
			},
			func() hcat.Watcherer {
				d := idep.NewConnectLeafQuery("foo")
				st := hcat.NewStore()
				st.Save(d.ID(), &api.LeafCert{
					Service:       "foo",
					CertPEM:       "PEM",
					PrivateKeyPEM: "KEY",
				})
				return fakeWatcher{st}
			}(),
			"PEMKEY",
			false,
		},
		{
			"root_ca",
			hcat.TemplateInput{
				Contents: `{{range caRoots}}{{.RootCertPEM}}{{end}}`,
			},
			func() hcat.Watcherer {
				d := idep.NewConnectCAQuery()
				st := hcat.NewStore()
				st.Save(d.ID(), []*api.CARoot{
					{
						Name:        "Consul CA Root Cert",
						RootCertPEM: "PEM",
						Active:      true,
					},
				})
				return fakeWatcher{st}
			}(),
			"PEM",
			false,
		},
		{
			"func_connect",
			hcat.TemplateInput{
				Contents: `{{ range connect "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewHealthConnectQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.HealthService{
					{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					{
						Node:    "node2",
						Address: "5.6.7.8",
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		},

		// helpers
		{
			"helper_by_key",
			hcat.TemplateInput{
				Contents: `{{ range $key, $pairs := tree "list" | byKey }}{{ $key }}:{{ range $pairs }}{{ .Key }}={{ .Value }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testKVListQueryID("list")
				st.Save(id, []*dep.KeyPair{
					{Key: "", Value: ""},
					{Key: "foo/bar", Value: "a"},
					{Key: "zip/zap", Value: "b"},
				})
				return fakeWatcher{st}
			}(),
			"foo:bar=azip:zap=b",
			false,
		},
		{
			"helper_by_tag",
			hcat.TemplateInput{
				Contents: `{{ range $tag, $services := service "webapp" | byTag }}{{ $tag }}:{{ range $services }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "staging"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"staging"},
					},
				})
				return fakeWatcher{st}
			}(),
			"prod:1.2.3.4staging:1.2.3.45.6.7.8",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), testFunc(tc))
	}
}
