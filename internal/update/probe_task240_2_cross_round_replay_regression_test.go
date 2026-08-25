package update_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func TestTask240Bug02ReplayCannotMoveAcrossRounds(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/replay.db")
	if err != nil { t.Fatal(err) }
	defer st.Close()
	sv := service.New(st)
	for _, id := range []string{"r1", "r2"} {
		if _, err := sv.Round.Register(id, "", 8); err != nil { t.Fatal(err) }
		if _, err := sv.Round.Open(id); err != nil { t.Fatal(err) }
	}
	first := &model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParamDigest: "d", Dimension: 8}
	if _, err := sv.Update.Receive(first); err != nil { t.Fatal(err) }
	second := &model.ClientUpdate{ID: "u1", RoundID: "r2", ClientID: "c2", ParamDigest: "d2", Dimension: 8}
	if _, err := sv.Update.Receive(second); !errors.Is(err, model.ErrUpdateConflict) {
		t.Fatalf("cross-round replay was accepted: %v", err)
	}
	stored, err := sv.Update.Get("u1")
	if err != nil { t.Fatal(err) }
	if stored.RoundID != "r1" || stored.ClientID != "c1" || stored.ParamDigest != "d" {
		t.Fatalf("original update identity was moved: %#v", stored)
	}
}
