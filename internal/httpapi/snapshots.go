package httpapi

import (
	"errors"
	"net/http"
)

var errBadRound = errors.New("round_id required")

func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string `json:"id"`
		RoundID string `json:"round_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	snap, err := s.sv.Snapshot.Publish(req.ID, req.RoundID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	roundID := r.URL.Query().Get("round_id")
	if roundID == "" {
		writeErr(w, http.StatusBadRequest, errBadRound)
		return
	}
	snaps, err := s.sv.Snapshot.ListByRound(roundID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"snapshots": snaps})
}

func (s *Server) handlePublishedSnapshot(w http.ResponseWriter, r *http.Request) {
	roundID := r.URL.Query().Get("round_id")
	if roundID == "" {
		writeErr(w, http.StatusBadRequest, errBadRound)
		return
	}
	snap, err := s.sv.Snapshot.Published(roundID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	snap, err := s.sv.Snapshot.Supersede(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	sc, err := s.sv.SelfCheck()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sc)
}
