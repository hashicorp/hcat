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
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"indent",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 4 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"    hello\n    hello\r\n    HELLO\r\n    hello\n    HELLO",
			false,
		},
		{
			"indent_negative",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent -4 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"",
			true,
		},
		{
			"indent_zero",
			hcat.TemplateInput{
				Contents: `{{ "hello\nhello\r\nHELLO\r\nhello\nHELLO" | indent 0 }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"hello\nhello\r\nHELLO\r\nhello\nHELLO",
			false,
		},
		{
			"join",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," | join ";" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"a;b;c",
			false,
		},
		{
			"trimSpace",
			hcat.TemplateInput{
				Contents: `{{ "\t hi\n " | trimSpace }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"hi",
			false,
		},
		{
			"split",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"[a b c]",
			false,
		},
		{
			"replaceAll",
			hcat.TemplateInput{
				Contents: `{{ "hello my hello" | regexReplaceAll "hello" "bye" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"bye my bye",
			false,
		},
		{
			"regexReplaceAll",
			hcat.TemplateInput{
				Contents: `{{ "foo" | regexReplaceAll "\\w" "x" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"xxx",
			false,
		},
		{
			"regexMatch",
			hcat.TemplateInput{
				Contents: `{{ "foo" | regexMatch "[a-z]+" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"true",
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
