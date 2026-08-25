// Package update 负责客户端更新的接收与幂等去重。
// 写入即落库，更新 ID 为幂等键；轮次停止接收后拒绝新写入。
package update

import (
	"fmt"
	"time"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/store"
)

// Service 更新接收服务。
type Service struct {
	store *store.Store
	round *round.Service
	now   func() time.Time
}

// New 构造更新接收服务。
func New(s *store.Store, rs *round.Service) *Service {
	return &Service{store: s, round: rs, now: time.Now}
}

// Receive 接收一个客户端更新。幂等：相同 ID 的重复写入视为重放。
// 仅做接收与去重，谱系校验由 lineage 包完成。
func (s *Service) Receive(u *model.ClientUpdate) (*model.ClientUpdate, error) {
	if err := model.ValidateUpdateInput(u); err != nil {
		return nil, err
	}
	receiving, err := s.round.IsReceiving(u.RoundID)
	if err != nil {
		return nil, err
	}
	if !receiving {
		return nil, fmt.Errorf("%w: round %s", model.ErrRoundClosed, u.RoundID)
	}
	// 幂等检查：已存在相同 ID 更新 → 重放。
	existing, err := s.store.GetUpdate(u.ID)
	if err == nil {
		if existing.RoundID != u.RoundID {
			return nil, fmt.Errorf("%w: update %s belongs to round %s", model.ErrUpdateConflict, u.ID, existing.RoundID)
		}
		// 更新为 replay 状态并记录。
		existing.State = model.UpdateStateReplay
		existing.Reason = "duplicate update id within replay window"
		if err := s.store.PutUpdate(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != model.ErrNotFound {
		return nil, err
	}
	u.State = model.UpdateStateNew
	u.CreatedAt = s.now().UTC()
	if err := s.store.PutUpdate(u); err != nil {
		return nil, err
	}
	return u, nil
}

// Get 读取更新。
func (s *Service) Get(id string) (*model.ClientUpdate, error) {
	return s.store.GetUpdate(id)
}

// ListByRound 列出某轮次全部更新。
func (s *Service) ListByRound(roundID string) ([]*model.ClientUpdate, error) {
	return s.store.ListUpdatesByRound(roundID)
}

// Isolate 主动隔离一个更新（研究员判定其异常）。
func (s *Service) Isolate(id, reason string) (*model.ClientUpdate, error) {
	u, err := s.store.GetUpdate(id)
	if err != nil {
		return nil, err
	}
	if u.State == model.UpdateStateIsolated {
		return u, nil
	}
	if err := s.round.ValidateUpdateMutation(u.RoundID, u.State, model.UpdateStateIsolated); err != nil {
		// 允许从 new/valid/replay/forked 隔离。
		return nil, err
	}
	u.State = model.UpdateStateIsolated
	u.Reason = reason
	if err := s.store.PutUpdate(u); err != nil {
		return nil, err
	}
	return u, nil
}
