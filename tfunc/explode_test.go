package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func TestExplodeExecute(t *testing.T) {
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
