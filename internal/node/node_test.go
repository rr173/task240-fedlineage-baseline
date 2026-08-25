package node_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/node"
	"task240-fedlineage/internal/store"
)

func TestRegisterRequiresExistingCompatibleParent(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/node.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := node.New(st)
	if _, err := svc.Register("root", "", "r1", "d", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register("missing-parent", "unknown", "r1", "d", 8); !errors.Is(err, model.ErrParentMissing) {
		t.Fatalf("missing parent accepted: %v", err)
	}
	if _, err := svc.Register("wrong-dim", "root", "r1", "d", 16); !errors.Is(err, model.ErrDigestMismatch) {
		t.Fatalf("parent dimension mismatch accepted: %v", err)
	}
	if _, err := svc.Register("child", "root", "r1", "d", 8); err != nil {
		t.Fatal(err)
	}
	cycle, err := svc.DetectCycle("root", "child")
	if err != nil || !cycle {
		t.Fatalf("expected cycle detection, got %v %v", cycle, err)
	}
}
