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
	if err := os.Setenv("CT_TEST", "1"); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Unsetenv("CT_TEST") }()

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
				// CT_TEST set above
				Contents: `{{ env "CT_TEST" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"1",
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
