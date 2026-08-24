// Package service 是编排层，组合 round/update/node/lineage/aggregate/snapshot
// 各业务包，对外提供端到端能力，并承载自检（self-check）逻辑。
package service

import (
	"task240-fedlineage/internal/aggregate"
	"task240-fedlineage/internal/lineage"
	"task240-fedlineage/internal/node"
	"task240-fedlineage/internal/round"
	"task240-fedlineage/internal/snapshot"
	"task240-fedlineage/internal/store"
	"task240-fedlineage/internal/update"
)

// Services 聚合所有业务包服务，供 HTTP 层与命令行复用。
type Services struct {
	Store    *store.Store
	Round    *round.Service
	Update   *update.Service
	Node     *node.Service
	Lineage  *lineage.Service
	Aggregate *aggregate.Service
	Snapshot *snapshot.Service
}

// New 构造全部业务服务（依赖顺序固定）。
func New(s *store.Store) *Services {
	rs := round.New(s)
	us := update.New(s, rs)
	ns := node.New(s)
	ls := lineage.New(s, ns, rs, us)
	as := aggregate.New(s, rs)
	ss := snapshot.New(s, rs, as)
	return &Services{Store: s, Round: rs, Update: us, Node: ns, Lineage: ls, Aggregate: as, Snapshot: ss}
}

// SelfCheck 执行自检：返回各子系统是否可达、实体计数，用于 /api/selfcheck。
type SelfCheck struct {
	OK           bool             `json:"ok"`
	Models       int              `json:"models"`
	Rounds       int              `json:"rounds"`
	Updates      int              `json:"updates"`
	Snapshots    int              `json:"snapshots"`
	Details      map[string]int   `json:"details"`
}

// SelfCheck 运行一次一致性自检。
func (sv *Services) SelfCheck() (*SelfCheck, error) {
	models, err := sv.Store.ListModels()
	if err != nil {
		return nil, err
	}
	rounds, err := sv.Store.ListRounds()
	if err != nil {
		return nil, err
	}
	details := map[string]int{"models": len(models), "rounds": len(rounds)}
	sc := &SelfCheck{
		OK:        true,
		Models:    len(models),
		Rounds:    len(rounds),
		Details:   details,
	}
	return sc, nil
}
