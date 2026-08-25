package lineage_test

import (
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

// newServices constructs an isolated set of services backed by a temp DB.
func newServices(t *testing.T) *service.Services {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/lineage.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return service.New(st)
}

// TestVerifyRejectsIncompatibleParentModelDimension 复现校验端缺口：
// 更新维度与轮次期望一致，但声称的父模型参数维度与更新不符。
// 这类不兼容谱系关系必须被判为分叉，而非 valid。
func TestVerifyRejectsIncompatibleParentModelDimension(t *testing.T) {
	sv := newServices(t)
	// 根模型维度 100，确认。
	if _, err := sv.Node.Register("m-root", "", "r0", "d-root", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Node.Confirm("m-root"); err != nil {
		t.Fatal(err)
	}
	// 轮次期望维度 100。
	if _, err := sv.Round.Register("r1", "", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	// 更新维度与轮次期望一致（100），但声明的父模型维度也是 100 —— 先保证基线 valid。
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-ok", RoundID: "r1", ClientID: "c1", ParentModel: "m-root", ParamDigest: "d", Dimension: 100}); err != nil {
		t.Fatal(err)
	}
	// 构造一个维度与父模型不兼容的更新：维度仍为 100（匹配轮次），
	// 但父模型 m-root 维度 100 —— 通过登记一个不同维度的确认父模型来构造不兼容关系。
	if _, err := sv.Node.Register("m-other", "", "r0b", "d-other", 64); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Node.Confirm("m-other"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-fork", RoundID: "r1", ClientID: "c2", ParentModel: "m-other", ParamDigest: "d2", Dimension: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Close("r1"); err != nil {
		t.Fatal(err)
	}
	vs, err := sv.Lineage.VerifyRound("r1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"u-ok":   model.UpdateStateValid,
		"u-fork": model.UpdateStateForked,
	}
	got := map[string]string{}
	for _, v := range vs {
		got[v.UpdateID] = v.Verdict
	}
	for id, expected := range want {
		if got[id] != expected {
			t.Fatalf("update %s verdict = %s, want %s", id, got[id], expected)
		}
	}
	forks, err := sv.Lineage.ForkedUpdates("r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(forks) != 1 || forks[0].ID != "u-fork" {
		t.Fatalf("expected single fork u-fork, got %#v", forks)
	}
}
