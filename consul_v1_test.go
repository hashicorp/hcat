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
				st.Save(d.String(), []*dep.HealthService{
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
				st.Save(d.String(), []*dep.HealthService{
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
			nil,
			"",
			true,
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
				st.Save(d.String(), []string{"dc1", "dc2"})
				return st
			}(),
			"[dc1 dc2]",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.ti.FuncMapMerge = FuncMapConsulV1()
			tpl := NewTemplate(tc.ti)

			w := fakeWatcher{tc.i}
			a, err := tpl.Execute(w)
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
