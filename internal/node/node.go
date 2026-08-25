// Package node 维护全局模型节点（谱系图中的节点）及其状态。
// 负责节点登记、确认与过期判定，并维护节点之间的父子谱系边。
package node

import (
	"fmt"
	"time"

	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/store"
)

// Service 模型节点服务。
type Service struct {
	store *store.Store
	now   func() time.Time
}

// New 构造模型节点服务。
func New(s *store.Store) *Service {
	return &Service{store: s, now: time.Now}
}

// Register 登记一个模型节点（候选态），可选父节点。
func (s *Service) Register(id, parentID, roundID, paramDigest string, dimension int) (*model.GlobalModel, error) {
	if err := model.ValidateModelInput(id, roundID, paramDigest, dimension); err != nil {
		return nil, err
	}
	if _, err := s.store.GetModel(id); err == nil {
		return nil, fmt.Errorf("%w: model %s", model.ErrDuplicateID, id)
	}
	if parentID != "" {
		_, err := s.store.GetModel(parentID)
		if err != nil {
			if err == model.ErrNotFound {
				return nil, fmt.Errorf("%w: model %s", model.ErrParentMissing, parentID)
			}
			return nil, err
		}
		cycle, err := s.DetectCycle(id, parentID)
		if err != nil {
			return nil, err
		}
		if cycle {
			return nil, model.ErrCycle
		}
	}
	m := &model.GlobalModel{
		ID:          id,
		ParentID:    parentID,
		ParamDigest: paramDigest,
		Dimension:   dimension,
		RoundID:     roundID,
		State:       model.NodeStateCandidate,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.store.PutModel(m); err != nil {
		return nil, err
	}
	if parentID != "" {
		if err := s.store.PutEdge(model.LineageEdge{Child: id, Parent: parentID}); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Confirm 确认节点（被某轮次采纳为父模型）。
func (s *Service) Confirm(id string) (*model.GlobalModel, error) {
	m, err := s.store.GetModel(id)
	if err != nil {
		return nil, err
	}
	m.State = model.NodeStateConfirmed
	if err := s.store.PutModel(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Stale 将节点标记为过期（被后继轮次取代）。
func (s *Service) Stale(id string) (*model.GlobalModel, error) {
	m, err := s.store.GetModel(id)
	if err != nil {
		return nil, err
	}
	m.State = model.NodeStateStale
	if err := s.store.PutModel(m); err != nil {
		return nil, err
	}
	return m, nil
}

// FlagConflict 标记节点冲突（同一父位置多个候选）。
func (s *Service) FlagConflict(id string) (*model.GlobalModel, error) {
	m, err := s.store.GetModel(id)
	if err != nil {
		return nil, err
	}
	m.State = model.NodeStateConflicted
	if err := s.store.PutModel(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Get 读取节点。
func (s *Service) Get(id string) (*model.GlobalModel, error) {
	return s.store.GetModel(id)
}

// List 列出全部节点。
func (s *Service) List() ([]*model.GlobalModel, error) {
	return s.store.ListModels()
}

// Parents 返回节点的父节点 ID。
func (s *Service) Parents(id string) ([]string, error) {
	return s.store.ParentsOf(id)
}

// Children 返回节点的子节点 ID。
func (s *Service) Children(id string) ([]string, error) {
	return s.store.ChildrenOf(id)
}

// DetectCycle 检测从给定祖先出发是否会形成环（即 ancestor 已为 id 的后代）。
func (s *Service) DetectCycle(child, parent string) (bool, error) {
	// 从 parent 向上遍历其祖先，若遇到 child 则成环。
	visited := map[string]bool{}
	stack := []string{parent}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == child {
			return true, nil
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		parents, err := s.store.ParentsOf(cur)
		if err != nil {
			return false, err
		}
		stack = append(stack, parents...)
	}
	return false, nil
}
