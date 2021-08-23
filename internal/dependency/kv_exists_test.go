package dependency

import (
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVExistsQuery_NonBlocking(t *testing.T) {
	q, err := NewKVExistsQuery("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(q).(BlockingQuery); ok {
		t.Fatal("should NOT be blocking")
	}
}

func TestKVExistsQuery_SetOptions(t *testing.T) {
	q, err := NewKVExistsQuery("")
	if err != nil {
		t.Fatal(err)
	}
	q.SetOptions(QueryOptions{WaitIndex: 100, WaitTime: 100})
	if q.opts.WaitIndex != 0 {
		t.Fatal("WaitIndex should be zero")
	}
	if q.opts.WaitTime != 0 {
		t.Fatal("WaitTime should be zero")
	}
}

func TestNewKVExistsQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *KVExistsQuery
		err  bool
	}{
		{
			"empty",
			"",
			&KVExistsQuery{},
			false,
		},

		{
			"dc_only",
			"@dc1",
			nil,
			true,
		},
		{
			"key",
			"key",
			&KVExistsQuery{
				key: "key",
			},
			false,
		},
		{
			"dc",
			"key@dc1",
			&KVExistsQuery{
				key: "key",
				dc:  "dc1",
			},
			false,
		},
		{
			"dots",
			"key.with.dots",
			&KVExistsQuery{
				key: "key.with.dots",
			},
			false,
		},
		{
			"slashes",
			"key/with/slashes",
			&KVExistsQuery{
				key: "key/with/slashes",
			},
			false,
		},
		{
			"dashes",
			"key-with-dashes",
			&KVExistsQuery{
				key: "key-with-dashes",
			},
			false,
		},
		{
			"leading_slash",
			"/leading/slash",
			&KVExistsQuery{
				key: "leading/slash",
			},
			false,
		},
		{
			"trailing_slash",
			"trailing/slash/",
			&KVExistsQuery{
				key: "trailing/slash/",
			},
			false,
		},
		{
			"underscores",
			"key_with_underscores",
			&KVExistsQuery{
				key: "key_with_underscores",
			},
			false,
		},
		{
			"special_characters",
			"config/facet:größe-lf-si",
			&KVExistsQuery{
				key: "config/facet:größe-lf-si",
			},
			false,
		},
		{
			"splat",
			"config/*/timeouts/",
			&KVExistsQuery{
				key: "config/*/timeouts/",
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVExistsQuery(tc.i)
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

func TestNewKVExistsQueryV1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		opts []string
		exp  *KVExistsQuery
		err  bool
	}{
		{
			"empty",
			"",
			[]string{},
			nil,
			true,
		},
		{
			"key",
			"key",
			[]string{},
			&KVExistsQuery{
				key: "key",
			},
			false,
		},
		{
			"no_key",
			"",
			[]string{"dc=dc1"},
			nil,
			true,
		},
		{
			"dc",
			"key",
			[]string{"dc=dc1"},
			&KVExistsQuery{
				key: "key",
				dc:  "dc1",
			},
			false,
		},
		{
			"namespace",
			"key",
			[]string{"ns=test-namespace"},
			&KVExistsQuery{
				key: "key",
				ns:  "test-namespace",
			},
			false,
		},
		{
			"all_parameters",
			"key",
			[]string{"dc=dc1", "ns=test-namespace"},
			&KVExistsQuery{
				key: "key",
				dc:  "dc1",
				ns:  "test-namespace",
			},
			false,
		},
		{
			"invalid_parameter",
			"key",
			[]string{"invalid=param"},
			nil,
			true,
		},
		{
			"dots",
			"key.with.dots",
			[]string{},
			&KVExistsQuery{
				key: "key.with.dots",
			},
			false,
		},
		{
			"slashes",
			"key/with/slashes",
			[]string{},
			&KVExistsQuery{
				key: "key/with/slashes",
			},
			false,
		},
		{
			"dashes",
			"key-with-dashes",
			[]string{},
			&KVExistsQuery{
				key: "key-with-dashes",
			},
			false,
		},
		{
			"leading_slash",
			"/leading/slash",
			[]string{},
			&KVExistsQuery{
				key: "leading/slash",
			},
			false,
		},
		{
			"trailing_slash",
			"trailing/slash/",
			[]string{},
			&KVExistsQuery{
				key: "trailing/slash/",
			},
			false,
		},
		{
			"underscores",
			"key_with_underscores",
			[]string{},
			&KVExistsQuery{
				key: "key_with_underscores",
			},
			false,
		},
		{
			"special_characters",
			"config/facet:größe-lf-si",
			[]string{},
			&KVExistsQuery{
				key: "config/facet:größe-lf-si",
			},
			false,
		},
		{
			"splat",
			"config/*/timeouts/",
			[]string{},
			&KVExistsQuery{
				key: "config/*/timeouts/",
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVExistsQueryV1(tc.i, tc.opts)

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

func TestKVExistsQuery_Fetch(t *testing.T) {
	t.Parallel()

	testConsul.SetKVString(t, "test-kv-exists/key", "value")
	testConsul.SetKVString(t, "test-kv-exists/key_empty", "")

	cases := []struct {
		name string
		i    string
		exp  interface{}
	}{
		{
			"exists",
			"test-kv-exists/key",
			dep.KVExists(true),
		},
		{
			"exists_empty_string",
			"test-kv-exists/key_empty",
			dep.KVExists(true),
		},
		{
			"no_exist",
			"test-kv-exists/not/a/real/key/like/ever",
			dep.KVExists(false),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewKVExistsQueryV1(tc.i, []string{})
			if err != nil {
				t.Fatal(err)
			}

			act, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tc.exp, act)
		})
	}

	t.Run("stops", func(t *testing.T) {
		d, err := NewKVGetQuery("test-kv-exists/key")
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
		case <-time.After(100 * time.Millisecond):
			t.Errorf("did not stop")
		}
	})

}

func TestKVExistsQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"key",
			"key",
			"kv.exists(key)",
		},
		{
			"dc",
			"key@dc1",
			"kv.exists(key@dc1)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewKVExistsQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
