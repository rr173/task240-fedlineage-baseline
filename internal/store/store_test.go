package store_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

// TestStoreAtMostOnePublishedSnapshotPerRound 复现并发发布：同一轮次多次
// 发布不同快照 ID 时，仅第一次成功，其余得到 ErrSnapshotConflict，且库中
// publish 态快照唯一。
func TestStoreAtMostOnePublishedSnapshotPerRound(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/conflict.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mk := func(id string) *model.RoundSnapshot {
		return &model.RoundSnapshot{ID: id, RoundID: "r1", State: model.SnapshotStatePublish, Summary: "{}"}
	}
	if err := st.PutPublishedSnapshotIfAbsent(mk("s1")); err != nil {
		t.Fatalf("first publish failed: %v", err)
	}
	if err := st.PutPublishedSnapshotIfAbsent(mk("s2")); !errors.Is(err, model.ErrSnapshotConflict) {
		t.Fatalf("second publish should conflict, got %v", err)
	}
	published, err := st.GetPublishedSnapshot("r1")
	if err != nil || published.ID != "s1" {
		t.Fatalf("expected s1 to remain the winner, got %#v %v", published, err)
	}
	snaps, err := st.ListSnapshotsByRound("r1")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, s := range snaps {
		if s.State == model.SnapshotStatePublish {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly one publish snapshot, got %d", n)
	}
}

// TestStoreConcurrentPublishOnlyOneWinner 在同一轮次上并发发布，断言恰一
// 个请求成功，其余得到冲突结果（-race 下验证无数据竞争）。
func TestStoreConcurrentPublishOnlyOneWinner(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/race.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const goroutines = 16
	var wg sync.WaitGroup
	var ok, conflicts int32
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			snap := &model.RoundSnapshot{
				ID:      fmt.Sprintf("snap-%d", i),
				RoundID: "r-concurrent",
				State:   model.SnapshotStatePublish,
				Summary: "{}",
			}
			err := st.PutPublishedSnapshotIfAbsent(snap)
			if err == nil {
				atomic.AddInt32(&ok, 1)
				return
			}
			if errors.Is(err, model.ErrSnapshotConflict) {
				atomic.AddInt32(&conflicts, 1)
				return
			}
			t.Errorf("unexpected error: %v", err)
		}()
	}
	close(start)
	wg.Wait()

	if ok != 1 {
		t.Fatalf("expected exactly one winner, got %d", ok)
	}
	if conflicts != goroutines-1 {
		t.Fatalf("expected %d conflicts, got %d", goroutines-1, conflicts)
	}
	published, err := st.GetPublishedSnapshot("r-concurrent")
	if err != nil {
		t.Fatalf("expected a single published snapshot, got %v", err)
	}
	snaps, err := st.ListSnapshotsByRound("r-concurrent")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, s := range snaps {
		if s.State == model.SnapshotStatePublish {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly one publish row, got %d (winner=%s)", n, published.ID)
	}
}
