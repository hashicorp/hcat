package hcat

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"text/template"
	"time"

	"github.com/hashicorp/consul/api"
	dep "github.com/hashicorp/hcat/internal/dependency"
)

func TestNewTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    *TemplateInput
		e    *Template
	}{
		{
			"nil",
			nil,
			NewTemplate(nil),
		},
		{
			"contents",
			&TemplateInput{
				Contents: "test",
			},
			&Template{
				contents: "test",
				hexMD5:   "098f6bcd4621d373cade4e832627b4f6",
			},
		},
		{
			"custom_delims",
			&TemplateInput{
				Contents:   "test",
				LeftDelim:  "<<",
				RightDelim: ">>",
			},
			&Template{
				contents:   "test",
				hexMD5:     "098f6bcd4621d373cade4e832627b4f6",
				leftDelim:  "<<",
				rightDelim: ">>",
			},
		},
		{
			"err_missing_key",
			&TemplateInput{
				Contents:      "test",
				ErrMissingKey: true,
			},
			&Template{
				contents:      "test",
				hexMD5:        "098f6bcd4621d373cade4e832627b4f6",
				errMissingKey: true,
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tmpl := NewTemplate(tc.i)
			if !reflect.DeepEqual(tc.e, tmpl) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, tmpl)
			}
		})
	}
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
		ti   *TemplateInput
		i    Recaller
		e    string
		err  bool
	}{
		{
			"nil",
			&TemplateInput{
				Contents: `test`,
			},
			nil,
			"test",
			false,
		},
		{
			"bad_func",
			&TemplateInput{
				Contents: `{{ bad_func }}`,
			},
			nil,
			"",
			true,
		},
		{
			"missing_deps",
			&TemplateInput{
				Contents: `{{ key "foo" }}`,
			},
			NewStore(),
			"",
			false,
		},

		// missing keys
		{
			"err_missing_keys__true",
			&TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: true,
			},
			nil,
			"",
			true,
		},
		{
			"err_missing_keys__false",
			&TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: false,
			},
			nil,
			"<no value>",
			false,
		},

		// funcs
		{
			"func_base64Decode",
			&TemplateInput{
				Contents: `{{ base64Decode "aGVsbG8=" }}`,
			},
			nil,
			"hello",
			false,
		},
		{
			"func_base64Decode_bad",
			&TemplateInput{
				Contents: `{{ base64Decode "aGVsxxbG8=" }}`,
			},
			nil,
			"",
			true,
		},
		{
			"func_base64Encode",
			&TemplateInput{
				Contents: `{{ base64Encode "hello" }}`,
			},
			nil,
			"aGVsbG8=",
			false,
		},
		{
			"func_base64URLDecode",
			&TemplateInput{
				Contents: `{{ base64URLDecode "dGVzdGluZzEyMw==" }}`,
			},
			nil,
			"testing123",
			false,
		},
		{
			"func_base64URLDecode_bad",
			&TemplateInput{
				Contents: `{{ base64URLDecode "aGVsxxbG8=" }}`,
			},
			nil,
			"",
			true,
		},
		{
			"func_base64URLEncode",
			&TemplateInput{
				Contents: `{{ base64URLEncode "testing123" }}`,
			},
			nil,
			"dGVzdGluZzEyMw==",
			false,
		},
		{
			"func_datacenters",
			&TemplateInput{
				Contents: `{{ datacenters }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewCatalogDatacentersQuery(false)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []string{"dc1", "dc2"})
				return st
			}(),
			"[dc1 dc2]",
			false,
		},
		{
			"func_datacenters_ignore",
			&TemplateInput{
				Contents: `{{ datacenters true }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewCatalogDatacentersQuery(true)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []string{"dc1", "dc2"})
				return st
			}(),
			"[dc1 dc2]",
			false,
		},
		{
			"func_file",
			&TemplateInput{
				Contents: `{{ file "/path/to/file" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewFileQuery("/path/to/file")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), "content")
				return st
			}(),
			"content",
			false,
		},
		{
			"func_key",
			&TemplateInput{
				Contents: `{{ key "key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				d.EnableBlocking()
				st.Save(d.String(), "5")
				return st
			}(),
			"5",
			false,
		},
		{
			"func_keyExists",
			&TemplateInput{
				Contents: `{{ keyExists "key" }} {{ keyExists "no_key" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), true)
				return st
			}(),
			"true false",
			false,
		},
		{
			"func_keyOrDefault",
			&TemplateInput{
				Contents: `{{ keyOrDefault "key" "100" }} {{ keyOrDefault "no_key" "200" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVGetQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), "150")
				return st
			}(),
			"150 200",
			false,
		},
		{
			"func_ls",
			&TemplateInput{
				Contents: `{{ range ls "list" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVListQuery("list")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.KeyPair{
					&dep.KeyPair{Key: "", Value: ""},
					&dep.KeyPair{Key: "foo", Value: "bar"},
					&dep.KeyPair{Key: "foo/zip", Value: "zap"},
				})
				return st
			}(),
			"foo=bar",
			false,
		},
		{
			"func_node",
			&TemplateInput{
				Contents: `{{ with node }}{{ .Node.Node }}{{ range .Services }}{{ .Service }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewCatalogNodeQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), &dep.CatalogNode{
					Node: &dep.Node{Node: "node1"},
					Services: []*dep.CatalogNodeService{
						&dep.CatalogNodeService{
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
			&TemplateInput{
				Contents: `{{ range nodes }}{{ .Node }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewCatalogNodesQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.Node{
					&dep.Node{Node: "node1"},
					&dep.Node{Node: "node2"},
				})
				return st
			}(),
			"node1node2",
			false,
		},
		{
			"func_secret_read",
			&TemplateInput{
				Contents: `{{ with secret "secret/foo" }}{{ .Data.zip }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), &dep.Secret{
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
			"func_secret_read_versions",
			&TemplateInput{
				Contents: `{{with secret "secret/foo"}}{{.Data.zip}}{{end}}:{{with secret "secret/foo?version=1"}}{{.Data.zip}}{{end}}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewVaultReadQuery("secret/foo")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), &dep.Secret{
					Data: map[string]interface{}{"zip": "zap"},
				})
				d1, err := dep.NewVaultReadQuery("secret/foo?version=1")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d1.String(), &dep.Secret{
					Data: map[string]interface{}{"zip": "zed"},
				})
				return st
			}(),
			"zap:zed",
			false,
		},
		{
			"func_secret_read_no_exist",
			&TemplateInput{
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
			&TemplateInput{
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
			&TemplateInput{
				Contents: `{{ with secret "transit/encrypt/foo" "plaintext=a" }}{{ .Data.ciphertext }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewVaultWriteQuery("transit/encrypt/foo", map[string]interface{}{
					"plaintext": "a",
				})
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), &dep.Secret{
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
			&TemplateInput{
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
			&TemplateInput{
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
			&TemplateInput{
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
			&TemplateInput{
				Contents: `{{ secrets "secret/" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewVaultListQuery("secret/")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []string{"bar", "foo"})
				return st
			}(),
			"[bar foo]",
			false,
		},
		{
			"func_secrets_no_exist",
			&TemplateInput{
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
			&TemplateInput{
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
			&TemplateInput{
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
			&TemplateInput{
				Contents: `{{ range service "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					&dep.HealthService{
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
			&TemplateInput{
				Contents: `{{ range service "webapp" "passing,any" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp|passing,any")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					&dep.HealthService{
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
			&TemplateInput{
				Contents: `{{ range services }}{{ .Name }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewCatalogServicesQuery("")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.CatalogSnippet{
					&dep.CatalogSnippet{
						Name: "service1",
					},
					&dep.CatalogSnippet{
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
			&TemplateInput{
				Contents: `{{ range tree "key" }}{{ .Key }}={{ .Value }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVListQuery("key")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.KeyPair{
					&dep.KeyPair{Key: "", Value: ""},
					&dep.KeyPair{Key: "admin/port", Value: "1134"},
					&dep.KeyPair{Key: "maxconns", Value: "5"},
					&dep.KeyPair{Key: "minconns", Value: "2"},
				})
				return st
			}(),
			"admin/port=1134maxconns=5minconns=2",
			false,
		},

		// scratch
		{
			"scratch.Key",
			&TemplateInput{
				Contents: `{{ scratch.Set "a" "2" }}{{ scratch.Key "a" }}`,
			},
			NewStore(),
			"true",
			false,
		},
		{
			"scratch.Get",
			&TemplateInput{
				Contents: `{{ scratch.Set "a" "2" }}{{ scratch.Get "a" }}`,
			},
			NewStore(),
			"2",
			false,
		},
		{
			"scratch.SetX",
			&TemplateInput{
				Contents: `{{ scratch.SetX "a" "2" }}{{ scratch.SetX "a" "1" }}{{ scratch.Get "a" }}`,
			},
			NewStore(),
			"2",
			false,
		},
		{
			"scratch.MapSet",
			&TemplateInput{
				Contents: `{{ scratch.MapSet "a" "foo" "bar" }}{{ scratch.MapValues "a" }}`,
			},
			NewStore(),
			"[bar]",
			false,
		},
		{
			"scratch.MapSetX",
			&TemplateInput{
				Contents: `{{ scratch.MapSetX "a" "foo" "bar" }}{{ scratch.MapSetX "a" "foo" "baz" }}{{ scratch.MapValues "a" }}`,
			},
			NewStore(),
			"[bar]",
			false,
		},

		// helpers
		{
			"helper_by_key",
			&TemplateInput{
				Contents: `{{ range $key, $pairs := tree "list" | byKey }}{{ $key }}:{{ range $pairs }}{{ .Key }}={{ .Value }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVListQuery("list")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.KeyPair{
					&dep.KeyPair{Key: "", Value: ""},
					&dep.KeyPair{Key: "foo/bar", Value: "a"},
					&dep.KeyPair{Key: "zip/zap", Value: "b"},
				})
				return st
			}(),
			"foo:bar=azip:zap=b",
			false,
		},
		{
			"helper_by_tag",
			&TemplateInput{
				Contents: `{{ range $tag, $services := service "webapp" | byTag }}{{ $tag }}:{{ range $services }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "staging"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"staging"},
					},
				})
				return st
			}(),
			"prod:1.2.3.4staging:1.2.3.45.6.7.8",
			false,
		},
		{
			"helper_contains",
			&TemplateInput{
				Contents: `{{ range service "webapp" }}{{ if .Tags | contains "prod" }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "staging"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"staging"},
					},
				})
				return st
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsAll",
			&TemplateInput{
				Contents: `{{ $requiredTags := parseJSON "[\"prod\",\"us-realm\"]" }}{{ range service "webapp" }}{{ if .Tags | containsAll $requiredTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "us-realm"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "ca-realm"},
					},
				})
				return st
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsAll__empty",
			&TemplateInput{
				Contents: `{{ $requiredTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsAll $requiredTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "us-realm"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "ca-realm"},
					},
				})
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"helper_containsAny",
			&TemplateInput{
				Contents: `{{ $acceptableTags := parseJSON "[\"v2\",\"v3\"]" }}{{ range service "webapp" }}{{ if .Tags | containsAny $acceptableTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "v2"},
					},
				})
				return st
			}(),
			"5.6.7.8",
			false,
		},
		{
			"helper_containsAny__empty",
			&TemplateInput{
				Contents: `{{ $acceptableTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsAny $acceptableTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "v2"},
					},
				})
				return st
			}(),
			"",
			false,
		},
		{
			"helper_containsNone",
			&TemplateInput{
				Contents: `{{ $forbiddenTags := parseJSON "[\"devel\",\"staging\"]" }}{{ range service "webapp" }}{{ if .Tags | containsNone $forbiddenTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"devel", "v2"},
					},
				})
				return st
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsNone__empty",
			&TemplateInput{
				Contents: `{{ $forbiddenTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsNone $forbiddenTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"staging", "v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"devel", "v2"},
					},
				})
				return st
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"helper_containsNotAll",
			&TemplateInput{
				Contents: `{{ $excludingTags := parseJSON "[\"es-v1\",\"es-v2\"]" }}{{ range service "webapp" }}{{ if .Tags | containsNotAll $excludingTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "es-v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "hybrid", "es-v1", "es-v2"},
					},
				})
				return st
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsNotAll__empty",
			&TemplateInput{
				Contents: `{{ $excludingTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsNotAll $excludingTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "es-v1"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "hybrid", "es-v1", "es-v2"},
					},
				})
				return st
			}(),
			"",
			false,
		},
		{
			"helper_env",
			&TemplateInput{
				// CT_TEST set above
				Contents: `{{ env "CT_TEST" }}`,
			},
			NewStore(),
			"1",
			false,
		},
		{
			"helper_executeTemplate",
			&TemplateInput{
				Contents: `{{ define "custom" }}{{ key "foo" }}{{ end }}{{ executeTemplate "custom" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVGetQuery("foo")
				if err != nil {
					t.Fatal(err)
				}
				d.EnableBlocking()
				st.Save(d.String(), "bar")
				return st
			}(),
			"bar",
			false,
		},
		{
			"helper_executeTemplate__dot",
			&TemplateInput{
				Contents: `{{ define "custom" }}{{ key . }}{{ end }}{{ executeTemplate "custom" "foo" }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVGetQuery("foo")
				if err != nil {
					t.Fatal(err)
				}
				d.EnableBlocking()
				st.Save(d.String(), "bar")
				return st
			}(),
			"bar",
			false,
		},
		{
			"helper_explode",
			&TemplateInput{
				Contents: `{{ range $k, $v := tree "list" | explode }}{{ $k }}{{ $v }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewKVListQuery("list")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.KeyPair{
					&dep.KeyPair{Key: "", Value: ""},
					&dep.KeyPair{Key: "foo/bar", Value: "a"},
					&dep.KeyPair{Key: "zip/zap", Value: "b"},
				})
				return st
			}(),
			"foomap[bar:a]zipmap[zap:b]",
			false,
		},
		{
			"helper_explodemap",
			&TemplateInput{
				Contents: `{{ scratch.MapSet "explode-test" "foo/bar" "a"}}{{ scratch.MapSet "explode-test" "qux" "c"}}{{ scratch.MapSet "explode-test" "zip/zap" "d"}}{{ scratch.Get "explode-test" | explodeMap }}`,
			},
			NewStore(),
			"map[foo:map[bar:a] qux:c zip:map[zap:d]]",
			false,
		},
		{
			"helper_in",
			&TemplateInput{
				Contents: `{{ range service "webapp" }}{{ if "prod" | in .Tags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthServiceQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "staging"},
					},
					&dep.HealthService{
						Address: "5.6.7.8",
						Tags:    []string{"staging"},
					},
				})
				return st
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_indent",
			&TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 4 }}`,
			},
			NewStore(),
			"    hello\n    hello\r\n    HELLO\r\n    hello\n    HELLO",
			false,
		},
		{
			"helper_indent_negative",
			&TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent -4 }}`,
			},
			NewStore(),
			"    hello\n    hello\r\n    HELLO\r\n    hello\n    HELLO",
			true,
		},
		{
			"helper_indent_zero",
			&TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 0 }}`,
			},
			NewStore(),
			"hello\nhello\r\nHELLO\r\nhello\nHELLO",
			false,
		},
		{
			"helper_loop",
			&TemplateInput{
				Contents: `{{ range loop 3 }}1{{ end }}`,
			},
			NewStore(),
			"111",
			false,
		},
		{
			"helper_loop__i",
			&TemplateInput{
				Contents: `{{ range $i := loop 3 }}{{ $i }}{{ end }}`,
			},
			NewStore(),
			"012",
			false,
		},
		{
			"helper_loop_start",
			&TemplateInput{
				Contents: `{{ range loop 1 3 }}1{{ end }}`,
			},
			NewStore(),
			"11",
			false,
		},
		{
			"helper_loop_text",
			&TemplateInput{
				Contents: `{{ range loop 1 "3" }}1{{ end }}`,
			},
			NewStore(),
			"11",
			false,
		},
		{
			"helper_loop_parseInt",
			&TemplateInput{
				Contents: `{{ $i := print "3" | parseInt }}{{ range loop 1 $i }}1{{ end }}`,
			},
			NewStore(),
			"11",
			false,
		},
		{
			// GH-1143
			"helper_loop_var",
			&TemplateInput{
				Contents: `{{$n := 3 }}` +
					`{{ range $i := loop $n }}{{ $i }}{{ end }}`,
			},
			NewStore(),
			"012",
			false,
		},
		{
			"helper_join",
			&TemplateInput{
				Contents: `{{ "a,b,c" | split "," | join ";" }}`,
			},
			NewStore(),
			"a;b;c",
			false,
		},
		{
			"helper_parseBool",
			&TemplateInput{
				Contents: `{{ "true" | parseBool }}`,
			},
			NewStore(),
			"true",
			false,
		},
		{
			"helper_parseFloat",
			&TemplateInput{
				Contents: `{{ "1.2" | parseFloat }}`,
			},
			NewStore(),
			"1.2",
			false,
		},
		{
			"helper_parseInt",
			&TemplateInput{
				Contents: `{{ "-1" | parseInt }}`,
			},
			NewStore(),
			"-1",
			false,
		},
		{
			"helper_parseJSON",
			&TemplateInput{
				Contents: `{{ "{\"foo\": \"bar\"}" | parseJSON }}`,
			},
			NewStore(),
			"map[foo:bar]",
			false,
		},
		{
			"helper_parseUint",
			&TemplateInput{
				Contents: `{{ "1" | parseUint }}`,
			},
			NewStore(),
			"1",
			false,
		},
		{
			"helper_parseYAML",
			&TemplateInput{
				Contents: `{{ "foo: bar" | parseYAML }}`,
			},
			NewStore(),
			"map[foo:bar]",
			false,
		},
		{
			"helper_parseYAMLv2",
			&TemplateInput{
				Contents: `{{ "foo: bar\nbaz: \"foo\"" | parseYAML }}`,
			},
			NewStore(),
			"map[baz:foo foo:bar]",
			false,
		},
		{
			"helper_parseYAMLnested",
			&TemplateInput{
				Contents: `{{ "foo:\n  bar: \"baz\"\n  baz: 7" | parseYAML }}`,
			},
			NewStore(),
			"map[foo:map[bar:baz baz:7]]",
			false,
		},
		{
			"helper_plugin",
			&TemplateInput{
				Contents: `{{ "1" | plugin "echo" }}`,
			},
			NewStore(),
			"1",
			false,
		},
		{
			"helper_plugin_disabled",
			&TemplateInput{
				Contents:     `{{ "1" | plugin "echo" }}`,
				FuncMapMerge: template.FuncMap{"plugin": DenyFunc},
			},
			NewStore(),
			"",
			true,
		},
		{
			"helper_regexMatch",
			&TemplateInput{
				Contents: `{{ "foo" | regexMatch "[a-z]+" }}`,
			},
			NewStore(),
			"true",
			false,
		},
		{
			"helper_regexReplaceAll",
			&TemplateInput{
				Contents: `{{ "foo" | regexReplaceAll "\\w" "x" }}`,
			},
			NewStore(),
			"xxx",
			false,
		},
		{
			"helper_replaceAll",
			&TemplateInput{
				Contents: `{{ "hello my hello" | regexReplaceAll "hello" "bye" }}`,
			},
			NewStore(),
			"bye my bye",
			false,
		},
		{
			"helper_split",
			&TemplateInput{
				Contents: `{{ "a,b,c" | split "," }}`,
			},
			NewStore(),
			"[a b c]",
			false,
		},
		{
			"helper_timestamp",
			&TemplateInput{
				Contents: `{{ timestamp }}`,
			},
			NewStore(),
			"1970-01-01T00:00:00Z",
			false,
		},
		{
			"helper_helper_timestamp__formatted",
			&TemplateInput{
				Contents: `{{ timestamp "2006-01-02" }}`,
			},
			NewStore(),
			"1970-01-01",
			false,
		},
		{
			"helper_toJSON",
			&TemplateInput{
				Contents: `{{ "a,b,c" | split "," | toJSON }}`,
			},
			NewStore(),
			"[\"a\",\"b\",\"c\"]",
			false,
		},
		{
			"helper_toLower",
			&TemplateInput{
				Contents: `{{ "HI" | toLower }}`,
			},
			NewStore(),
			"hi",
			false,
		},
		{
			"helper_toTitle",
			&TemplateInput{
				Contents: `{{ "this is a sentence" | toTitle }}`,
			},
			NewStore(),
			"This Is A Sentence",
			false,
		},
		{
			"helper_toTOML",
			&TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toTOML }}`,
			},
			NewStore(),
			"foo = \"bar\"",
			false,
		},
		{
			"helper_toUpper",
			&TemplateInput{
				Contents: `{{ "hi" | toUpper }}`,
			},
			NewStore(),
			"HI",
			false,
		},
		{
			"helper_toYAML",
			&TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toYAML }}`,
			},
			NewStore(),
			"foo: bar",
			false,
		},
		{
			"helper_trimSpace",
			&TemplateInput{
				Contents: `{{ "\t hi\n " | trimSpace }}`,
			},
			NewStore(),
			"hi",
			false,
		},
		{
			"helper_sockaddr",
			&TemplateInput{
				Contents: `{{ sockaddr "GetAllInterfaces | include \"flag\" \"loopback\" | include \"type\" \"IPv4\" | sort \"address\" | limit 1 | attr \"address\""}}`,
			},
			NewStore(),
			"127.0.0.1",
			false,
		},
		{
			"math_add",
			&TemplateInput{
				Contents: `{{ 2 | add 2 }}`,
			},
			NewStore(),
			"4",
			false,
		},
		{
			"math_subtract",
			&TemplateInput{
				Contents: `{{ 2 | subtract 2 }}`,
			},
			NewStore(),
			"0",
			false,
		},
		{
			"math_multiply",
			&TemplateInput{
				Contents: `{{ 2 | multiply 2 }}`,
			},
			NewStore(),
			"4",
			false,
		},
		{
			"math_divide",
			&TemplateInput{
				Contents: `{{ 2 | divide 2 }}`,
			},
			NewStore(),
			"1",
			false,
		},
		{
			"math_modulo",
			&TemplateInput{
				Contents: `{{ 3 | modulo 2 }}`,
			},
			NewStore(),
			"1",
			false,
		},
		{
			"math_minimum",
			&TemplateInput{
				Contents: `{{ 3 | minimum 2 }}`,
			},
			NewStore(),
			"2",
			false,
		},
		{
			"math_maximum",
			&TemplateInput{
				Contents: `{{ 3 | maximum 2 }}`,
			},
			NewStore(),
			"3",
			false,
		},
		{
			"leaf_cert",
			&TemplateInput{
				Contents: `{{with caLeaf "foo"}}` +
					`{{.CertPEM}}{{.PrivateKeyPEM}}{{end}}`,
			},
			func() *Store {
				d := dep.NewConnectLeafQuery("foo")
				st := NewStore()
				st.Save(d.String(), &api.LeafCert{
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
			&TemplateInput{
				Contents: `{{range caRoots}}{{.RootCertPEM}}{{end}}`,
			},
			func() *Store {
				d := dep.NewConnectCAQuery()
				st := NewStore()
				st.Save(d.String(), []*api.CARoot{
					&api.CARoot{
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
			&TemplateInput{
				Contents: `{{ range connect "webapp" }}{{ .Address }}{{ end }}`,
			},
			func() *Store {
				st := NewStore()
				d, err := dep.NewHealthConnectQuery("webapp")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.String(), []*dep.HealthService{
					&dep.HealthService{
						Node:    "node1",
						Address: "1.2.3.4",
					},
					&dep.HealthService{
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
	//		ti   *TemplateInput
	//		i    Recaller
	//		e    string
	//		err  bool
	//	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := NewTemplate(tc.ti)
			if err != nil {
				t.Fatal(err)
			}

			a, err := tpl.Execute(tc.i)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if a != nil && !bytes.Equal([]byte(tc.e), a.Output) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a.Output))
			}
		})
	}
}
