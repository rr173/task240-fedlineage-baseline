package service_test

import (
	"errors"
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
	// 重放 u1：身份字段完全一致，仅作为去重重放。
	r, err := sv.Update.Receive(upd("u1", "c1"))
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

// TestConcurrentReceiveProducesSingleFirstRecord 锁定并发修复：多个 goroutine
// 同时提交同一 ID 时，仅一个产生首次记录（new），其余成为重放（replay）。
func TestConcurrentReceiveProducesSingleFirstRecord(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	const n = 32
	type result struct {
		state string
		err   error
	}
	res := make(chan result, n)
	for i := 0; i < n; i++ {
		go func() {
			r, err := sv.Update.Receive(&model.ClientUpdate{
				ID: "u1", RoundID: "r1", ClientID: "c1",
				ParentModel: "", ParamDigest: "d", Dimension: 8,
			})
			if err != nil {
				res <- result{err: err}
				return
			}
			res <- result{state: r.State}
		}()
	}
	var firsts, replays, errs int
	for i := 0; i < n; i++ {
		got := <-res
		switch {
		case got.err != nil:
			errs++
		case got.state == model.UpdateStateNew:
			firsts++
		case got.state == model.UpdateStateReplay:
			replays++
		}
	}
	if firsts != 1 {
		t.Fatalf("expected exactly 1 first record, got %d (replays=%d errs=%d)", firsts, replays, errs)
	}
	if firsts+replays+errs != n {
		t.Fatalf("lost results: firsts=%d replays=%d errs=%d total=%d", firsts, replays, errs, n)
	}
	// 库中该 ID 仅一行，状态为 replay（被并发重放者覆写）。
	got, err := sv.Update.Get("u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.UpdateStateReplay {
		t.Fatalf("expected persisted replay, got %s", got.State)
	}
}

// TestReceiveIdentityChangeReturnsConflict 锁定身份冲突修复：相同 ID 但身份字段
// （客户端/参数摘要/维度/父模型）变化的重复提交返回冲突，而非普通重放。
func TestReceiveIdentityChangeReturnsConflict(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	base := func() *model.ClientUpdate {
		return &model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParentModel: "", ParamDigest: "d", Dimension: 8}
	}
	if _, err := sv.Update.Receive(base()); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		mut  func(u *model.ClientUpdate)
	}{
		{"client_id", func(u *model.ClientUpdate) { u.ClientID = "c2" }},
		{"param_digest", func(u *model.ClientUpdate) { u.ParamDigest = "d2" }},
		{"dimension", func(u *model.ClientUpdate) { u.Dimension = 16 }},
		{"parent_model", func(u *model.ClientUpdate) { u.ParentModel = "m0" }},
	}
	for _, c := range cases {
		u := base()
		c.mut(u)
		if _, err := sv.Update.Receive(u); !errors.Is(err, model.ErrUpdateConflict) {
			t.Fatalf("%s: expected conflict, got %v", c.name, err)
		}
	}
	// 首记录本身不被篡改：状态仍是 new，身份仍是首次写入值。
	got, err := sv.Update.Get("u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.UpdateStateNew {
		t.Fatalf("first record mutated to %s", got.State)
	}
	if got.ClientID != "c1" || got.ParamDigest != "d" || got.Dimension != 8 || got.ParentModel != "" {
		t.Fatalf("first record identity mutated: %#v", got)
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
