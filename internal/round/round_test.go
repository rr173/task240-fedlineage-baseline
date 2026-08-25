package round_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/store"
)

func TestRoundParentDimensionAndLifecycle(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/round.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := round.New(st)
	if _, err := svc.Register("r1", "", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register("r2", "r1", 16); !errors.Is(err, model.ErrDigestMismatch) {
		t.Fatalf("incompatible parent accepted: %v", err)
	}
	if _, err := svc.Register("r2", "missing", 8); !errors.Is(err, model.ErrParentMissing) {
		t.Fatalf("missing parent accepted: %v", err)
	}
	if _, err := svc.Open("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Close("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Seal("r1"); !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("round sealed before aggregable: %v", err)
	}
}
