package combat

type MoveCatalogEntry struct {
	ID                   string `json:"id"`
	Category             string `json:"category"`
	EnergyCost           int    `json:"energyCost"`
	EnergyGain           int    `json:"energyGain"`
	TargetScope          string `json:"targetScope"`
	RequiresTargetPlayer bool   `json:"requiresTargetPlayer"`
	MaxConsecutive       int    `json:"maxConsecutive,omitempty"`
	MaxUsesPerGame       int    `json:"maxUsesPerGame,omitempty"`
}

func BuildMoveCatalog(def GameDefinition) []MoveCatalogEntry {
	order := def.MoveOrder
	if len(order) == 0 {
		order = sortedMoveIDs(def.Moves)
	}

	entries := make([]MoveCatalogEntry, 0, len(order))
	for _, moveID := range order {
		spec, ok := def.Moves[moveID]
		if !ok {
			continue
		}
		entries = append(entries, MoveCatalogEntry{
			ID:                   spec.ID,
			Category:             string(spec.Category),
			EnergyCost:           spec.EnergyCost,
			EnergyGain:           spec.EnergyGain,
			TargetScope:          string(spec.TargetScope),
			RequiresTargetPlayer: spec.TargetScope == TargetSingleEnemy,
			MaxConsecutive:       maxConsecutive(spec.UsageLimit),
			MaxUsesPerGame:       maxUsesPerGame(spec.UsageLimit),
		})
	}

	return entries
}

func maxConsecutive(limit *UsageLimit) int {
	if limit == nil {
		return 0
	}
	return limit.MaxConsecutive
}

func maxUsesPerGame(limit *UsageLimit) int {
	if limit == nil {
		return 0
	}
	return limit.MaxUsesPerGame
}

func sortedMoveIDs(moves map[string]MoveSpec) []string {
	ids := make([]string, 0, len(moves))
	for moveID := range moves {
		ids = append(ids, moveID)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}
