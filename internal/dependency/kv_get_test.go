package dependency

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVGetQuery_Blocking(t *testing.T) {
	q, err := NewKVGetQuery("key")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(q).(BlockingQuery); !ok {
		t.Fatal("should be blocking")
	}
}

func TestKVGetQuery_SetOptions(t *testing.T) {
	q, err := NewKVGetQuery("key")
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

type newKVGetCase struct {
	exp *KVExistsQuery
	act *KVGetQuery
	err error
}

func verifyNewKVGetQuery(t *testing.T, tc newKVGetCase) {
	if tc.act != nil {
		tc.act.stopCh = nil
	}

	if tc.exp == nil {
		assert.Error(t, tc.err)
	} else {
		exp := &KVGetQuery{KVExistsQuery: *tc.exp}
		assert.Equal(t, exp, tc.act)
	}
}

func TestNewKVGetQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *KVExistsQuery
	}{
		{
			"empty",
			"",
			nil,
		},
		{
			"key",
			"key",
			&KVExistsQuery{
				key: "key",
			},
		},
		{
			"dots",
			"key.with.dots",
			&KVExistsQuery{
				key: "key.with.dots",
			},
		},
		{
			"slashes",
			"key/with/slashes",
			&KVExistsQuery{
				key: "key/with/slashes",
			},
		},
		{
			"dashes",
			"key-with-dashes",
			&KVExistsQuery{
				key: "key-with-dashes",
			},
		},
		{
			"leading_slash",
			"/leading/slash",
			&KVExistsQuery{
				key: "leading/slash",
			},
		},
		{
			"trailing_slash",
			"trailing/slash/",
			&KVExistsQuery{
				key: "trailing/slash/",
			},
		},
		{
			"underscores",
			"key_with_underscores",
			&KVExistsQuery{
				key: "key_with_underscores",
			},
		},
		{
			"special_characters",
			"config/facet:größe-lf-si",
			&KVExistsQuery{
				key: "config/facet:größe-lf-si",
			},
		},
		{
			"splat",
			"config/*/timeouts/",
			&KVExistsQuery{
				key: "config/*/timeouts/",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVGetQuery(tc.i)
			verifyNewKVGetQuery(t, newKVGetCase{tc.exp, act, err})
		})

		t.Run(fmt.Sprintf("V1/%s", tc.name), func(t *testing.T) {
			act, err := NewKVGetQueryV1(tc.i, []string{})
			verifyNewKVGetQuery(t, newKVGetCase{tc.exp, act, err})
		})
	}
}

func TestNewKVGetQueryWithParameters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  *KVExistsQuery
	}{
		{
			"dc_only",
			"@dc1",
			nil,
		},
		{
			"dc",
			"key@dc1",
			&KVExistsQuery{
				key: "key",
				dc:  "dc1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVGetQuery(tc.i)
			verifyNewKVGetQuery(t, newKVGetCase{tc.exp, act, err})
		})
	}
}

func TestNewKVGetQueryV1WithParameters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		opts []string
		exp  *KVExistsQuery
	}{
		{
			"no_key",
			"",
			[]string{"dc=dc1"},
			nil,
		},
		{
			"dc",
			"key",
			[]string{"dc=dc1"},
			&KVExistsQuery{
				key: "key",
				dc:  "dc1",
			},
		},
		{
			"namespace",
			"key",
			[]string{"ns=test-namespace"},
			&KVExistsQuery{
				key: "key",
				ns:  "test-namespace",
			},
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
		},
		{
			"invalid_parameter",
			"key",
			[]string{"invalid=param"},
			nil,
		},
		{
			"invalid_format",
			"key",
			[]string{"invalid-param"},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			act, err := NewKVGetQueryV1(tc.i, tc.opts)
			verifyNewKVGetQuery(t, newKVGetCase{tc.exp, act, err})
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
			assert.Equal(t, tc.exp, d.ID())
		})
	}
}
