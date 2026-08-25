// Package model 定义联邦学习模型更新谱系一致性服务领域的核心实体与错误。
//
// 该服务校验多轮联邦聚合中客户端更新的合法祖先关系：模型参数摘要、
// 轮次父节点与聚合输入的一致性。它不是训练调度器，而是一个可验证谱系
// 的校验与封存后端。
package model

import (
	"errors"
	"fmt"
	"time"
)

// 聚合轮次状态机。
const (
	RoundStatePreparing  = "preparing"  // 准备：轮次已登记，尚未开放接收更新。
	RoundStateReceiving  = "receiving"  // 接收中：客户端可并行上报更新。
	RoundStateValidating = "validating" // 待校验：已停止接收，等待谱系校验。
	RoundStateAggregable = "aggregable" // 可聚合：更新集合已确认，可发布。
	RoundStateSealed     = "sealed"     // 封存：轮次冻结，不接受任何修改。
)

// 客户端更新状态机。
const (
	UpdateStateNew      = "new"      // 新到：刚写入，未校验。
	UpdateStateValid    = "valid"    // 有效：父关系与形状通过。
	UpdateStateReplay   = "replay"   // 重放：与已存在更新 ID 冲突（重放窗口内）。
	UpdateStateForked   = "forked"   // 分叉：父轮次与已确认谱系不一致。
	UpdateStateIsolated = "isolated" // 隔离：被研究员主动隔离，不参与聚合。
)

// 模型节点状态机。
const (
	NodeStateCandidate  = "candidate"  // 候选：摘要已登记，未确认。
	NodeStateConfirmed  = "confirmed"  // 确认：被某轮次采纳为父模型。
	NodeStateStale      = "stale"      // 过期：被后继轮次取代。
	NodeStateConflicted = "conflicted" // 冲突：同一父位置出现多个节点。
)

// 轮次快照状态机。
const (
	SnapshotStateDraft     = "draft"     // 草稿：未发布。
	SnapshotStatePublish   = "publish"   // 发布：对外可见的不可变快照。
	SnapshotStateSupersede = "supersede" // 替代：被后续快照取代。
)

// 常见错误。
var (
	ErrNotFound         = errors.New("entity not found")
	ErrParamMissing     = errors.New("parameter digest missing")
	ErrParentFuture     = errors.New("parent round is in the future")
	ErrUpdateConflict   = errors.New("update id conflict")
	ErrSealedMutation   = errors.New("sealed round cannot be modified")
	ErrRoundClosed      = errors.New("round closed for receiving")
	ErrInvalidState     = errors.New("invalid state transition")
	ErrCycle            = errors.New("lineage cycle detected")
	ErrUnknownNode      = errors.New("unknown model node referenced")
	ErrDigestMismatch   = errors.New("parameter digest shape mismatch")
	ErrDuplicateID      = errors.New("duplicate identifier")
	ErrInvalidDimension = errors.New("dimension must be positive")
	ErrParentMissing    = errors.New("parent entity not found")
	ErrSnapshotConflict = errors.New("published snapshot already exists")
	ErrStateCorrupt     = errors.New("persisted state is inconsistent")
)

// GlobalModel 表示一个全局模型节点（某一轮聚合产出的参数摘要）。
type GlobalModel struct {
	ID          string // 模型节点 ID，全局唯一。
	ParentID    string // 父模型节点 ID，空串表示根模型。
	ParamDigest string // 参数摘要（形状 + 维度指纹），用于一致性校验。
	Dimension   int    // 参数张量总维度，用于形状校验。
	RoundID     string // 产生该节点的聚合轮次 ID。
	State       string // 模型节点状态。
	CreatedAt   time.Time
}

// AggregateRound 表示一个联邦聚合轮次。
type AggregateRound struct {
	ID          string
	ParentRound string    // 父轮次 ID（谱系父边），空串表示首轮。
	State       string    // 轮次状态。
	ExpectedDim int       // 期望参数维度（不一致更新会被隔离）。
	SealedAt    time.Time // 封存时间，零值表示未封存。
	CreatedAt   time.Time
	ClosedAt    time.Time // 停止接收的时间，零值表示仍开放。
}

// ClientUpdate 表示客户端上报的一次模型更新。
type ClientUpdate struct {
	ID          string // 更新 ID（幂等键）。
	RoundID     string // 所属轮次。
	ClientID    string // 上报客户端标识。
	ParentModel string // 客户端声明的父模型节点 ID。
	ParamDigest string // 参数摘要。
	Dimension   int    // 参数维度。
	State       string // 更新状态。
	Reason      string // 隔离/异常原因。
	CreatedAt   time.Time
}

// RoundSnapshot 表示轮次发布的不可变谱系快照。
type RoundSnapshot struct {
	ID        string
	RoundID   string
	State     string
	Summary   string // 快照内容摘要（JSON 文本）。
	CreatedAt time.Time
}

// LineageEdge 表示模型节点之间的父子边（谱系图）。
type LineageEdge struct {
	Child  string
	Parent string
}

// UpdateVerification 是谱系校验的结果。
type UpdateVerification struct {
	UpdateID   string
	RoundID    string
	Verdict    string // valid | replay | forked | isolated
	Reason     string
	VerifiedAt time.Time
}

// StateCheck 校验状态是否在给定集合内。
func stateIn(state string, allowed ...string) bool {
	for _, a := range allowed {
		if state == a {
			return true
		}
	}
	return false
}

// ValidateDimension rejects dimensions that cannot describe a model tensor.
// Keeping this rule in the domain package prevents the HTTP and smoke-test
// entry points from accepting different notions of a valid model shape.
func ValidateDimension(dimension int) error {
	if dimension <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidDimension, dimension)
	}
	return nil
}

// ValidateModelInput checks the identity fields that are required before a
// node can participate in the lineage graph.
func ValidateModelInput(id, roundID, paramDigest string, dimension int) error {
	if id == "" {
		return fmt.Errorf("%w: model id empty", ErrDuplicateID)
	}
	if roundID == "" {
		return fmt.Errorf("%w: model round id empty", ErrParamMissing)
	}
	if paramDigest == "" {
		return ErrParamMissing
	}
	return ValidateDimension(dimension)
}

// ValidateUpdateInput checks fields that are meaningful at ingestion time.
// A dimension mismatch is intentionally not rejected here: it is a business
// verdict (forked) produced by lineage verification after the round closes.
func ValidateUpdateInput(u *ClientUpdate) error {
	if u == nil {
		return fmt.Errorf("%w: update is nil", ErrParamMissing)
	}
	if u.ID == "" {
		return fmt.Errorf("%w: update id empty", ErrDuplicateID)
	}
	if u.RoundID == "" || u.ClientID == "" {
		return fmt.Errorf("%w: update round and client are required", ErrParamMissing)
	}
	if u.ParamDigest == "" {
		return ErrParamMissing
	}
	return ValidateDimension(u.Dimension)
}

// ValidateRoundTransition 校验聚合轮次状态迁移合法性。
func ValidateRoundTransition(from, to string) error {
	switch from {
	case RoundStatePreparing:
		if !stateIn(to, RoundStateReceiving, RoundStateValidating) {
			return fmt.Errorf("%w: %s->%s", ErrInvalidState, from, to)
		}
	case RoundStateReceiving:
		if !stateIn(to, RoundStateValidating) {
			return fmt.Errorf("%w: %s->%s", ErrInvalidState, from, to)
		}
	case RoundStateValidating:
		if !stateIn(to, RoundStateAggregable, RoundStateReceiving) {
			return fmt.Errorf("%w: %s->%s", ErrInvalidState, from, to)
		}
	case RoundStateAggregable:
		if !stateIn(to, RoundStateSealed, RoundStateValidating) {
			return fmt.Errorf("%w: %s->%s", ErrInvalidState, from, to)
		}
	case RoundStateSealed:
		return fmt.Errorf("%w: %s is terminal", ErrInvalidState, from)
	default:
		return fmt.Errorf("%w: unknown state %s", ErrInvalidState, from)
	}
	return nil
}

// ValidateUpdateTransition 校验客户端更新状态迁移合法性。
func ValidateUpdateTransition(from, to string) error {
	switch from {
	case UpdateStateNew:
		if !stateIn(to, UpdateStateValid, UpdateStateReplay, UpdateStateForked, UpdateStateIsolated) {
			return fmt.Errorf("%w: %s->%s", ErrInvalidState, from, to)
		}
	case UpdateStateValid, UpdateStateReplay, UpdateStateForked:
		if to != UpdateStateIsolated {
			return fmt.Errorf("%w: %s->%s only isolated allowed", ErrInvalidState, from, to)
		}
	case UpdateStateIsolated:
		// 隔离态为终态，仅可在聚合确认时整体重算（不在此处迁移）。
		return fmt.Errorf("%w: %s is terminal for direct mutation", ErrInvalidState, from)
	default:
		return fmt.Errorf("%w: unknown state %s", ErrInvalidState, from)
	}
	return nil
}
