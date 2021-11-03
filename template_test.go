package hcat

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestNewTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    TemplateInput
		e    *Template
	}{
		{
			"nil",
			TemplateInput{Name: "nil"},
			NewTemplate(TemplateInput{Name: "nil"}),
		},
		{
			"contents",
			TemplateInput{
				Name:     "test",
				Contents: "test",
			},
			&Template{
				name:     "test",
				contents: "test",
				hexMD5:   "098f6bcd4621d373cade4e832627b4f6",
			},
		},
		{
			"custom_delims",
			TemplateInput{
				Name:       "test",
				Contents:   "test",
				LeftDelim:  "<<",
				RightDelim: ">>",
			},
			&Template{
				name:       "test",
				contents:   "test",
				hexMD5:     "098f6bcd4621d373cade4e832627b4f6",
				leftDelim:  "<<",
				rightDelim: ">>",
			},
		},
		{
			"err_missing_key",
			TemplateInput{
				Name:          "test",
				Contents:      "test",
				ErrMissingKey: true,
			},
			&Template{
				name:          "test",
				contents:      "test",
				hexMD5:        "098f6bcd4621d373cade4e832627b4f6",
				errMissingKey: true,
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tmpl := NewTemplate(tc.i)
			tc.e.dirty, tmpl.dirty = nil, nil // don't compare well
			if !reflect.DeepEqual(tc.e, tmpl) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, tmpl)
			}
		})
	}

	t.Run("explicit_name_checks", func(t *testing.T) {
		contentsMD5 := "098f6bcd4621d373cade4e832627b4f6"

		tmpl := NewTemplate(
			TemplateInput{
				Name:     "foo",
				Contents: "test",
			})
		if tmpl.ID() != (contentsMD5 + "_foo") {
			t.Fatalf("ID is wrong, got '%s', want '%s'\n", tmpl.ID(),
				contentsMD5+"_foo")
		}

		tmpl = NewTemplate(
			TemplateInput{
				Contents: "test",
			})
		if tmpl.ID() != contentsMD5 {
			t.Fatalf("ID is wrong, got '%s', want '%s'\n", tmpl.ID(), contentsMD5)
		}

	})
}

func TestTemplate_Execute(t *testing.T) {
	t.Parallel()
	now = func() time.Time { return time.Unix(0, 0).UTC() }

	// set an environment variable for the tests
	if err := os.Setenv("CT_TEST", "1"); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Unsetenv("CT_TEST") }()

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("test")
	defer os.Remove(f.Name())

	cases := []struct {
		name string
		ti   TemplateInput
		i    *Store
		e    string
		err  bool
	}{
		{
			"nil",
			TemplateInput{
				Contents: `test`,
			},
			nil,
			"test",
			false,
		},
		{
			"bad_func",
			TemplateInput{
				Contents: `{{ bad_func }}`,
			},
			nil,
			"",
			true,
		},
		{
			"missing_deps",
			TemplateInput{
				Contents: `{{ key "foo" }}`,
			},
			NewStore(),
			"",
			false,
		},

		// missing keys
		{
			"err_missing_keys__true",
			TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: true,
			},
			nil,
			"",
			true,
		},
		{
			"err_missing_keys__false",
			TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: false,
			},
			nil,
			"<no value>",
			false,
		},
		{
			"func_datacenters",
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
			"func_datacenters_ignore",
			TemplateInput{
				Contents: `{{ datacenters true }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewCatalogDatacentersQuery(true)
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
			"func_key",
			TemplateInput{
				Contents: `{{ key "key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), "5")
				return st
			}(),
			"5",
			false,
		},
		{
			"func_keyExists",
			TemplateInput{
				Contents: `{{ keyExists "key" }} {{ keyExists "no_key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), true)
				return st
			}(),
			"true false",
			false,
		},
		{
			"func_keyOrDefault",
			TemplateInput{
				Contents: `{{ keyOrDefault "key" "100" }} {{ keyOrDefault "no_key" "200" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), "150")
				return st
			}(),
			"150 200",
			false,
		},
		{
			"func_ls",
			TemplateInput{
				Contents: `{{ range ls "list" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewKVListQuery("list")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.KeyPair{
					{Key: "", Value: ""},
					{Key: "foo", Value: "bar"},
					{Key: "foo/zip", Value: "zap"},
				})
				return st
			}(),
			"foo=bar",
			false,
		},
		{
			"func_node",
			TemplateInput{
				Contents: `{{ with node }}{{ .Node.Node }}{{ range .Services }}{{ .Service }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"node1service1",
			false,
		},
		{
			"func_nodes",
			TemplateInput{
				Contents: `{{ range nodes }}{{ .Node }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewCatalogNodesQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []*dep.Node{
					{Node: "node1"},
					{Node: "node2"},
				})
				return st
			}(),
			"node1node2",
			false,
		},
		{
			"func_secret_read",
			TemplateInput{
				Contents: `{{ with secret "secret/foo" }}{{ .Data.zip }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					LeaseID:       "abcd1234",
					LeaseDuration: 120,
					Renewable:     true,
					Data:          map[string]interface{}{"zip": "zap"},
				})
				return st
			}(),
			"zap",
			false,
		},
		{
			"func_secret_read_dash_error",
			TemplateInput{
				// the dash "-" in "zip-zap" triggers a template parsing error
				// see next entry for test of workaround
				Contents: `{{ with secret "secret/foo" }}{{ .Data.zip-zap }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					LeaseID:       "abcd1234",
					LeaseDuration: 120,
					Renewable:     true,
					Data:          map[string]interface{}{"zip-zap": "zoom"},
				})
				return st
			}(),
			"",
			true,
		},
		{
			"func_secret_read_dash_error_workaround",
			TemplateInput{
				Contents: `{{ with secret "secret/foo" }}{{ index .Data "zip-zap" }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					LeaseID:       "abcd1234",
					LeaseDuration: 120,
					Renewable:     true,
					Data:          map[string]interface{}{"zip-zap": "zoom"},
				})
				return st
			}(),
			"zoom",
			false,
		},
		{
			"func_secret_read_versions",
			TemplateInput{
				Contents: `{{with secret "secret/foo"}}{{.Data.zip}}{{end}}:{{with secret "secret/foo?version=1"}}{{.Data.zip}}{{end}}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					Data: map[string]interface{}{"zip": "zap"},
				})
				d1, err := idep.NewVaultReadQuery("secret/foo?version=1")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d1.ID(), &dep.Secret{
					Data: map[string]interface{}{"zip": "zed"},
				})
				return st
			}(),
			"zap:zed",
			false,
		},
		{
			"func_secret_read_no_exist",
			TemplateInput{
				Contents: `{{ with secret "secret/nope" }}{{ .Data.zip }}{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"",
			false,
		},
		{
			"func_secret_read_no_exist_falsey",
			TemplateInput{
				Contents: `{{ if secret "secret/nope" }}yes{{ else }}no{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"no",
			false,
		},
		{
			"func_secret_write",
			TemplateInput{
				Contents: `{{ with secret "transit/encrypt/foo" "plaintext=a" }}{{ .Data.ciphertext }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultWriteQuery("transit/encrypt/foo", map[string]interface{}{
					"plaintext": "a",
				})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					LeaseID:       "abcd1234",
					LeaseDuration: 120,
					Renewable:     true,
					Data:          map[string]interface{}{"ciphertext": "encrypted"},
				})
				return st
			}(),
			"encrypted",
			false,
		},
		{
			"func_secret_write_no_exist",
			TemplateInput{
				Contents: `{{ with secret "secret/nope" "a=b" }}{{ .Data.zip }}{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"",
			false,
		},
		{
			"func_secret_write_no_exist_falsey",
			TemplateInput{
				Contents: `{{ if secret "secret/nope" "a=b" }}yes{{ else }}no{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"no",
			false,
		},
		{
			"func_secret_no_exist_falsey_with",
			TemplateInput{
				Contents: `{{ with secret "secret/nope" }}{{ .Data.foo.bar }}{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"",
			false,
		},
		{
			"func_secrets",
			TemplateInput{
				Contents: `{{ secrets "secret/" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := idep.NewVaultListQuery("secret/")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []string{"bar", "foo"})
				return st
			}(),
			"[bar foo]",
			false,
		},
		{
			"func_secrets_no_exist",
			TemplateInput{
				Contents: `{{ secrets "secret/" }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"[]",
			false,
		},
		{
			"func_secrets_no_exist_falsey",
			TemplateInput{
				Contents: `{{ if secrets "secret/" }}yes{{ else }}no{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"no",
			false,
		},
		{
			"func_secrets_no_exist_falsey_with",
			TemplateInput{
				Contents: `{{ with secrets "secret/" }}{{ . }}{{ end }}`,
			},
			func() *Store {
				return NewStore()
			}(),
			"",
			false,
		},
		{
			"func_service",
			TemplateInput{
				Contents: `{{ range service "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"func_service_filter",
			TemplateInput{
				Contents: `{{ range service "webapp" "passing,any" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"func_services",
			TemplateInput{
				Contents: `{{ range services }}{{ .Name }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"service1service2",
			false,
		},
		{
			"func_tree",
			TemplateInput{
				Contents: `{{ range tree "key" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"admin/port=1134maxconns=5minconns=2",
			false,
		},
		{
			"leaf_cert",
			TemplateInput{
				Contents: `{{with caLeaf "foo"}}` +
					`{{.CertPEM}}{{.PrivateKeyPEM}}{{end}}`,
			},
			func() *Store {
				d := idep.NewConnectLeafQuery("foo")
				st := NewStore()
				st.Save(d.ID(), &api.LeafCert{
					Service:       "foo",
					CertPEM:       "PEM",
					PrivateKeyPEM: "KEY",
				})
				return st
			}(),
			"PEMKEY",
			false,
		},
		{
			"root_ca",
			TemplateInput{
				Contents: `{{range caRoots}}{{.RootCertPEM}}{{end}}`,
			},
			func() *Store {
				d := idep.NewConnectCAQuery()
				st := NewStore()
				st.Save(d.ID(), []*api.CARoot{
					{
						Name:        "Consul CA Root Cert",
						RootCertPEM: "PEM",
						Active:      true,
					},
				})
				return st
			}(),
			"PEM",
			false,
		},
		{
			"func_connect",
			TemplateInput{
				Contents: `{{ range connect "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
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
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
	}

	//	struct {
	//		name string
	//		ti   TemplateInput
	//		i    Recaller
	//		e    string
	//		err  bool
	//	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := NewTemplate(tc.ti)

			w := fakeWatcher{tc.i}
			a, err := tpl.Execute(w.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		})
	}
}

func TestCachedTemplate(t *testing.T) {
	t.Run("cache-is-used", func(t *testing.T) {
		ti := TemplateInput{
			Contents: `{{ key "key" }}`,
		}
		rec := func() *Store {
			st := NewStore()
			d, err := idep.NewKVGetQuery("key")
			if err != nil {
				t.Fatal(err)
			}
			st.Save(d.ID(), "value")
			return st
		}()
		tpl := NewTemplate(ti)
		w := fakeWatcher{rec}
		content, err := tpl.Execute(w.Recaller(tpl))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, []byte("value")) {
			t.Fatal("bad content:", string(content))
		}
		// Cache used here
		content, err = tpl.Execute(w.Recaller(tpl))
		if err != ErrNoNewValues {
			t.Fatal("error should be ErrNoNewValues")
		}
		if !bytes.Equal(content, []byte("value")) {
			t.Fatal("bad content:", string(content))
		}
	})
}

type fakeWatcher struct {
	*Store
}

func (fakeWatcher) Buffer(string) bool       { return false }
func (f fakeWatcher) Complete(Notifier) bool { return true }
func (f fakeWatcher) Mark(Notifier)          {}
func (f fakeWatcher) Sweep(Notifier)         {}
func (f fakeWatcher) Recaller(Notifier) Recaller {
	return func(d dep.Dependency) (value interface{}, found bool) {
		return f.Store.Recall(d.ID())
	}
}
