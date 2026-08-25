package model_test

import (
	"errors"
	"testing"

	"task240-fedlineage/internal/model"
)

func TestValidateDimensionAndUpdateInput(t *testing.T) {
	if err := model.ValidateDimension(0); !errors.Is(err, model.ErrInvalidDimension) {
		t.Fatalf("expected invalid dimension, got %v", err)
	}
	u := &model.ClientUpdate{ID: "u1", RoundID: "r1", ClientID: "c1", ParamDigest: "d", Dimension: 4}
	if err := model.ValidateUpdateInput(u); err != nil {
		t.Fatalf("valid update rejected: %v", err)
	}
	u.ClientID = ""
	if err := model.ValidateUpdateInput(u); !errors.Is(err, model.ErrParamMissing) {
		t.Fatalf("missing client should be rejected, got %v", err)
	}
}

func TestStateTransitionsRejectTerminalMutation(t *testing.T) {
	if err := model.ValidateRoundTransition(model.RoundStateSealed, model.RoundStateReceiving); !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("sealed round accepted mutation: %v", err)
	}
	if err := model.ValidateUpdateTransition(model.UpdateStateValid, model.UpdateStateIsolated); err != nil {
		t.Fatalf("valid update should be isolatable: %v", err)
	}
	if err := model.ValidateUpdateTransition(model.UpdateStateIsolated, model.UpdateStateValid); !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("isolated update accepted reverse transition: %v", err)
	}
}

func TestValidateSnapshotPublicationRejectsExistingPublish(t *testing.T) {
	// 已有 publish 态快照 → 第二次发布应被拒绝为冲突。
	if err := model.ValidateSnapshotPublication(model.SnapshotStatePublish); !errors.Is(err, model.ErrSnapshotConflict) {
		t.Fatalf("existing publish should conflict, got %v", err)
	}
	// 草稿或被替代态不阻塞重新发布。
	for _, st := range []string{model.SnapshotStateDraft, model.SnapshotStateSupersede} {
		if err := model.ValidateSnapshotPublication(st); err != nil {
			t.Fatalf("state %s should not block publication, got %v", st, err)
		}
	}
}
