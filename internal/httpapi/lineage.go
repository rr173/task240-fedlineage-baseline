package httpapi

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleVerifyUpdate(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.sv.Lineage.Verify(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleVerifyRound(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	vs, err := s.sv.Lineage.VerifyRound(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"verifications": vs})
}

func (s *Server) handleForkedUpdates(w http.ResponseWriter, r *http.Request) {
	roundID := r.URL.Query().Get("round_id")
	if roundID == "" {
		writeErr(w, http.StatusBadRequest, errBadRound)
		return
	}
	us, err := s.sv.Lineage.ForkedUpdates(roundID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"forks": us})
}

func (s *Server) handleAncestors(w http.ResponseWriter, r *http.Request) {
	var req parentChildReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	path, err := s.sv.Lineage.AncestorPath(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"model_id": req.ID, "ancestors": path})
}

func (s *Server) handleComputeAggregate(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	set, err := s.sv.Aggregate.Compute(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *Server) handleConfirmAggregate(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	set, err := s.sv.Aggregate.Confirm(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *Server) handleAuditAggregate(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	a, err := s.sv.Aggregate.Audit(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
