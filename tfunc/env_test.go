package tfunc

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/hcat"
)

func TestEnvExecute(t *testing.T) {
	t.Parallel()

	// set an environment variable for the tests
	envVars := map[string]string{"HCAT_TEST": "foo", "EMPTY_VAR": ""}
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			t.Fatal(err)
		}
		defer func(e string) { os.Unsetenv(e) }(k)
	}

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"helper_env",
			hcat.TemplateInput{
				// HCAT_TEST set above
				Contents: `{{ env "HCAT_TEST" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"foo",
			false,
		},
		{
			"func_envOrDefault",
			hcat.TemplateInput{
				Contents: `{{ envOrDefault "HCAT_TEST" "100" }} {{ envOrDefault "EMPTY_VAR" "200" }} {{ envOrDefault "UNSET_VAR" "300" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"foo  300",
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
