package tfunc

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func Test_byMeta(t *testing.T) {
	t.Parallel()
	svcA := &dep.HealthService{
		ServiceMeta: map[string]string{
			"version":         "v2",
			"version_num":     "2",
			"bad_version_num": "1zz",
			"env":             "dev",
		},
		ID: "svcA",
	}

	svcB := &dep.HealthService{
		ServiceMeta: map[string]string{
			"version":         "v11",
			"version_num":     "11",
			"bad_version_num": "1zz",
			"env":             "prod",
		},
		ID: "svcB",
	}

	svcC := &dep.HealthService{
		ServiceMeta: map[string]string{
			"version":         "v11",
			"version_num":     "11",
			"bad_version_num": "1zz",
			"env":             "prod",
		},
		ID: "svcC",
	}

	type args struct {
		meta     string
		services []*dep.HealthService
	}

	tests := []struct {
		name       string
		args       args
		wantGroups map[string][]*dep.HealthService
		wantErr    bool
	}{
		{
			name: "version string",
			args: args{
				meta:     "version",
				services: []*dep.HealthService{svcA, svcB, svcC},
			},
			wantGroups: map[string][]*dep.HealthService{
				"v11": {svcB, svcC},
				"v2":  {svcA},
			},
			wantErr: false,
		},
		{
			name: "version number",
			args: args{
				meta:     "version_num|int",
				services: []*dep.HealthService{svcA, svcB, svcC},
			},
			wantGroups: map[string][]*dep.HealthService{
				"00011": {svcB, svcC},
				"00002": {svcA},
			},
			wantErr: false,
		},
		{
			name: "bad version number",
			args: args{
				meta:     "bad_version_num|int",
				services: []*dep.HealthService{svcA, svcB, svcC},
			},
			wantGroups: nil,
			wantErr:    true,
		},
		{
			name: "multiple meta",
			args: args{
				meta:     "env,version_num|int,version",
				services: []*dep.HealthService{svcA, svcB, svcC},
			},
			wantGroups: map[string][]*dep.HealthService{
				"dev_00002_v2":   {svcA},
				"prod_00011_v11": {svcB, svcC},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGroups, err := byMeta(tt.args.meta, tt.args.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("byMeta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			onlyIDs := func(groups map[string][]*dep.HealthService) (ids map[string]map[string]int) {
				ids = make(map[string]map[string]int)
				for group, svcs := range groups {
					ids[group] = make(map[string]int)
					for _, svc := range svcs {
						ids[group][svc.ID] = 1
					}
				}
				return
			}

			gotIDs := onlyIDs(gotGroups)
			wantIDs := onlyIDs(tt.wantGroups)
			if !reflect.DeepEqual(gotGroups, tt.wantGroups) {
				t.Errorf("byMeta() = %v, want %v", gotIDs, wantIDs)
			}
		})
	}
}

func TestConsulExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
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

type fakeWatcher struct {
	*hcat.Store
}

func (fakeWatcher) Buffer(hcat.Notifier) bool     { return false }
func (f fakeWatcher) Complete(hcat.Notifier) bool { return true }
func (f fakeWatcher) Recaller(hcat.Notifier) hcat.Recaller {
	return func(d dep.Dependency) (value interface{}, found bool) {
		return f.Store.Recall(d.ID())
	}
}
