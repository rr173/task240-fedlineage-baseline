package service_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestTask240Bug03ParentDimensionMismatchIsForked(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Node.Register("m-root", "", "r1", "root", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Node.Confirm("m-root"); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Register("r1", "", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Register("r2", "r1", 16); !errors.Is(err, model.ErrDigestMismatch) {
		t.Fatalf("incompatible parent round was accepted: %v", err)
	}
	if _, err := sv.Node.Register("m-child", "m-root", "r1", "child", 16); !errors.Is(err, model.ErrDigestMismatch) {
		t.Fatalf("incompatible parent model was accepted: %v", err)
	}

	st, err := store.Open(t.TempDir() + "/legacy.db")
	if err != nil { t.Fatal(err) }
	defer st.Close()
	legacy := service.New(st)
	if err := st.PutModel(&model.GlobalModel{ID: "legacy-parent", RoundID: "legacy-r0", ParamDigest: "legacy", Dimension: 16, State: model.NodeStateConfirmed}); err != nil { t.Fatal(err) }
	if _, err := legacy.Round.Register("legacy-r1", "", 8); err != nil { t.Fatal(err) }
	if _, err := legacy.Round.Open("legacy-r1"); err != nil { t.Fatal(err) }
	if _, err := legacy.Update.Receive(&model.ClientUpdate{ID: "legacy-u1", RoundID: "legacy-r1", ClientID: "c1", ParentModel: "legacy-parent", ParamDigest: "legacy", Dimension: 8}); err != nil { t.Fatal(err) }
	if _, err := legacy.Round.Close("legacy-r1"); err != nil { t.Fatal(err) }
	vs, err := legacy.Lineage.VerifyRound("legacy-r1")
	if err != nil { t.Fatal(err) }
	if len(vs) != 1 || vs[0].Verdict != model.UpdateStateForked {
		t.Fatalf("dimension-incompatible parent was accepted: %#v", vs)
	}
}
