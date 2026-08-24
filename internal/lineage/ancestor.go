package lineage

import (
	"task240-fedlineage/internal/model"
)

// AncestorPath 返回从给定模型节点到根节点的祖先路径（含自身）。
// 用于回答“该更新声称的父模型究竟源自哪个确认模型”。
func (s *Service) AncestorPath(child string) ([]string, error) {
	path := []string{}
	visited := map[string]bool{}
	cur := child
	for cur != "" {
		if visited[cur] {
			return nil, model.ErrCycle
		}
		visited[cur] = true
		path = append(path, cur)
		parents, err := s.store.ParentsOf(cur)
		if err != nil {
			return nil, err
		}
		if len(parents) == 0 {
			break
		}
		// 取第一个父节点继续向上（多父场景用于冲突检测，祖先路径取主链）。
		cur = parents[0]
	}
	return path, nil
}

// RootModel 返回某模型节点的根祖先（无父者）。
func (s *Service) RootModel(child string) (string, error) {
	path, err := s.AncestorPath(child)
	if err != nil {
		return "", err
	}
	return path[len(path)-1], nil
}
