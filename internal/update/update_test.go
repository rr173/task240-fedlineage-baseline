package update_test

import (
	"errors"
	"sync"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestDuplicateUpdateCannotMoveAcrossRounds(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/update.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sv := service.New(st)
	for _, id := range []string{"r1", "r2"} {
		if _, err := sv.Round.Register(id, "", 8); err != nil {
			t.Fatal(err)
		}
		if _, err := sv.Round.Open(id); err != nil {
			t.Fatal(err)
		}
	}
	u := &model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParamDigest: "d", Dimension: 8}
	if _, err := sv.Update.Receive(u); err != nil {
		t.Fatal(err)
	}
	u.RoundID = "r2"
	if _, err := sv.Update.Receive(u); !errors.Is(err, model.ErrUpdateConflict) {
		t.Fatalf("cross-round duplicate accepted: %v", err)
	}
}

// TestConcurrentReceiveSingleFirstRecord 锁定并发修复：同一 ID 并发提交时
// 仅一个产生首次记录，其余成为重放。
func TestConcurrentReceiveSingleFirstRecord(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/update.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sv := service.New(st)
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	const n = 64
	var wg sync.WaitGroup
	type res struct {
		state string
		err   error
	}
	out := make(chan res, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := sv.Update.Receive(&model.ClientUpdate{
				ID: "u1", RoundID: "r1", ClientID: "c1",
				ParentModel: "", ParamDigest: "d", Dimension: 8,
			})
			if err != nil {
				out <- res{err: err}
				return
			}
			out <- res{state: r.State}
		}()
	}
	wg.Wait()
	close(out)
	var firsts, replays, errs int
	for r := range out {
		switch {
		case r.err != nil:
			errs++
		case r.state == model.UpdateStateNew:
			firsts++
		case r.state == model.UpdateStateReplay:
			replays++
		}
	}
	if firsts != 1 {
		t.Fatalf("expected exactly 1 first record, got %d (replays=%d errs=%d)", firsts, replays, errs)
	}
	if firsts+replays+errs != n {
		t.Fatalf("lost results: firsts=%d replays=%d errs=%d", firsts, replays, errs)
	}
}

// TestReceiveIdentityChangeReturnsConflict 锁定身份冲突修复：相同 ID 但身份字段
// 变化的重复提交返回冲突，而非普通重放；首记录身份不被篡改。
func TestReceiveIdentityChangeReturnsConflict(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/update.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sv := service.New(st)
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
	// 首记录不被篡改：状态仍为 new，身份字段为首次写入值。
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
