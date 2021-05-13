package tfunc

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcat"
)

func TestTimeExecute(t *testing.T) {
	t.Parallel()

	// overwrite now variable from ./time.go
	now = func() time.Time { return time.Unix(0, 0).UTC() }

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Watcherer
		e    string
		err  bool
	}{
		{
			"timestamp",
			hcat.TemplateInput{
				Contents: `{{ timestamp }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"1970-01-01T00:00:00Z",
			false,
		},
		{
			"helper_timestamp__formatted",
			hcat.TemplateInput{
				Contents: `{{ timestamp "2006-01-02" }}`,
			},
			fakeWatcher{hcat.NewStore()},
			"1970-01-01",
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
