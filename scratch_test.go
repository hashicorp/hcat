package hat

import (
	"fmt"
	"reflect"
	"testing"
)

func TestScratch_Key(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    *scratch
		k    string
		e    bool
	}{
		{
			"no_exist",
			&scratch{},
			"",
			false,
		},
		{
			"exist",
			&scratch{
				values: map[string]interface{}{
					"foo": nil,
				},
			},
			"foo",
			true,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			r := tc.s.Key(tc.k)
			if !reflect.DeepEqual(tc.e, r) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, r)
			}
		})
	}
}

func TestScratch_Get(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    *scratch
		k    string
		e    interface{}
	}{
		{
			"no_exist",
			&scratch{},
			"",
			nil,
		},
		{
			"exist",
			&scratch{
				values: map[string]interface{}{
					"foo": "bar",
				},
			},
			"foo",
			"bar",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			r := tc.s.Get(tc.k)
			if !reflect.DeepEqual(tc.e, r) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, r)
			}
		})
	}
}

func TestScratch_Set(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    func(*scratch)
		e    *scratch
	}{
		{
			"no_exist",
			func(s *scratch) {
				s.Set("foo", "bar")
			},
			&scratch{
				values: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		{
			"overwrites",
			func(s *scratch) {
				s.Set("foo", "bar")
				s.Set("foo", "zip")
			},
			&scratch{
				values: map[string]interface{}{
					"foo": "zip",
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var s scratch
			tc.f(&s)

			if !reflect.DeepEqual(tc.e.values, s.values) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e.values, s.values)
			}
		})
	}
}

func TestScratch_SetX(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    func(*scratch)
		e    *scratch
	}{
		{
			"no_exist",
			func(s *scratch) {
				s.SetX("foo", "bar")
			},
			&scratch{
				values: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		{
			"overwrites",
			func(s *scratch) {
				s.SetX("foo", "bar")
				s.SetX("foo", "zip")
			},
			&scratch{
				values: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var s scratch
			tc.f(&s)

			if !reflect.DeepEqual(tc.e.values, s.values) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e.values, s.values)
			}
		})
	}
}

func TestScratch_MapSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    func(*scratch)
		e    *scratch
	}{
		{
			"no_exist",
			func(s *scratch) {
				s.MapSet("a", "foo", "bar")
				s.MapSet("b", "foo", "bar")
			},
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{
						"foo": "bar",
					},
					"b": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			"overwrites",
			func(s *scratch) {
				s.MapSet("a", "foo", "bar")
				s.MapSet("a", "foo", "zip")
			},
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{
						"foo": "zip",
					},
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var s scratch
			tc.f(&s)

			if !reflect.DeepEqual(tc.e.values, s.values) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e.values, s.values)
			}
		})
	}
}

func TestScratch_MapSetX(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    func(*scratch)
		e    *scratch
	}{
		{
			"no_exist",
			func(s *scratch) {
				s.MapSetX("a", "foo", "bar")
				s.MapSetX("b", "foo", "bar")
			},
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{
						"foo": "bar",
					},
					"b": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			"overwrites",
			func(s *scratch) {
				s.MapSetX("a", "foo", "bar")
				s.MapSetX("a", "foo", "zip")
			},
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var s scratch
			tc.f(&s)

			if !reflect.DeepEqual(tc.e.values, s.values) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e.values, s.values)
			}
		})
	}
}

func TestScratch_MapValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    *scratch
		e    []interface{}
		err  bool
	}{
		{
			"sorted",
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{
						"foo":   "bar",
						"zip":   "zap",
						"peach": "banana",
					},
				},
			},
			[]interface{}{"bar", "banana", "zap"},
			false,
		},
		{
			"empty",
			&scratch{
				values: map[string]interface{}{
					"a": map[string]interface{}{},
				},
			},
			[]interface{}{},
			false,
		},
		{
			"not_map",
			&scratch{
				values: map[string]interface{}{
					"a": true,
				},
			},
			nil,
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			r, err := tc.s.MapValues("a")
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.e, r) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, r)
			}
		})
	}
}
