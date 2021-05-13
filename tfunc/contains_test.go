package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func TestContainsExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"helper_contains",
			hcat.TemplateInput{
				Contents: `{{ range service "webapp" }}{{ if .Tags | contains "prod" }}{{ .Address }}{{ end }}{{ end }}`,
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
			"1.2.3.4",
			false,
		},
		{
			"helper_containsAll",
			hcat.TemplateInput{
				Contents: `{{ $requiredTags := parseJSON "[\"prod\",\"us-realm\"]" }}{{ range service "webapp" }}{{ if .Tags | containsAll $requiredTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "us-realm"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "ca-realm"},
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsAll__empty",
			hcat.TemplateInput{
				Contents: `{{ $requiredTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsAll $requiredTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "us-realm"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "ca-realm"},
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"helper_containsAny",
			hcat.TemplateInput{
				Contents: `{{ $acceptableTags := parseJSON "[\"v2\",\"v3\"]" }}{{ range service "webapp" }}{{ if .Tags | containsAny $acceptableTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"5.6.7.8",
			false,
		},
		{
			"helper_containsAny__empty",
			hcat.TemplateInput{
				Contents: `{{ $acceptableTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsAny $acceptableTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"",
			false,
		},
		{
			"helper_containsNone",
			hcat.TemplateInput{
				Contents: `{{ $forbiddenTags := parseJSON "[\"devel\",\"staging\"]" }}{{ range service "webapp" }}{{ if .Tags | containsNone $forbiddenTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"devel", "v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsNone__empty",
			hcat.TemplateInput{
				Contents: `{{ $forbiddenTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsNone $forbiddenTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"staging", "v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"devel", "v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.45.6.7.8",
			false,
		},
		{
			"helper_containsNotAll",
			hcat.TemplateInput{
				Contents: `{{ $excludingTags := parseJSON "[\"es-v1\",\"es-v2\"]" }}{{ range service "webapp" }}{{ if .Tags | containsNotAll $excludingTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "es-v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "hybrid", "es-v1", "es-v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"1.2.3.4",
			false,
		},
		{
			"helper_containsNotAll__empty",
			hcat.TemplateInput{
				Contents: `{{ $excludingTags := parseJSON "[]" }}{{ range service "webapp" }}{{ if .Tags | containsNotAll $excludingTags }}{{ .Address }}{{ end }}{{ end }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testHealthServiceQueryID("webapp")
				st.Save(id, []*dep.HealthService{
					{
						Address: "1.2.3.4",
						Tags:    []string{"prod", "es-v1"},
					},
					{
						Address: "5.6.7.8",
						Tags:    []string{"prod", "hybrid", "es-v1", "es-v2"},
					},
				})
				return fakeWatcher{st}
			}(),
			"",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := NewTemplate(tc.ti)

			a, err := tpl.Execute(tc.i.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		})
	}
}
