package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
)

func TestTransformExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"func_base64Decode",
			hcat.TemplateInput{
				Contents: `{{ base64Decode "aGVsbG8=" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"hello",
			false,
		},
		{
			"func_base64Decode_bad",
			hcat.TemplateInput{
				Contents: `{{ base64Decode "aGVsxxbG8=" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"",
			true,
		},
		{
			"func_base64Encode",
			hcat.TemplateInput{
				Contents: `{{ base64Encode "hello" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"aGVsbG8=",
			false,
		},
		{
			"func_base64URLDecode",
			hcat.TemplateInput{
				Contents: `{{ base64URLDecode "dGVzdGluZzEyMw==" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"testing123",
			false,
		},
		{
			"func_base64URLDecode_bad",
			hcat.TemplateInput{
				Contents: `{{ base64URLDecode "aGVsxxbG8=" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"",
			true,
		},
		{
			"func_base64URLEncode",
			hcat.TemplateInput{
				Contents: `{{ base64URLEncode "testing123" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"dGVzdGluZzEyMw==",
			false,
		},
		{
			"func_sha256",
			hcat.TemplateInput{
				Contents: `{{ sha256Hex "hello" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			false,
		},
		{
			"func_md5sum",
			hcat.TemplateInput{
				Contents: `{{ "hello" | md5sum }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"5d41402abc4b2a76b9719d911017c592",
			false,
		},
		{
			"helper_toJSON",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," | toJSON }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"[\"a\",\"b\",\"c\"]",
			false,
		},
		{
			"helper_toJSONPretty",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," | toJSONPretty }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"[\n  \"a\",\n  \"b\",\n  \"c\"\n]",
			false,
		},
		{
			"helper_toUnescapedJSON",
			hcat.TemplateInput{
				Contents: `{{ "a?b&c,x?y&z" | split "," | toUnescapedJSON }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"[\"a?b&c\",\"x?y&z\"]",
			false,
		},
		{
			"helper_toUnescapedJSONPretty",
			hcat.TemplateInput{
				Contents: `{{ tree "list" | explode | toUnescapedJSONPretty }}`,
			},
			func() hcat.Watcherer {
				st := hcat.NewStore()
				id := testKVListQueryID("list")
				st.Save(id, []*dep.KeyPair{
					{Key: "a", Value: "b&c"},
					{Key: "x", Value: "y&z"},
					{Key: "k", Value: "<>&&"},
				})
				return fakeWatcher{st}
			}(),
			"{\n  \"a\": \"b&c\",\n  \"k\": \"<>&&\",\n  \"x\": \"y&z\"\n}",
			false,
		},
		{
			"helper_toLower",
			hcat.TemplateInput{
				Contents: `{{ "HI" | toLower }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"hi",
			false,
		},
		{
			"helper_toTitle",
			hcat.TemplateInput{
				Contents: `{{ "this is a sentence" | toTitle }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"This Is A Sentence",
			false,
		},
		{
			"helper_toTOML",
			hcat.TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toTOML }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"foo = \"bar\"",
			false,
		},
		{
			"helper_toUpper",
			hcat.TemplateInput{
				Contents: `{{ "hi" | toUpper }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"HI",
			false,
		},
		{
			"helper_toYAML",
			hcat.TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toYAML }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"foo: bar",
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
