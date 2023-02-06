// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestVaultExecute(t *testing.T) {
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
			tpl := newTemplate(tc.ti)

			a, err := tpl.Execute(tc.i.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		}
	}

	cases := []testCase{
		{
			"func_secret_read",
			hcat.TemplateInput{
				Contents: `{{ with secret "secret/foo" }}{{ .Data.zip }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
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
				return fakeWatcher{st}
			}(),
			"zap",
			false,
		},
		{
			"func_secret_read_dash_error",
			hcat.TemplateInput{
				// the dash "-" in "zip-zap" triggers a template parsing error
				// see next entry for test of workaround
				Contents: `{{ with secret "secret/foo" }}{{ .Data.zip-zap }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
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
				return fakeWatcher{st}
			}(),
			"",
			true,
		},
		{
			"func_secret_read_dash_error_workaround",
			hcat.TemplateInput{
				Contents: `{{ with secret "secret/foo" }}{{ index .Data "zip-zap" }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
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
				return fakeWatcher{st}
			}(),
			"zoom",
			false,
		},
		{
			"func_secret_read_versions",
			hcat.TemplateInput{
				Contents: `{{with secret "secret/foo"}}{{.Data.zip}}{{end}}:{{with secret "secret/foo?version=1"}}{{.Data.zip}}{{end}}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
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
				return fakeWatcher{st}
			}(),
			"zap:zed",
			false,
		},
		{
			"func_secret_read_no_exist",
			hcat.TemplateInput{
				Contents: `{{ with secret "secret/nope" }}{{ .Data.zip }}{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"",
			false,
		},
		{
			"func_secret_read_no_exist_falsey",
			hcat.TemplateInput{
				Contents: `{{ if secret "secret/nope" }}yes{{ else }}no{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"no",
			false,
		},
		{
			"func_secret_write",
			hcat.TemplateInput{
				Contents: `{{ with secret "transit/encrypt/foo" "plaintext=a" }}{{ .Data.ciphertext }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
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
				return fakeWatcher{st}
			}(),
			"encrypted",
			false,
		},
		{
			"func_secret_write_empty",
			hcat.TemplateInput{
				Contents: `{{ with secret "transit/encrypt/foo" "" }}{{ .Data.ciphertext }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewVaultWriteQuery("transit/encrypt/foo", nil)
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), &dep.Secret{
					Data: map[string]interface{}{"ciphertext": "encrypted"},
				})
				return fakeWatcher{st}
			}(),
			"encrypted",
			false,
		},
		{
			"func_secret_write_no_exist",
			hcat.TemplateInput{
				Contents: `{{ with secret "secret/nope" "a=b" }}{{ .Data.zip }}{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"",
			false,
		},
		{
			"func_secret_write_no_exist_falsey",
			hcat.TemplateInput{
				Contents: `{{ if secret "secret/nope" "a=b" }}yes{{ else }}no{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"no",
			false,
		},
		{
			"func_secret_no_exist_falsey_with",
			hcat.TemplateInput{
				Contents: `{{ with secret "secret/nope" }}{{ .Data.foo.bar }}{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"",
			false,
		},
		{
			"func_secrets",
			hcat.TemplateInput{
				Contents: `{{ secrets "secret/" }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				d, err := idep.NewVaultListQuery("secret/")
				if err != nil {
					t.Fatal(err)
				}
				st.Save(d.ID(), []string{"bar", "foo"})
				return fakeWatcher{st}
			}(),
			"[bar foo]",
			false,
		},
		{
			"func_secrets_no_exist",
			hcat.TemplateInput{
				Contents: `{{ secrets "secret/" }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"[]",
			false,
		},
		{
			"func_secrets_no_exist_falsey",
			hcat.TemplateInput{
				Contents: `{{ if secrets "secret/" }}yes{{ else }}no{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"no",
			false,
		},
		{
			"func_secrets_no_exist_falsey_with",
			hcat.TemplateInput{
				Contents: `{{ with secrets "secret/" }}{{ . }}{{ end }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"",
			false,
		},
		{
			"func_secret_nil_pointer_evaluation",
			hcat.TemplateInput{
				Contents: `{{ $v := secret "secret/foo" }}{{ $v.Data.zip }}`,
			},
			func() hcat.Watcherer {
				return fakeWatcher{hcat.NewStore()}
			}(),
			"<no value>",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), testFunc(tc))
	}
}
