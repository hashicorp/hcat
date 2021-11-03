package hcat

import (
	"testing"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
	"github.com/stretchr/testify/assert"
)

func TestTemplateExecute_consul_v1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   TemplateInput
		i    *Store
		e    string
		err  bool
	}{
		{
			"func_service",
			TemplateInput{
				Contents: `{{ range service "webapp" "ns=namespace" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		}, {
			"func_connect",
			TemplateInput{
				Contents: `{{ range connect "webapp" "ns=namespace" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		}, {
			"func_node",
			TemplateInput{
				Contents: `{{ with node }}{{ .Node.Node }}{{ range .Services }}{{ .Service }}{{ end }}{{ end }}`,
			},
			nil,
			"",
			true,
		}, {
			"func_nodes",
			TemplateInput{
				Contents: `{{ range nodes }}{{ .Node }}{{ end }}`,
			},
			nil,
			"",
			true,
		}, {
			"func_services",
			TemplateInput{
				Contents: `{{ range services }}{{ .Name }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"webapi",
			false,
		}, {
			"func_datacenters_v0",
			TemplateInput{
				Contents: `{{ datacenters }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewCatalogDatacentersQuery(false)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []string{"dc1", "dc2"})
				return st
			}(),
			"[dc1 dc2]",
			false,
		},
		{
			"func_keys",
			TemplateInput{
				Contents: `{{ range keys "key" }}{{ .Key }}:{{ .Value }};{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"key:value-1;key/test:value-2;",
			false,
		},
		{
			"func_key",
			TemplateInput{
				Contents: `{{ key "key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVGetQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), dep.KvValue("test"))
				return st
			}(),
			"test",
			false,
		},
		{
			"func_key_exists",
			TemplateInput{
				Contents: `{{ keyExists "key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVExistsQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), dep.KVExists(true))
				return st
			}(),
			"true",
			false,
		},
		{
			"func_key_exists_get",
			TemplateInput{
				Contents: `{{- with $kv := keyExistsGet "key" }}{{ .Key }}:{{ .Value }}{{- end}}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVExistsGetQueryV1("key", []string{})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.KeyPair{
					Key:    "key",
					Value:  "value-1",
					Exists: true,
				})
				return st
			}(),
			"key:value-1",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.ti.FuncMapMerge = FuncMapConsulV1()
			tpl := NewTemplate(tc.ti)

			w := fakeWatcher{tc.i}
			a, err := tpl.Execute(w.Recaller(tpl))
			if tc.err {
				assert.Error(t, err, "expected: funcNotImplementedError")
				assert.Contains(t, err.Error(), errFuncNotImplemented.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.e, string(a))
		})
	}
}
