package round

// Stats 返回某轮次的更新状态统计。
type Stats struct {
	RoundID string         `json:"round_id"`
	Total   int            `json:"total"`
	ByState map[string]int `json:"by_state"`
}

// Stats 统计轮次内各状态更新数。
func (s *Service) Stats(roundID string) (*Stats, error) {
	if _, err := s.store.GetRound(roundID); err != nil {
		return nil, err
	}
	us, err := s.store.ListUpdatesByRound(roundID)
	if err != nil {
		return nil, err
	}
	st := &Stats{RoundID: roundID, ByState: map[string]int{}}
	for _, u := range us {
		st.Total++
		st.ByState[u.State]++
	}
	return st, nil
}
