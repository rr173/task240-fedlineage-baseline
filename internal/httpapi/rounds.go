package httpapi

import (
	"encoding/json"
	"net/http"

	"task240-fedlineage/internal/model"
)

func (s *Server) handleListRounds(w http.ResponseWriter, r *http.Request) {
	rounds, err := s.sv.Round.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"rounds": rounds})
}

type registerRoundReq struct {
	ID          string `json:"id"`
	ParentRound string `json:"parent_round"`
	ExpectedDim int    `json:"expected_dim"`
}

func (s *Server) handleRegisterRound(w http.ResponseWriter, r *http.Request) {
	var req registerRoundReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	round, err := s.sv.Round.Register(req.ID, req.ParentRound, req.ExpectedDim)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, round)
}

type idReq struct {
	ID string `json:"id"`
}

func (s *Server) handleOpenRound(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	round, err := s.sv.Round.Open(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) handleCloseRound(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	round, err := s.sv.Round.Close(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) handleMarkAggregable(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	round, err := s.sv.Round.MarkAggregable(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) handleSealRound(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	round, err := s.sv.Round.Seal(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) handleRoundStats(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.sv.Round.Stats(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleReceiveUpdate(w http.ResponseWriter, r *http.Request) {
	var u model.ClientUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.sv.Update.Receive(&u)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	code := http.StatusCreated
	if res.State == model.UpdateStateReplay {
		code = http.StatusOK
	}
	writeJSON(w, code, res)
}

func (s *Server) handleListUpdates(w http.ResponseWriter, r *http.Request) {
	roundID := r.URL.Query().Get("round_id")
	if roundID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrNotFound)
		return
	}
	us, err := s.sv.Update.ListByRound(roundID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"updates": us})
}

type isolateReq struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

func (s *Server) handleIsolateUpdate(w http.ResponseWriter, r *http.Request) {
	var req isolateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	u, err := s.sv.Update.Isolate(req.ID, req.Reason)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
