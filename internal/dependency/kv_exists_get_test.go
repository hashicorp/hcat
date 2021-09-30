package dependency

import (
	"testing"

	"github.com/hashicorp/hcat/dep"
	"github.com/stretchr/testify/assert"
)

func TestNewKVExistsGetQuery_NonBlocking(t *testing.T) {
	q, err := NewKVExistsGetQueryV1("key", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(q).(BlockingQuery); ok {
		t.Fatal("should NOT be blocking")
	}
}

func TestKVExistsGetQuery_SetOptions(t *testing.T) {
	q, err := NewKVExistsGetQueryV1("key", []string{})
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

type newKVExistsGetCase struct {
	exp *KVExistsQuery
	act *KVExistsGetQuery
	err error
}

func verifyNewKVExistsGetQueryV1(t *testing.T, tc newKVExistsGetCase) {
	if tc.act != nil {
		tc.act.stopCh = nil
	}

	if tc.exp == nil {
		assert.Error(t, tc.err)
	} else {
		exp := &KVExistsGetQuery{KVExistsQuery: *tc.exp}
		assert.Equal(t, exp, tc.act)
	}
}

func TestNewKVExistsGetQueryV1(t *testing.T) {
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
			act, err := NewKVExistsGetQueryV1(tc.i, []string{})
			verifyNewKVExistsGetQueryV1(t, newKVExistsGetCase{tc.exp, act, err})
		})
	}
}

func TestNewKVExistsGetQueryV1WithParameters(t *testing.T) {
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
			act, err := NewKVExistsGetQueryV1(tc.i, tc.opts)
			verifyNewKVExistsGetQueryV1(t, newKVExistsGetCase{tc.exp, act, err})
		})
	}
}

func TestKVExistsGetQuery_Fetch(t *testing.T) {
	t.Parallel()

	testConsul.SetKVString(t, "test-kv-get/key", "value")
	testConsul.SetKVString(t, "test-kv-get/key_empty", "")

	cases := []struct {
		name string
		i    string
		exp  *dep.KeyPair
	}{
		{
			"exists",
			"test-kv-get/key",
			&dep.KeyPair{
				Path:   "test-kv-get/key",
				Key:    "test-kv-get/key",
				Exists: true,
				Value:  "value",
			},
		},
		{
			"exists_empty_string",
			"test-kv-get/key_empty",
			&dep.KeyPair{
				Path:   "test-kv-get/key_empty",
				Key:    "test-kv-get/key_empty",
				Exists: true,
				Value:  "",
			},
		},
		{
			"does_not_exist",
			"test-kv-get/not/a/real/key",
			&dep.KeyPair{
				Path:   "test-kv-get/not/a/real/key",
				Key:    "test-kv-get/not/a/real/key",
				Exists: false,
				Value:  "",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewKVExistsGetQueryV1(tc.i, []string{})
			if err != nil {
				t.Fatal(err)
			}

			kv, _, err := d.Fetch(testClients)
			if err != nil {
				t.Fatal(err)
			}

			act, ok := kv.(*dep.KeyPair)
			assert.True(t, ok, "unexpected dependency type")

			assert.Equal(t, tc.exp.Path, act.Path)
			assert.Equal(t, tc.exp.Key, act.Key)
			assert.Equal(t, tc.exp.Exists, act.Exists)
			assert.Equal(t, tc.exp.Value, act.Value)
		})
	}
}

func TestKVExistsGetQuery_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"key",
			"key",
			"kv.exists.get(key)",
		},
		{
			"opts",
			"key dc=dc1 ns=ns",
			"kv.exists.get(key dc=dc1 ns=ns)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewKVExistsGetQueryV1(tc.i, []string{})
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}
