package combat

import "fmt"

type Combatant struct {
	ID     string
	TeamID string
	Energy int
	Health int
	Alive  bool
	Usage  UsageTracker
}

type UsageTracker struct {
	Consecutive map[string]int
}

func NewUsageTracker() UsageTracker {
	return UsageTracker{Consecutive: map[string]int{}}
}

func (u *UsageTracker) CanUse(moveID string, limit *UsageLimit) error {
	if limit == nil {
		return nil
	}
	if limit.MaxConsecutive > 0 && u.Consecutive[moveID] >= limit.MaxConsecutive {
		if moveID == "defense" {
			return fmt.Errorf("players cannot use defense three rounds in a row")
		}
		return fmt.Errorf("move %q cannot be used again yet", moveID)
	}
	return nil
}

func (u *UsageTracker) RecordUse(moveID string, spec MoveSpec, _ map[string]MoveSpec) {
	for _, resetID := range spec.ResetsStreak {
		u.Consecutive[resetID] = 0
	}

	if spec.Category == MoveDefend {
		u.Consecutive[moveID]++
		return
	}

	u.Consecutive[moveID] = 0
}
