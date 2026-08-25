package service_test

import (
	"path/filepath"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func newTestServices(t *testing.T) *service.Services {
	t.Helper()
	db := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return service.New(st)
}

// TestEndToEndReplay 复现 REQ 端到端场景：客户端重放旧轮次更新被隔离，移除后聚合可发布。
func TestEndToEndReplay(t *testing.T) {
	sv := newTestServices(t)

	if _, err := sv.Node.Register("m-root", "", "r0", "d-root", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Node.Confirm("m-root"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Register("r1", "", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	// 两个正常更新。
	upd := func(id, client string) *model.ClientUpdate {
		return &model.ClientUpdate{ID: id, RoundID: "r1", ClientID: client, ParentModel: "m-root", ParamDigest: "d-root", Dimension: 100}
	}
	if _, err := sv.Update.Receive(upd("u1", "c1")); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Update.Receive(upd("u2", "c2")); err != nil {
		t.Fatal(err)
	}
	// 重放 u1。
	r, err := sv.Update.Receive(upd("u1", "c3"))
	if err != nil {
		t.Fatal(err)
	}
	if r.State != model.UpdateStateReplay {
		t.Fatalf("expected replay, got %s", r.State)
	}
	// 关闭并校验。
	if _, err := sv.Round.Close("r1"); err != nil {
		t.Fatal(err)
	}
	vs, err := sv.Lineage.VerifyRound("r1")
	if err != nil {
		t.Fatal(err)
	}
	// 重放的 u1 已被覆写为 replay 终态并跳过校验，仅 u2 参与校验 → 1 条。
	if len(vs) != 1 {
		t.Fatalf("expected 1 verification (replay skipped), got %d", len(vs))
	}
	// 隔离重放更新，移除后聚合集合仅含 u2。
	if _, err := sv.Update.Isolate("u1", "replayed old round update"); err != nil {
		t.Fatal(err)
	}
	// 聚合确认 + 快照发布。
	if _, err := sv.Aggregate.Confirm("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Snapshot.Publish("s1", "r1"); err != nil {
		t.Fatal(err)
	}
	// 封存。
	if _, err := sv.Round.Seal("r1"); err != nil {
		t.Fatal(err)
	}
	rnd, err := sv.Round.Get("r1")
	if err != nil {
		t.Fatal(err)
	}
	if rnd.State != model.RoundStateSealed {
		t.Fatalf("expected sealed, got %s", rnd.State)
	}
}

// TestForkDetection 检测维度不匹配的更新被判为分叉。
func TestForkDetection(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Node.Register("m0", "", "r0", "d0", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Node.Confirm("m0"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Register("r2", "", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r2"); err != nil {
		t.Fatal(err)
	}
	// 维度 50 与期望 100 不符 → 分叉。
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "fx", RoundID: "r2", ClientID: "c", ParentModel: "m0", ParamDigest: "dx", Dimension: 50}); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Close("r2"); err != nil {
		t.Fatal(err)
	}
	// 必须先校验，分叉状态才会被写入。
	if _, err := sv.Lineage.VerifyRound("r2"); err != nil {
		t.Fatal(err)
	}
	forks, err := sv.Lineage.ForkedUpdates("r2")
	if err != nil {
		t.Fatal(err)
	}
	if len(forks) != 1 {
		t.Fatalf("expected 1 fork, got %d", len(forks))
	}
}

// TestSelfCheck 自检返回健康且计数一致。
func TestSelfCheck(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Node.Register("m1", "", "r0", "d1", 10); err != nil {
		t.Fatal(err)
	}
	sc, err := sv.SelfCheck()
	if err != nil {
		t.Fatal(err)
	}
	if !sc.OK {
		t.Fatal("self check not ok")
	}
	if sc.Models != 1 {
		t.Fatalf("expected 1 model, got %d", sc.Models)
	}
}
