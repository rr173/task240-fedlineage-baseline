// Package aggregate 负责在被校验的更新集合上形成可聚合集合：
// 排除隔离/分叉/重放更新，计算可聚合集合并标记轮次为可聚合。
package aggregate

import (
	"encoding/json"
	"fmt"
	"sort"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/store"
)

// Service 聚合服务。
type Service struct {
	store *store.Store
	round *round.Service
}

// New 构造聚合服务。
func New(s *store.Store, rs *round.Service) *Service {
	return &Service{store: s, round: rs}
}

// AggregableSet 表示一个可聚合集合的摘要。
type AggregableSet struct {
	RoundID     string           `json:"round_id"`
	UpdateCount int              `json:"update_count"`
	UpdateIDs   []string         `json:"update_ids"`
	Excluded    []ExcludedUpdate `json:"excluded"`
	ExpectedDim int              `json:"expected_dim"`
}

// ExcludedUpdate 描述被排除的更新及原因。
type ExcludedUpdate struct {
	UpdateID string `json:"update_id"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
}

// Compute 计算某轮次的可聚合集合：仅纳入状态为 valid 的更新。
func (s *Service) Compute(roundID string) (*AggregableSet, error) {
	r, err := s.round.Get(roundID)
	if err != nil {
		return nil, err
	}
	if r.State != model.RoundStateValidating && r.State != model.RoundStateAggregable {
		return nil, fmt.Errorf("%w: round %s in state %s", model.ErrInvalidState, roundID, r.State)
	}
	us, err := s.store.ListUpdatesByRound(roundID)
	if err != nil {
		return nil, err
	}
	set := &AggregableSet{RoundID: roundID, ExpectedDim: r.ExpectedDim}
	for _, u := range us {
		switch u.State {
		case model.UpdateStateValid, model.UpdateStateForked:
			set.UpdateCount++
			set.UpdateIDs = append(set.UpdateIDs, u.ID)
		default:
			set.Excluded = append(set.Excluded, ExcludedUpdate{UpdateID: u.ID, State: u.State, Reason: u.Reason})
		}
	}
	sort.Strings(set.UpdateIDs)
	sort.Slice(set.Excluded, func(i, j int) bool {
		return set.Excluded[i].UpdateID < set.Excluded[j].UpdateID
	})
	return set, nil
}

// Confirm 确认可聚合集合，并将轮次标记为可聚合。
func (s *Service) Confirm(roundID string) (*AggregableSet, error) {
	set, err := s.Compute(roundID)
	if err != nil {
		return nil, err
	}
	if _, err := s.round.MarkAggregable(roundID); err != nil {
		return nil, err
	}
	return set, nil
}

// Summary 将可聚合集合序列化为摘要 JSON。
func (set *AggregableSet) Summary() (string, error) {
	b, err := json.Marshal(set)
	if err != nil {
		return "", fmt.Errorf("marshal aggregable set: %w", err)
	}
	return string(b), nil
}
