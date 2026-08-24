// Package round 维护聚合轮次状态机与生命周期。
// 负责轮次的登记、开放接收、停止接收与封存，并对状态迁移做约束校验。
package round

import (
	"fmt"
	"time"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/store"
)

// Service 轮次服务。
type Service struct {
	store *store.Store
	now   func() time.Time
}

// New 构造轮次服务。
func New(s *store.Store) *Service {
	return &Service{store: s, now: time.Now}
}

// Register 登记一个新轮次（preparing 状态，可指定父轮次）。
func (s *Service) Register(id, parentRound string, expectedDim int) (*model.AggregateRound, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: round id empty", model.ErrDuplicateID)
	}
	if _, err := s.store.GetRound(id); err == nil {
		return nil, fmt.Errorf("%w: round %s", model.ErrDuplicateID, id)
	}
	r := &model.AggregateRound{
		ID:          id,
		ParentRound: parentRound,
		State:       model.RoundStatePreparing,
		ExpectedDim: expectedDim,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.store.PutRound(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Open 将轮次置为接收中（开放客户端上报）。
func (s *Service) Open(id string) (*model.AggregateRound, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateRoundTransition(r.State, model.RoundStateReceiving); err != nil {
		return nil, err
	}
	r.State = model.RoundStateReceiving
	if err := s.store.PutRound(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Close 停止接收（置为待校验）。
func (s *Service) Close(id string) (*model.AggregateRound, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateRoundTransition(r.State, model.RoundStateValidating); err != nil {
		return nil, err
	}
	r.State = model.RoundStateValidating
	r.ClosedAt = s.now().UTC()
	if err := s.store.PutRound(r); err != nil {
		return nil, err
	}
	return r, nil
}

// MarkAggregable 校验通过，标记可聚合。
func (s *Service) MarkAggregable(id string) (*model.AggregateRound, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateRoundTransition(r.State, model.RoundStateAggregable); err != nil {
		return nil, err
	}
	r.State = model.RoundStateAggregable
	if err := s.store.PutRound(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Seal 封存轮次（终态，冻结修改）。
func (s *Service) Seal(id string) (*model.AggregateRound, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateRoundTransition(r.State, model.RoundStateSealed); err != nil {
		return nil, err
	}
	r.State = model.RoundStateSealed
	r.SealedAt = s.now().UTC()
	if err := s.store.PutRound(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Get 读取轮次。
func (s *Service) Get(id string) (*model.AggregateRound, error) {
	return s.store.GetRound(id)
}

// List 列出全部轮次。
func (s *Service) List() ([]*model.AggregateRound, error) {
	return s.store.ListRounds()
}

// IsReceiving 判断轮次是否仍开放接收。
func (s *Service) IsReceiving(id string) (bool, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return false, err
	}
	return r.State == model.RoundStateReceiving, nil
}

// IsSealed 判断轮次是否已封存。
func (s *Service) IsSealed(id string) (bool, error) {
	r, err := s.store.GetRound(id)
	if err != nil {
		return false, err
	}
	return r.State == model.RoundStateSealed, nil
}
