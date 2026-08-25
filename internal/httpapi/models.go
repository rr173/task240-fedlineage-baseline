package httpapi

import (
	"encoding/json"
	"net/http"

	"task240-fedlineage/internal/model"
)

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.sv.Node.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": models})
}

type registerModelReq struct {
	ID          string `json:"id"`
	ParentID    string `json:"parent_id"`
	RoundID     string `json:"round_id"`
	ParamDigest string `json:"param_digest"`
	Dimension   int    `json:"dimension"`
}

func (s *Server) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var req registerModelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// 形状一致性：若已有父节点，检测是否成环。
	if req.ParentID != "" {
		cycle, err := s.sv.Node.DetectCycle(req.ID, req.ParentID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if cycle {
			writeErr(w, http.StatusBadRequest, model.ErrCycle)
			return
		}
	}
	m, err := s.sv.Node.Register(req.ID, req.ParentID, req.RoundID, req.ParamDigest, req.Dimension)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

type confirmModelReq struct {
	ID     string `json:"id"`
	Action string `json:"action"` // confirm | stale | conflict
}

func (s *Server) handleConfirmModel(w http.ResponseWriter, r *http.Request) {
	var req confirmModelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var m *model.GlobalModel
	var err error
	switch req.Action {
	case "confirm", "":
		m, err = s.sv.Node.Confirm(req.ID)
	case "stale":
		m, err = s.sv.Node.Stale(req.ID)
	case "conflict":
		m, err = s.sv.Node.FlagConflict(req.ID)
	default:
		writeErr(w, http.StatusBadRequest, model.ErrInvalidState)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleStaleModel(w http.ResponseWriter, r *http.Request) {
	var req confirmModelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m, err := s.sv.Node.Stale(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleConflictModel(w http.ResponseWriter, r *http.Request) {
	var req confirmModelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m, err := s.sv.Node.FlagConflict(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type parentChildReq struct {
	ID string `json:"id"`
}

func (s *Server) handleModelParents(w http.ResponseWriter, r *http.Request) {
	var req parentChildReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	parents, err := s.sv.Node.Parents(req.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"parents": parents})
}

func (s *Server) handleModelChildren(w http.ResponseWriter, r *http.Request) {
	var req parentChildReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	children, err := s.sv.Node.Children(req.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"children": children})
}
