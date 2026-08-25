// Package httpapi 暴露 HTTP 接口，路由前缀统一为 /api。
// 提供模型、轮次、更新写入、谱系校验、隔离、聚合确认、快照发布与自检 API。
package httpapi

import (
	"encoding/json"
	"net/http"

	"task240-fedlineage/internal/service"
)

// Server 持有业务服务与 mux。
type Server struct {
	sv  *service.Services
	mux *http.ServeMux
}

// New 构造 HTTP 服务并注册全部路由。
func New(sv *service.Services) *Server {
	s := &Server{sv: sv, mux: http.NewServeMux()}
	s.register()
	return s
}

// Handler 返回 http.Handler。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) register() {
	m := s.mux
	m.HandleFunc("/api/health", s.handleHealth)
	// 模型节点
	m.HandleFunc("/api/models", s.handleListModels)
	m.HandleFunc("/api/models/register", s.handleRegisterModel)
	m.HandleFunc("/api/models/confirm", s.handleConfirmModel)
	m.HandleFunc("/api/models/stale", s.handleStaleModel)
	m.HandleFunc("/api/models/conflict", s.handleConflictModel)
	m.HandleFunc("/api/models/parents", s.handleModelParents)
	m.HandleFunc("/api/models/children", s.handleModelChildren)
	// 轮次
	m.HandleFunc("/api/rounds", s.handleListRounds)
	m.HandleFunc("/api/rounds/register", s.handleRegisterRound)
	m.HandleFunc("/api/rounds/open", s.handleOpenRound)
	m.HandleFunc("/api/rounds/close", s.handleCloseRound)
	m.HandleFunc("/api/rounds/aggregable", s.handleMarkAggregable)
	m.HandleFunc("/api/rounds/seal", s.handleSealRound)
	m.HandleFunc("/api/rounds/stats", s.handleRoundStats)
	// 更新
	m.HandleFunc("/api/updates/receive", s.handleReceiveUpdate)
	m.HandleFunc("/api/updates", s.handleListUpdates)
	m.HandleFunc("/api/updates/isolate", s.handleIsolateUpdate)
	// 谱系校验
	m.HandleFunc("/api/lineage/verify", s.handleVerifyUpdate)
	m.HandleFunc("/api/lineage/verify-round", s.handleVerifyRound)
	m.HandleFunc("/api/lineage/forks", s.handleForkedUpdates)
	m.HandleFunc("/api/lineage/ancestors", s.handleAncestors)
	// 聚合
	m.HandleFunc("/api/aggregate/compute", s.handleComputeAggregate)
	m.HandleFunc("/api/aggregate/confirm", s.handleConfirmAggregate)
	m.HandleFunc("/api/aggregate/audit", s.handleAuditAggregate)
	// 快照
	m.HandleFunc("/api/snapshots/publish", s.handlePublishSnapshot)
	m.HandleFunc("/api/snapshots", s.handleListSnapshots)
	m.HandleFunc("/api/snapshots/published", s.handlePublishedSnapshot)
	m.HandleFunc("/api/snapshots/supersede", s.handleSupersedeSnapshot)
	// 自检
	m.HandleFunc("/api/selfcheck", s.handleSelfCheck)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
