// Package snapshot 负责轮次谱系快照的发布：在轮次可聚合后生成不可变快照，
// 封存后可追溯、不可被重放覆盖。
package snapshot

import (
	"fmt"
	"time"

	"task240-fedlineage/internal/aggregate"
	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/store"
)

// Service 快照服务。
type Service struct {
	store     *store.Store
	round     *round.Service
	aggregate *aggregate.Service
	now       func() time.Time
}

// New 构造快照服务。
func New(s *store.Store, rs *round.Service, as *aggregate.Service) *Service {
	return &Service{store: s, round: rs, aggregate: as, now: time.Now}
}

// Publish 发布某轮次的不可变快照（需轮次处于可聚合或封存态）。
func (s *Service) Publish(id, roundID string) (*model.RoundSnapshot, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: snapshot id empty", model.ErrDuplicateID)
	}
	r, err := s.round.Get(roundID)
	if err != nil {
		return nil, err
	}
	if r.State != model.RoundStateAggregable && r.State != model.RoundStateSealed {
		return nil, fmt.Errorf("%w: round %s in state %s cannot publish", model.ErrInvalidState, roundID, r.State)
	}
	if _, err := s.store.GetSnapshot(id); err == nil {
		return nil, fmt.Errorf("%w: snapshot %s", model.ErrDuplicateID, id)
	}
	set, err := s.aggregate.Compute(roundID)
	if err != nil {
		return nil, err
	}
	summary, err := set.Summary()
	if err != nil {
		return nil, err
	}
	snap := &model.RoundSnapshot{ID: id, RoundID: roundID, State: model.SnapshotStatePublish, Summary: summary, CreatedAt: s.now().UTC()}
	if err := s.store.PutSnapshot(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// Get 读取快照。
func (s *Service) Get(id string) (*model.RoundSnapshot, error) {
	return s.store.GetSnapshot(id)
}

// Published 返回某轮次已发布的快照。
func (s *Service) Published(roundID string) (*model.RoundSnapshot, error) {
	return s.store.GetPublishedSnapshot(roundID)
}

// ListByRound 列出某轮次全部快照。
func (s *Service) ListByRound(roundID string) ([]*model.RoundSnapshot, error) {
	return s.store.ListSnapshotsByRound(roundID)
}

// Supersede 将一个已发布快照标记为被替代（不删除，保留可追溯）。
func (s *Service) Supersede(id string) (*model.RoundSnapshot, error) {
	snap, err := s.store.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if snap.State != model.SnapshotStatePublish {
		return nil, fmt.Errorf("%w: snapshot %s not published", model.ErrInvalidState, id)
	}
	snap.State = model.SnapshotStateSupersede
	if err := s.store.PutSnapshot(snap); err != nil {
		return nil, err
	}
	return snap, nil
}
