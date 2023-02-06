// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestTemplateExecuteConsulV1(t *testing.T) {
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
			tc.ti.FuncMapMerge = ConsulV1()
			tpl := newTemplate(tc.ti)

			a, err := tpl.Execute(tc.i.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if tc.err && !errors.Is(err, errFuncNotImplemented) {
				t.Errorf("bad error: %v", err)
			}

			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		}
	}

	cases := []testCase{
		{
			"func_service",
			hcat.TemplateInput{
				Contents: `{{ range service "webapp" "ns=namespace" }}{{ .Address }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewHealthConnectQueryV1("webapp", []string{"ns=namespace"})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.HealthService{
					{
						Node:      "node1",
						Address:   "1.2.3.4",
						Namespace: "namespace",
					},
					{
						Node:      "node2",
						Address:   "5.6.7.8",
						Namespace: "namespace",
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		}, {
			"func_connect",
			hcat.TemplateInput{
				Contents: `{{ range connect "webapp" "ns=namespace" }}{{ .Address }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewHealthConnectQueryV1("webapp", []string{"ns=namespace"})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.HealthService{
					{
						Node:      "node1",
						Address:   "1.2.3.4",
						Namespace: "namespace",
					},
					{
						Node:      "node2",
						Address:   "5.6.7.8",
						Namespace: "namespace",
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		}, {
			"func_node",
			hcat.TemplateInput{
				Contents: `{{ with node }}{{ .Node.Node }}{{ range .Services }}{{ .Service }}{{ end }}{{ end }}`,
			},
			fakeWatcher{nil},
			"",
			true,
		}, {
			"func_nodes",
			hcat.TemplateInput{
				Contents: `{{ range nodes }}{{ .Node }}{{ end }}`,
			},
			fakeWatcher{nil},
			"",
			true,
		}, {
			"func_services",
			hcat.TemplateInput{
				Contents: `{{ range services }}{{ .Name }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewCatalogServicesQueryV1([]string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.CatalogSnippet{
					{
						Name: "web",
						Tags: dep.ServiceTags([]string{"tag1", "tag2"}),
					},
					{
						Name: "api",
						Tags: dep.ServiceTags([]string{"tag3"}),
					},
				})
				return fakeWatcher{st}
			}(),
			"webapi",
			false,
		}, {
			"func_datacenters_v0",
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
			"func_keys",
			hcat.TemplateInput{
				Contents: `{{ range keys "key" }}{{ .Key }}:{{ .Value }};{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVListQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.KeyPair{
					{
						Key:   "key",
						Value: "value-1",
					},
					{
						Key:   "key/test",
						Value: "value-2",
					},
				})
				return fakeWatcher{st}
			}(),
			"key:value-1;key/test:value-2;",
			false,
		},
		{
			"func_key",
			hcat.TemplateInput{
				Contents: `{{ key "key" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVGetQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), dep.KvValue("test"))
				return fakeWatcher{st}
			}(),
			"test",
			false,
		},
		{
			"func_key_exists",
			hcat.TemplateInput{
				Contents: `{{ keyExists "key" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVExistsQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), dep.KVExists(true))
				return fakeWatcher{st}
			}(),
			"true",
			false,
		},
		{
			"func_key_exists_get",
			hcat.TemplateInput{
				Contents: `{{- with $kv := keyExistsGet "key" }}{{ .Key }}:{{ .Value }}{{- end}}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewKVExistsGetQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.KeyPair{
					Key:    "key",
					Value:  "value-1",
					Exists: true,
				})
				return fakeWatcher{st}
			}(),
			"key:value-1",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), testFunc(tc))
	}

}
