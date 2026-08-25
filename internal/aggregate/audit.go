package aggregate

import (
	"fmt"
	"strings"
)

// Audit 对可聚合集合做审计：汇总被排除更新的原因分布，给出可读结论。
type Audit struct {
	RoundID       string         `json:"round_id"`
	Included      int            `json:"included"`
	Excluded      int            `json:"excluded"`
	ReasonBuckets map[string]int `json:"reason_buckets"`
	Verdict       string         `json:"verdict"`
}

// Audit 审计一个可聚合集合的排除原因。
func (s *Service) Audit(roundID string) (*Audit, error) {
	set, err := s.Compute(roundID)
	if err != nil {
		return nil, err
	}
	a := &Audit{RoundID: roundID, Included: set.UpdateCount, Excluded: len(set.Excluded), ReasonBuckets: map[string]int{}}
	for _, e := range set.Excluded {
		key := fmt.Sprintf("%s:%s", e.State, shortReason(e.Reason))
		a.ReasonBuckets[key]++
	}
	if len(set.Excluded) == 0 {
		a.Verdict = "clean"
	} else {
		a.Verdict = "needs_review"
	}
	return a, nil
}

func shortReason(reason string) string {
	if i := strings.Index(reason, ":"); i >= 0 {
		return strings.TrimSpace(reason[i+1:])
	}
	return reason
}
