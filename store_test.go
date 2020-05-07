package hat

import (
	"reflect"
	"testing"

	dep "github.com/hashicorp/hat/internal/dependency"
)

func TestNewStore(t *testing.T) {
	t.Parallel()
	st := NewStore()

	if st.data == nil {
		t.Errorf("expected data to not be nil")
	}

	if st.receivedData == nil {
		t.Errorf("expected receivedData to not be nil")
	}
}

func TestRecall(t *testing.T) {
	t.Parallel()
	st := NewStore()

	d, err := dep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		&dep.Node{
			Node:    "node",
			Address: "address",
		},
	}

	st.Save(d, nodes)

	data, ok := st.Recall(d)
	if !ok {
		t.Fatal("expected data from Store")
	}

	result := data.([]*dep.Node)
	if !reflect.DeepEqual(result, nodes) {
		t.Errorf("expected %#v to be %#v", result, nodes)
	}
}

func TestForceSet(t *testing.T) {
	t.Parallel()
	st := NewStore()

	d, err := dep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		&dep.Node{
			Node:    "node",
			Address: "address",
		},
	}

	st.forceSet(d.String(), nodes)

	data, ok := st.Recall(d)
	if !ok {
		t.Fatal("expected data from Store")
	}

	result := data.([]*dep.Node)
	if !reflect.DeepEqual(result, nodes) {
		t.Errorf("expected %#v to be %#v", result, nodes)
	}
}

func TestForget(t *testing.T) {
	t.Parallel()
	st := NewStore()

	d, err := dep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		&dep.Node{
			Node:    "node",
			Address: "address",
		},
	}

	st.Save(d, nodes)
	st.Delete(d)

	if _, ok := st.Recall(d); ok {
		t.Errorf("expected %#v to not be forgotten", d)
	}
}
