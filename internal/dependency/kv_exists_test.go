package dependency

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVExistsQuery_NonBlocking(t *testing.T) {
	q, err := NewKVExistsQuery("key")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(q).(BlockingQuery); ok {
		t.Fatal("should NOT be blocking")
	}
}

func TestKVExistsQuery_SetOptions(t *testing.T) {
	q, err := NewKVExistsQuery("key")
	if err != nil {
		t.Fatal(err)
	}
	q.SetOptions(QueryOptions{WaitIndex: 100, WaitTime: 100})
	// WaitIndex and WaitTime should always be 0 regardless of query options
	if q.opts.WaitIndex != 0 {
		t.Fatal("WaitIndex should be 0")
	}
	if q.opts.WaitTime != 0 {
		t.Fatal("WaitTime should be 0")
	}
}

type newKVExistsCase struct {
	exp *KVExistsQuery
	act *KVExistsQuery
	err error
}

func verifyNewKVExistsQuery(t *testing.T, tc newKVExistsCase) {
	if tc.act != nil {
		tc.act.stopCh = nil
	}

	if tc.exp == nil {
		assert.Error(t, tc.err)
	} else {
		assert.Equal(t, tc.exp, tc.act)
	}
}

func TestNewKVExists(t *testing.T) {
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
			act, err := NewKVExistsQuery(tc.i)
			verifyNewKVExistsQuery(t, newKVExistsCase{tc.exp, act, err})
		})

		t.Run(fmt.Sprintf("V1/%s", tc.name), func(t *testing.T) {
			act, err := NewKVExistsQueryV1(tc.i, []string{})
			verifyNewKVExistsQuery(t, newKVExistsCase{tc.exp, act, err})
		})
	}
}

func TestNewKVExistsQueryWithParameters(t *testing.T) {
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
			act, err := NewKVExistsQuery(tc.i)
			verifyNewKVExistsQuery(t, newKVExistsCase{tc.exp, act, err})
		})
	}
}

func TestNewKVExistsQueryV1WithParameters(t *testing.T) {
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
			act, err := NewKVExistsQueryV1(tc.i, tc.opts)
			verifyNewKVExistsQuery(t, newKVExistsCase{tc.exp, act, err})
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
			assert.Equal(t, tc.exp, d.ID())
		})
	}
}
