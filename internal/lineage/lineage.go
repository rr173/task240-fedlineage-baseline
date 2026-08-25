// Package lineage 负责客户端更新的谱系校验：验证父模型关系、
// 参数形状一致性与分叉/重放检测，并写出校验结论。
package lineage

import (
	"fmt"
	"time"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/node"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/store"
	"task240-fedlineage/internal/update"
)

// Service 谱系校验服务。
type Service struct {
	store  *store.Store
	node   *node.Service
	round  *round.Service
	update *update.Service
	now    func() time.Time
}

// New 构造谱系校验服务。
func New(s *store.Store, ns *node.Service, rs *round.Service, us *update.Service) *Service {
	return &Service{store: s, node: ns, round: rs, update: us, now: time.Now}
}

// Verify 校验一个更新：检查其父模型是否存在、参数形状是否与轮次期望一致、
// 是否构成分叉（父模型不在已确认谱系内）。结果写入校验表与更新状态。
func (s *Service) Verify(updateID string) (*model.UpdateVerification, error) {
	u, err := s.store.GetUpdate(updateID)
	if err != nil {
		return nil, err
	}
	// 已隔离或已重放则不重复校验。
	if u.State == model.UpdateStateIsolated || u.State == model.UpdateStateReplay {
		return &model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: u.State,
			Reason: "skipped: terminal state", VerifiedAt: s.now().UTC()}, nil
	}
	r, err := s.round.Get(u.RoundID)
	if err != nil {
		return nil, err
	}
	// 封存轮次只读：聚合证据已冻结，禁止任何状态改写，仅返回当前判定。
	if r.State == model.RoundStateSealed {
		return &model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: u.State,
			Reason: "skipped: sealed round is read-only", VerifiedAt: s.now().UTC()}, nil
	}
	if r.State != model.RoundStateValidating && r.State != model.RoundStateAggregable {
		return nil, fmt.Errorf("%w: round %s is not ready for verification", model.ErrInvalidState, r.ID)
	}
	// 形状校验：维度必须等于轮次期望。
	if u.Dimension != r.ExpectedDim {
		v := model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: model.UpdateStateForked,
			Reason: fmt.Sprintf("dimension %d != expected %d", u.Dimension, r.ExpectedDim), VerifiedAt: s.now().UTC()}
		if err := s.record(u, model.UpdateStateForked, v.Reason); err != nil {
			return nil, err
		}
		return &v, nil
	}
	// 父模型校验：声明父模型必须存在且处于确认态。
	if u.ParentModel != "" {
		pm, err := s.node.Get(u.ParentModel)
		if err != nil {
			v := model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: model.UpdateStateForked,
				Reason: "declared parent model not found", VerifiedAt: s.now().UTC()}
			if err := s.record(u, model.UpdateStateForked, v.Reason); err != nil {
				return nil, err
			}
			return &v, nil
		}
		if pm.State != model.NodeStateConfirmed {
			v := model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: model.UpdateStateForked,
				Reason: "parent model not confirmed", VerifiedAt: s.now().UTC()}
			if err := s.record(u, model.UpdateStateForked, v.Reason); err != nil {
				return nil, err
			}
			return &v, nil
		}
		// 形状指纹校验：父模型参数摘要维度指纹需与更新一致。
		if pm.Dimension != u.Dimension {
			v := model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: model.UpdateStateForked,
				Reason: "parent model dimension mismatch", VerifiedAt: s.now().UTC()}
			if err := s.record(u, model.UpdateStateForked, v.Reason); err != nil {
				return nil, err
			}
			return &v, nil
		}
	}
	v := model.UpdateVerification{UpdateID: u.ID, RoundID: u.RoundID, Verdict: model.UpdateStateValid,
		Reason: "parent relation and shape consistent", VerifiedAt: s.now().UTC()}
	if err := s.record(u, model.UpdateStateValid, v.Reason); err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Service) record(u *model.ClientUpdate, state, reason string) error {
	u.State = state
	u.Reason = reason
	if err := s.store.PutUpdate(u); err != nil {
		return err
	}
	return s.store.PutVerification(model.UpdateVerification{
		UpdateID: u.ID, RoundID: u.RoundID, Verdict: state, Reason: reason, VerifiedAt: s.now().UTC(),
	})
}

// VerifyRound 校验某轮次全部未隔离/未重放更新，返回结论列表。
func (s *Service) VerifyRound(roundID string) ([]model.UpdateVerification, error) {
	us, err := s.store.ListUpdatesByRound(roundID)
	if err != nil {
		return nil, err
	}
	out := []model.UpdateVerification{}
	for _, u := range us {
		if u.State == model.UpdateStateIsolated || u.State == model.UpdateStateReplay {
			continue
		}
		v, err := s.Verify(u.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, nil
}

// ForkedUpdates 返回某轮次被判定为分叉的更新。
func (s *Service) ForkedUpdates(roundID string) ([]*model.ClientUpdate, error) {
	us, err := s.store.ListUpdatesByRound(roundID)
	if err != nil {
		return nil, err
	}
	out := []*model.ClientUpdate{}
	for _, u := range us {
		if u.State == model.UpdateStateForked {
			out = append(out, u)
		}
	}
	return out, nil
}
