package service_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
)

func TestTask240Bug01SealedRoundRejectsUpdateMutation(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Node.Register("m-root", "", "r0", "d-root", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Node.Confirm("m-root"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Open("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParentModel: "m-root", ParamDigest: "d-root", Dimension: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Close("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Lineage.VerifyRound("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Aggregate.Confirm("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Snapshot.Publish("s1", "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Round.Seal("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Update.Isolate("u1", "late quarantine"); !errors.Is(err, model.ErrSealedMutation) {
		t.Fatalf("sealed round mutation was accepted: %v", err)
	}
	u, err := sv.Update.Get("u1")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != model.UpdateStateValid {
		t.Fatalf("sealed mutation changed update state to %s", u.State)
	}
}
