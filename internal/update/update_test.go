package update_test

import (
	"errors"
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
