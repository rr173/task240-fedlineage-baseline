package store_test

import (
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/store"
)

func TestStorePersistsEdgesAndSnapshots(t *testing.T) {
	path := t.TempDir() + "/lineage.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutModel(&model.GlobalModel{ID: "m1", RoundID: "r1", ParamDigest: "d1", Dimension: 8, State: model.NodeStateConfirmed}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutModel(&model.GlobalModel{ID: "m2", ParentID: "m1", RoundID: "r2", ParamDigest: "d2", Dimension: 8, State: model.NodeStateCandidate}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutEdge(model.LineageEdge{Child: "m2", Parent: "m1"}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSnapshot(&model.RoundSnapshot{ID: "s1", RoundID: "r1", State: model.SnapshotStatePublish, Summary: "{}"}); err != nil {
		t.Fatal(err)
	}
	edges, err := st.AllEdges()
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].Child != "m2" || edges[0].Parent != "m1" {
		t.Fatalf("unexpected edges: %#v", edges)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	snap, err := st.GetSnapshot("s1")
	if err != nil || snap.State != model.SnapshotStatePublish {
		t.Fatalf("snapshot did not survive reopen: %#v %v", snap, err)
	}
}
