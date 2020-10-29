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
		i    hcat.Recaller
		e    string
		err  bool
	}{
		{
			"helper_explode",
			hcat.TemplateInput{
				Contents: `{{ range $k, $v := tree "list" | explode }}{{ $k }}{{ $v }}{{ end }}`,
			},
			func() *hcat.Store {
				st := hcat.NewStore()
				id := testKVListQueryID("list")
				st.Save(id, []*dep.KeyPair{
					{Key: "", Value: ""},
					{Key: "foo/bar", Value: "a"},
					{Key: "zip/zap", Value: "b"},
				})
				return st
			}(),
			"foomap[bar:a]zipmap[zap:b]",
			false,
		},
		{
			"helper_explodemap",
			hcat.TemplateInput{
				Contents: `{{ scratch.MapSet "explode-test" "foo/bar" "a"}}{{ scratch.MapSet "explode-test" "qux" "c"}}{{ scratch.MapSet "explode-test" "zip/zap" "d"}}{{ scratch.Get "explode-test" | explodeMap }}`,
			},
			hcat.NewStore(),
			"map[foo:map[bar:a] qux:c zip:map[zap:d]]",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := NewTemplate(tc.ti)

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
