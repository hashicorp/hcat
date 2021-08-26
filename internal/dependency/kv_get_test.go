package dependency

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVGetQuery_Blocking(t *testing.T) {
	q, err := NewKVGetQuery("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(q).(BlockingQuery); !ok {
		t.Fatal("should be blocking")
	}
}

func TestKVGetQuery_SetOptions(t *testing.T) {
	q, err := NewKVGetQuery("")
	if err != nil {
		t.Fatal(err)
	}
	q.SetOptions(QueryOptions{WaitIndex: 100, WaitTime: 100})
	if q.opts.WaitIndex != 100 {
		t.Fatal("WaitIndex should be 100")
	}
	if q.opts.WaitTime != 100 {
		t.Fatal("WaitTime should be 100")
	}
}

func TestNewKVGetQuery(t *testing.T) {
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

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := NewKVGetQuery(tc.i)

			if act != nil {
				act.stopCh = nil
			}

			if tc.err {
				assert.Error(t, err)
			} else {
				exp := &KVGetQuery{KVExistsQuery: *tc.exp}
				assert.Equal(t, exp, act)
			}
		})
	}
}

func TestNewKVGetQueryV1(t *testing.T) {
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
			act, err := NewKVGetQueryV1(tc.i, tc.opts)

			if act != nil {
				act.stopCh = nil
			}

			if tc.err {
				assert.Error(t, err)
			} else {
				exp := &KVGetQuery{KVExistsQuery: *tc.exp}
				assert.Equal(t, exp, act)
			}
		})
	}
}

func TestKVGetQuery_Fetch(t *testing.T) {
	t.Parallel()

	testConsul.SetKVString(t, "test-kv-get/key", "value")
	testConsul.SetKVString(t, "test-kv-get/key_empty", "")

	cases := []struct {
		name string
		i    string
		exp  interface{}
	}{
		{
			"exists",
			"test-kv-get/key",
			dep.KvValue("value"),
		},
		{
			"exists_empty_string",
			"test-kv-get/key_empty",
			dep.KvValue(""),
		},
		{
			"no_exist",
			"test-kv-get/not/a/real/key/like/ever",
			nil,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewKVGetQuery(tc.i)
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
		d, err := NewKVGetQuery("test-kv-get/key")
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

	t.Run("fires_changes", func(t *testing.T) {
		d, err := NewKVGetQuery("test-kv-get/key")
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

		testConsul.SetKVString(t, "test-kv-get/key", "new-value")

		select {
		case err := <-errCh:
			t.Fatal(err)
		case data := <-dataCh:
			assert.Equal(t, data, dep.KvValue("new-value"))
		}
	})
}

func TestKVGetQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"key",
			"key",
			"kv.get(key)",
		},
		{
			"dc",
			"key@dc1",
			"kv.get(key@dc1)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := NewKVGetQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
