package dependency

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVListQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *KVListQuery
		err  bool
	}{
		{
			"empty",
			"",
			&KVListQuery{},
			false,
		},
		{
			"dc_only",
			"@dc1",
			nil,
			true,
		},
		{
			"prefix",
			"prefix",
			&KVListQuery{
				prefix: "prefix",
			},
			false,
		},
		{
			"dc",
			"prefix@dc1",
			&KVListQuery{
				prefix: "prefix",
				dc:     "dc1",
			},
			false,
		},
		{
			"dots",
			"prefix.with.dots",
			&KVListQuery{
				prefix: "prefix.with.dots",
			},
			false,
		},
		{
			"slashes",
			"prefix/with/slashes",
			&KVListQuery{
				prefix: "prefix/with/slashes",
			},
			false,
		},
		{
			"dashes",
			"prefix-with-dashes",
			&KVListQuery{
				prefix: "prefix-with-dashes",
			},
			false,
		},
		{
			"leading_slash",
			"/leading/slash",
			&KVListQuery{
				prefix: "leading/slash",
			},
			false,
		},
		{
			"trailing_slash",
			"trailing/slash/",
			&KVListQuery{
				prefix: "trailing/slash/",
			},
			false,
		},
		{
			"underscores",
			"prefix_with_underscores",
			&KVListQuery{
				prefix: "prefix_with_underscores",
			},
			false,
		},
		{
			"special_characters",
			"config/facet:größe-lf-si",
			&KVListQuery{
				prefix: "config/facet:größe-lf-si",
			},
			false,
		},
		{
			"splat",
			"config/*/timeouts/",
			&KVListQuery{
				prefix: "config/*/timeouts/",
			},
			false,
		},
		{
			"slash",
			"/",
			&KVListQuery{
				prefix: "/",
			},
			false,
		},
		{
			"slash-slash",
			"//",
			&KVListQuery{
				prefix: "/",
			},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := NewKVListQuery(tc.i)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if act != nil {
				act.stopCh = nil
			}

			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestNewKVListQueryV1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		prefix string
		opts   []string
		exp    *KVListQuery
		err    bool
	}{
		{
			"empty",
			"",
			[]string{},
			nil,
			true,
		},
		{
			"prefix",
			"prefix",
			[]string{},
			&KVListQuery{
				prefix: "prefix",
			},
			false,
		},
		{
			"query_parameters_without_prefix",
			"",
			[]string{"dc=dc1"},
			nil,
			true,
		},
		{
			"dc",
			"prefix",
			[]string{"dc=dc1"},
			&KVListQuery{
				prefix: "prefix",
				dc:     "dc1",
			},
			false,
		},
		{
			"namespace",
			"prefix",
			[]string{"ns=test"},
			&KVListQuery{
				prefix: "prefix",
				ns:     "test",
			},
			false,
		},
		{
			"all_parameters",
			"prefix",
			[]string{"dc=dc1", "ns=test"},
			&KVListQuery{
				prefix: "prefix",
				dc:     "dc1",
				ns:     "test",
			},
			false,
		},
		{
			"invalid_parameter",
			"",
			[]string{"invalid=param"},
			nil,
			true,
		},
		{
			"dots",
			"prefix.with.dots",
			[]string{},
			&KVListQuery{
				prefix: "prefix.with.dots",
			},
			false,
		},
		{
			"slashes",
			"prefix/with/slashes",
			[]string{},
			&KVListQuery{
				prefix: "prefix/with/slashes",
			},
			false,
		},
		{
			"dashes",
			"prefix-with-dashes",
			[]string{},
			&KVListQuery{
				prefix: "prefix-with-dashes",
			},
			false,
		},
		{
			"leading_slash",
			"/leading/slash",
			[]string{},
			&KVListQuery{
				prefix: "leading/slash",
			},
			false,
		},
		{
			"trailing_slash",
			"trailing/slash/",
			[]string{},
			&KVListQuery{
				prefix: "trailing/slash/",
			},
			false,
		},
		{
			"underscores",
			"prefix_with_underscores",
			[]string{},
			&KVListQuery{
				prefix: "prefix_with_underscores",
			},
			false,
		},
		{
			"special_characters",
			"config/facet:größe-lf-si",
			[]string{},
			&KVListQuery{
				prefix: "config/facet:größe-lf-si",
			},
			false,
		},
		{
			"splat",
			"config/*/timeouts/",
			[]string{},
			&KVListQuery{
				prefix: "config/*/timeouts/",
			},
			false,
		},
		{
			"slash",
			"/",
			[]string{},
			nil,
			true,
		},
		{
			"slash-slash",
			"//",
			[]string{},
			&KVListQuery{
				prefix: "/",
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVListQueryV1(tc.prefix, tc.opts)
			if act != nil {
				act.stopCh = nil
			}

			if tc.err {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tc.exp, act)
			}
		})
	}
}

func TestKVListQuery_Fetch(t *testing.T) {
	t.Parallel()

	testConsul.SetKVString(t, "test-kv-list/prefix/foo", "bar")
	testConsul.SetKVString(t, "test-kv-list/prefix/zip", "zap")
	testConsul.SetKVString(t, "test-kv-list/prefix/wave/ocean", "sleek")

	cases := []struct {
		name string
		i    string
		exp  []*dep.KeyPair
	}{
		{
			"exists",
			"test-kv-list/prefix",
			[]*dep.KeyPair{
				{
					Path:   "test-kv-list/prefix/foo",
					Key:    "foo",
					Value:  "bar",
					Exists: true,
				},
				{
					Path:   "test-kv-list/prefix/wave/ocean",
					Key:    "wave/ocean",
					Value:  "sleek",
					Exists: true,
				},
				{
					Path:   "test-kv-list/prefix/zip",
					Key:    "zip",
					Value:  "zap",
					Exists: true,
				},
			},
		},
		{
			"trailing",
			"test-kv-list/prefix/",
			[]*dep.KeyPair{
				{
					Path:   "test-kv-list/prefix/foo",
					Key:    "foo",
					Value:  "bar",
					Exists: true,
				},
				{
					Path:   "test-kv-list/prefix/wave/ocean",
					Key:    "wave/ocean",
					Value:  "sleek",
					Exists: true,
				},
				{
					Path:   "test-kv-list/prefix/zip",
					Key:    "zip",
					Value:  "zap",
					Exists: true,
				},
			},
		},
		{
			"no_exist",
			"test-kv-list/not/a/real/prefix/like/ever",
			[]*dep.KeyPair{},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewKVListQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}

			act, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}

			for _, p := range act.([]*dep.KeyPair) {
				p.CreateIndex = 0
				p.ModifyIndex = 0
			}

			assert.Equal(t, tc.exp, act)
		})
	}

	t.Run("stops", func(t *testing.T) {
		d, err := NewKVListQuery("test-kv-list/prefix")
		if err != nil {
			t.Fatal(err)
		}

		dataCh := make(chan interface{}, 1)
		errCh := make(chan error, 1)
		go func() {
			for {
				data, _, err := d.Fetch(testClients)
				if err != nil {
					errCh <- err
					return
				}
				dataCh <- data
			}
		}()

		select {
		case err := <-errCh:
			t.Fatal(err)
		case <-dataCh:
		}

		d.Stop()

		select {
		case err := <-errCh:
			if err != ErrStopped {
				t.Fatal(err)
			}
		case <-time.After(250 * time.Millisecond):
			t.Errorf("did not stop")
		}
	})

	t.Run("fires_changes", func(t *testing.T) {
		d, err := NewKVListQuery("test-kv-list/prefix/")
		if err != nil {
			t.Fatal(err)
		}

		_, qm, err := d.Fetch(testClients)
		if err != nil {
			t.Fatal(err)
		}

		dataCh := make(chan interface{}, 1)
		errCh := make(chan error, 1)
		go func() {
			for {
				d.SetOptions(QueryOptions{WaitIndex: qm.LastIndex})
				data, _, err := d.Fetch(testClients)
				if err != nil {
					errCh <- err
					return
				}
				dataCh <- data
				return
			}
		}()

		testConsul.SetKVString(t, "test-kv-list/prefix/foo", "new-bar")

		select {
		case err := <-errCh:
			t.Fatal(err)
		case data := <-dataCh:
			typed := data.([]*dep.KeyPair)
			if len(typed) == 0 {
				t.Fatal("bad length")
			}

			act := typed[0]
			act.CreateIndex = 0
			act.ModifyIndex = 0

			exp := &dep.KeyPair{
				Path:   "test-kv-list/prefix/foo",
				Key:    "foo",
				Value:  "new-bar",
				Exists: true,
			}

			assert.Equal(t, exp, act)
		}
	})
}

func TestKVListQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"prefix",
			"prefix",
			"kv.list(prefix)",
		},
		{
			"dc",
			"prefix@dc1",
			"kv.list(prefix@dc1)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewKVListQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.ID())
		})
	}
}
