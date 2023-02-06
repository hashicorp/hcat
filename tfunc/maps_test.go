// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func TestMapExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"helper_explode",
			hcat.TemplateInput{
				Contents: `{{ range $k, $v := tree "list" | explode }}{{ $k }}{{ $v }}{{ end }}`,
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
			"foomap[bar:a]zipmap[zap:b]",
			false,
		},
		{
			"helper_explodemap",
			hcat.TemplateInput{
				Contents: `{{ testMap | explodeMap }}`,
				FuncMapMerge: map[string]interface{}{
					"testMap": func() map[string]interface{} {
						m := make(map[string]interface{})
						m["foo"] = map[string]string{"bar": "a"}
						m["qux"] = "c"
						m["zip"] = map[string]string{"zap": "d"}
						return m
					},
				},
			},
			fakeWatcher{hcat.NewStore()},
			"map[foo:map[bar:a] qux:c zip:map[zap:d]]",
			false,
		},
		{
			"helper_mergeMap",
			hcat.TemplateInput{
				Contents: `{{ $base := "{\"voo\":{\"bar\":\"v\"}}" | parseJSON}}{{ $role := tree "list" | explode | mergeMap $base}}{{ range $k, $v := $role }}{{ $k }}{{ $v }}{{ end }}`,
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
			"foomap[bar:a]voomap[bar:v]zipmap[zap:b]",
			false,
		},
		{
			"helper_mergeMapWithOverride",
			hcat.TemplateInput{
				Contents: `{{ $base := "{\"zip\":{\"zap\":\"t\"},\"voo\":{\"bar\":\"v\"}}" | parseJSON}}{{ $role := tree "list" | explode | mergeMapWithOverride $base}}{{ range $k, $v := $role }}{{ $k }}{{ $v }}{{ end }}`,
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
			"foomap[bar:a]voomap[bar:v]zipmap[zap:b]",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := newTemplate(tc.ti)

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
