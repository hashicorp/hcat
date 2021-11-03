package hcat

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

func TestNewStore(t *testing.T) {
	t.Parallel()
	st := NewStore()

	if st.data == nil {
		t.Errorf("expected data to not be nil")
	}
}

func TestRecall(t *testing.T) {
	t.Parallel()
	st := NewStore()

	d, err := idep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		{
			Node:    "node",
			Address: "address",
		},
	}

	id := d.ID()
	st.Save(id, nodes)

	data, ok := st.Recall(id)
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

	d, err := idep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		{
			Node:    "node",
			Address: "address",
		},
	}

	st.forceSet(d.ID(), nodes)

	data, ok := st.Recall(d.ID())
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

	d, err := idep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		{
			Node:    "node",
			Address: "address",
		},
	}

	id := d.ID()
	st.Save(id, nodes)
	st.Delete(id)

	if _, ok := st.Recall(id); ok {
		t.Errorf("expected %#v to not be forgotten", d)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	st := NewStore()

	d, err := idep.NewCatalogNodesQuery("")
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*dep.Node{
		{
			Node:    "node",
			Address: "address",
		},
	}

	id := d.ID()
	st.Save(id, nodes)
	st.Reset()

	if _, ok := st.Recall(id); ok {
		t.Errorf("expected %#v to not be forgotten", d)
	}
}
