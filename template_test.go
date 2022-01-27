package hcat

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestNewTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    TemplateInput
		e    *Template
	}{
		{
			"nil",
			TemplateInput{Name: "nil"},
			NewTemplate(TemplateInput{Name: "nil"}),
		},
		{
			"contents",
			TemplateInput{
				Name:     "test",
				Contents: "test",
			},
			&Template{
				name:     "test",
				contents: "test",
				hexMD5:   "098f6bcd4621d373cade4e832627b4f6",
			},
		},
		{
			"custom_delims",
			TemplateInput{
				Name:       "test",
				Contents:   "test",
				LeftDelim:  "<<",
				RightDelim: ">>",
			},
			&Template{
				name:       "test",
				contents:   "test",
				hexMD5:     "098f6bcd4621d373cade4e832627b4f6",
				leftDelim:  "<<",
				rightDelim: ">>",
			},
		},
		{
			"err_missing_key",
			TemplateInput{
				Name:          "test",
				Contents:      "test",
				ErrMissingKey: true,
			},
			&Template{
				name:          "test",
				contents:      "test",
				hexMD5:        "098f6bcd4621d373cade4e832627b4f6",
				errMissingKey: true,
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tmpl := NewTemplate(tc.i)
			tc.e.dirty, tmpl.dirty = nil, nil // don't compare well
			if !reflect.DeepEqual(tc.e, tmpl) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, tmpl)
			}
		})
	}

	t.Run("explicit_name_checks", func(t *testing.T) {
		contentsMD5 := "098f6bcd4621d373cade4e832627b4f6"

		tmpl := NewTemplate(
			TemplateInput{
				Name:     "foo",
				Contents: "test",
			})
		if tmpl.ID() != (contentsMD5 + "_foo") {
			t.Fatalf("ID is wrong, got '%s', want '%s'\n", tmpl.ID(),
				contentsMD5+"_foo")
		}

		tmpl = NewTemplate(
			TemplateInput{
				Contents: "test",
			})
		if tmpl.ID() != contentsMD5 {
			t.Fatalf("ID is wrong, got '%s', want '%s'\n", tmpl.ID(), contentsMD5)
		}

	})
}

func TestTemplate_Execute(t *testing.T) {
	t.Parallel()

	// set an environment variable for the tests
	if err := os.Setenv("CT_TEST", "1"); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Unsetenv("CT_TEST") }()

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("test")
	defer os.Remove(f.Name())

	cases := []struct {
		name string
		ti   TemplateInput
		i    *Store
		e    string
		err  bool
	}{
		{
			"nil",
			TemplateInput{
				Contents: `test`,
			},
			nil,
			"test",
			false,
		},
		{
			"bad_func",
			TemplateInput{
				Contents: `{{ bad_func }}`,
			},
			nil,
			"",
			true,
		},
		// missing keys
		{
			"err_missing_keys__true",
			TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: true,
			},
			nil,
			"",
			true,
		},
		{
			"err_missing_keys__false",
			TemplateInput{
				Contents:      `{{ .Data.Foo }}`,
				ErrMissingKey: false,
			},
			nil,
			"<no value>",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := NewTemplate(tc.ti)

			w := fakeWatcher{tc.i}
			a, err := tpl.Execute(w.Recaller(tpl))
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(tc.e), a) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a))
			}
		})
	}
}

func TestCachedTemplate(t *testing.T) {
	d, err := idep.NewKVGetQuery("key")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("cache-is-used", func(t *testing.T) {
		recaller := func() *Store {
			st := NewStore()
			st.Save(d.ID(), "value")
			return st
		}()
		ti := TemplateInput{
			Contents: `{{ testStore "key" }}`,
			FuncMapMerge: map[string]interface{}{
				"testStore": func(key string) interface{} {
					v, ok := recaller.Recall(d.ID())
					if !ok {
						t.Errorf("key not found")
					}
					return v
				}},
		}
		tpl := NewTemplate(ti)
		w := fakeWatcher{recaller}
		content, err := tpl.Execute(w.Recaller(tpl))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, []byte("value")) {
			t.Fatal("bad content:", string(content))
		}
		// Cache used here
		content, err = tpl.Execute(w.Recaller(tpl))
		if err != ErrNoNewValues {
			t.Fatal("error should be ErrNoNewValues")
		}
		if !bytes.Equal(content, []byte("value")) {
			t.Fatal("bad content:", string(content))
		}
	})
}

type fakeWatcher struct {
	*Store
}

func (fakeWatcher) Buffering(string) bool    { return false }
func (f fakeWatcher) Complete(Notifier) bool { return true }
func (f fakeWatcher) Mark(Notifier)          {}
func (f fakeWatcher) Sweep(Notifier)         {}
func (f fakeWatcher) Recaller(Notifier) Recaller {
	return func(d dep.Dependency) (value interface{}, found bool, err error) {
		v, found := f.Store.Recall(d.ID())
		return v, found, nil
	}
}
