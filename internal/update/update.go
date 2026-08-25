// Package update 负责客户端更新的接收与幂等去重。
// 写入即落库，更新 ID 为幂等键；轮次停止接收后拒绝新写入。
package update

import (
	"fmt"
	"sync"
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

	// mu serializes the check-then-update replay path so that, once a first
	// record exists for an ID, every concurrent resubmission reads the same
	// committed row rather than observing a half-written state. The first-write
	// race itself is resolved atomically by store.ClaimUpdateInsert, so this
	// lock only guards the replay/conflict re-read and state mutation.
	mu sync.Mutex
}

// New 构造更新接收服务。
func New(s *store.Store, rs *round.Service) *Service {
	return &Service{store: s, round: rs, now: time.Now}
}

// Receive 接收一个客户端更新。幂等：相同 ID 的重复写入视为重放。
// 仅做接收与去重，谱系校验由 lineage 包完成。
//
// 并发安全：多个 goroutine 同时提交同一 ID 时，仅一个产生首次记录，
// 其余内容相同的请求成为重放；身份字段变化的重复提交返回冲突。
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

	// 原子地抢占首次写入：INSERT ... ON CONFLICT DO NOTHING 保证同一 ID
	// 在并发下只有一个调用者真正插入新行（created == true）。所有竞争失败者
	// 落到 created == false，进入重放/冲突判定分支。这一步消除了原来的
	// “先 GetUpdate 再 PutUpdate” 之间的竞态窗口。
	u.State = model.UpdateStateNew
	u.CreatedAt = s.now().UTC()
	created, err := s.store.ClaimUpdateInsert(u)
	if err != nil {
		return nil, err
	}
	if created {
		return u, nil
	}

	// 该 ID 已存在：内容相同 → 重放，身份字段变化 → 冲突。
	// 串行化重放路径，使并发重放者读到同一行已提交记录并基于其真实身份判定，
	// 而不是某个并发写入者尚未落库的中间态。
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.store.GetUpdate(u.ID)
	if err != nil {
		// 不应发生：ClaimUpdateInsert 报告非首次写入却读不到行。
		return nil, err
	}
	if err := model.ValidateReplayIdentity(existing, u); err != nil {
		return nil, err
	}
	// 内容相同 → 标记为重放并记录。
	existing.State = model.UpdateStateReplay
	existing.Reason = "duplicate update id within replay window"
	if err := s.store.PutUpdate(existing); err != nil {
		return nil, err
	}
	return existing, nil
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
