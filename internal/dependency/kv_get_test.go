package dependency

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVGetQuery_Blocking(t *testing.T) {
	t.Run("non-blocking_query", func(t *testing.T) {
		q, err := NewKVExistsQuery("")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := interface{}(q).(BlockingQuery); ok {
			t.Fatal("should NOT be blocking")
		}
	})
	t.Run("blocking_query", func(t *testing.T) {
		q, err := NewKVGetQuery("")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := interface{}(q).(BlockingQuery); !ok {
			t.Fatal("should be blocking")
		}
	})
}

func TestKVGetQuery_SetOptions(t *testing.T) {
	t.Run("get_query_options", func(t *testing.T) {
		q, err := NewKVGetQuery("")
		if err != nil {
			t.Fatal(err)
		}
		q.SetOptions(QueryOptions{WaitIndex: 100, WaitTime: 100})
		if q.opts.WaitIndex != 100 {
			t.Fatal("WaitIndex should be zero")
		}
		if q.opts.WaitTime != 100 {
			t.Fatal("WaitTime should be zero")
		}
	})
	t.Run("exists_query_options", func(t *testing.T) {
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
	})
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
		f    interface{}
	}{
		{
			"key",
			"key",
			"kv.get(key)",
			NewKVGetQuery,
		},
		{
			"dc",
			"key@dc1",
			"kv.get(key@dc1)",
			NewKVGetQuery,
		},
		{
			"dc",
			"key@dc1",
			"kv.exists(key@dc1)",
			NewKVExistsQuery,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var d fmt.Stringer
			var err error
			switch pv := reflect.ValueOf(tc.f); pv {
			case reflect.ValueOf(NewKVGetQuery):
				d, err = NewKVGetQuery(tc.i)
			case reflect.ValueOf(NewKVExistsQuery):
				d, err = NewKVExistsQuery(tc.i)
			}
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
