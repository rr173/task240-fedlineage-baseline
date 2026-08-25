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

// TestValidateUpdateMutationRejectsSealedRound 确认封存轮次上的更新迁移被拒绝：
// 聚合证据已冻结，任何状态改写都应返回 ErrSealedMutation。
func TestValidateUpdateMutationRejectsSealedRound(t *testing.T) {
	// 可变轮次：合法迁移放行。
	if err := model.ValidateUpdateMutation(model.RoundStateAggregable, model.UpdateStateValid, model.UpdateStateIsolated); err != nil {
		t.Fatalf("valid update in mutable round should be isolatable: %v", err)
	}
	// 封存轮次：无论原本迁移是否合法，一律拒绝并返回 ErrSealedMutation。
	if err := model.ValidateUpdateMutation(model.RoundStateSealed, model.UpdateStateValid, model.UpdateStateIsolated); !errors.Is(err, model.ErrSealedMutation) {
		t.Fatalf("sealed round accepted mutation: %v", err)
	}
	if err := model.ValidateUpdateMutation(model.RoundStateSealed, model.UpdateStateNew, model.UpdateStateValid); !errors.Is(err, model.ErrSealedMutation) {
		t.Fatalf("sealed round accepted verify-time mutation: %v", err)
	}
}
