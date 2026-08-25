package service_test

import (
	"strings"
	"testing"

	"task240-fedlineage/internal/model"
)

func TestTask240Bug04ForkedUpdateExcludedFromSnapshot(t *testing.T) {
	sv := newTestServices(t)
	if _, err := sv.Node.Register("m-root", "", "r0", "root", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Node.Confirm("m-root"); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Register("r1", "", 8); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Open("r1"); err != nil { t.Fatal(err) }
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-valid", RoundID: "r1", ClientID: "c1", ParentModel: "m-root", ParamDigest: "root", Dimension: 8}); err != nil { t.Fatal(err) }
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "u-fork", RoundID: "r1", ClientID: "c2", ParentModel: "m-root", ParamDigest: "fork", Dimension: 4}); err != nil { t.Fatal(err) }
	if _, err := sv.Round.Close("r1"); err != nil { t.Fatal(err) }
	if _, err := sv.Lineage.VerifyRound("r1"); err != nil { t.Fatal(err) }
	forked, err := sv.Update.Get("u-fork")
	if err != nil { t.Fatal(err) }
	if forked.State != model.UpdateStateForked { t.Fatalf("forked update state was not preserved: %s", forked.State) }
	if _, err := sv.Aggregate.Confirm("r1"); err != nil { t.Fatal(err) }
	snap, err := sv.Snapshot.Publish("s1", "r1")
	if err != nil { t.Fatal(err) }
	if strings.Contains(snap.Summary, "u-fork") { t.Fatalf("forked update leaked into snapshot: %s", snap.Summary) }
}
