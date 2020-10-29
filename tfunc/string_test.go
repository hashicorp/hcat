package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
)

func TestStringExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Recaller
		e    string
		err  bool
	}{
		{
			"indent",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 4 }}`,
			},
			hcat.NewStore(),
			"    hello\n    hello\r\n    HELLO\r\n    hello\n    HELLO",
			false,
		},
		{
			"indent_negative",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent -4 }}`,
			},
			hcat.NewStore(),
			"    hello\n    hello\r\n    HELLO\r\n    hello\n    HELLO",
			true,
		},
		{
			"indent_zero",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 0 }}`,
			},
			hcat.NewStore(),
			"hello\nhello\r\nHELLO\r\nhello\nHELLO",
			false,
		},
		{
			"join",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," | join ";" }}`,
			},
			hcat.NewStore(),
			"a;b;c",
			false,
		},
		{
			"trimSpace",
			hcat.TemplateInput{
				Contents: `{{ "\t hi\n " | trimSpace }}`,
			},
			hcat.NewStore(),
			"hi",
			false,
		},
		{
			"split",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," }}`,
			},
			hcat.NewStore(),
			"[a b c]",
			false,
		},
		{
			"replaceAll",
			hcat.TemplateInput{
				Contents: `{{ "hello my hello" | regexReplaceAll "hello" "bye" }}`,
			},
			hcat.NewStore(),
			"bye my bye",
			false,
		},
		{
			"regexReplaceAll",
			hcat.TemplateInput{
				Contents: `{{ "foo" | regexReplaceAll "\\w" "x" }}`,
			},
			hcat.NewStore(),
			"xxx",
			false,
		},
		{
			"regexMatch",
			hcat.TemplateInput{
				Contents: `{{ "foo" | regexMatch "[a-z]+" }}`,
			},
			hcat.NewStore(),
			"true",
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
