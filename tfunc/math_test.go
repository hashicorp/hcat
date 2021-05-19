package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
)

func TestMathExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"math_add",
			hcat.TemplateInput{
				Contents: `{{ 2 | add 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"4",
			false,
		},
		{
			"math_subtract",
			hcat.TemplateInput{
				Contents: `{{ 2 | subtract 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"0",
			false,
		},
		{
			"math_multiply",
			hcat.TemplateInput{
				Contents: `{{ 2 | multiply 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"4",
			false,
		},
		{
			"math_divide",
			hcat.TemplateInput{
				Contents: `{{ 2 | divide 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"1",
			false,
		},
		{
			"math_modulo",
			hcat.TemplateInput{
				Contents: `{{ 3 | modulo 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"1",
			false,
		},
		{
			"math_minimum",
			hcat.TemplateInput{
				Contents: `{{ 3 | minimum 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"2",
			false,
		},
		{
			"math_maximum",
			hcat.TemplateInput{
				Contents: `{{ 3 | maximum 2 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"3",
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
