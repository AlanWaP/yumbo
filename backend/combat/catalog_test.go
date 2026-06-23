package combat_test

import (
	"testing"

	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

func TestBuildMoveCatalogMarksSingleEnemyMovesAsTargeted(t *testing.T) {
	catalog := combat.BuildMoveCatalog(games.PowerDefenseWave)

	targeted := map[string]bool{}
	for _, entry := range catalog {
		targeted[entry.ID] = entry.RequiresTargetPlayer
	}

	if !targeted[games.MoveWave] || !targeted[games.MoveAirCannon] {
		t.Fatalf("expected wave and air cannon to require a target player, got %v", targeted)
	}
	if targeted[games.MovePower] || targeted[games.MoveDefense] || targeted[games.MoveSuperBlast] {
		t.Fatalf("expected non-single-enemy moves not to require a target player, got %v", targeted)
	}
}

func TestBuildMoveCatalogUsesDisplayOrder(t *testing.T) {
	catalog := combat.BuildMoveCatalog(games.PowerDefenseWave)

	if len(catalog) < 5 {
		t.Fatalf("expected five moves in catalog, got %d", len(catalog))
	}
	if catalog[0].ID != games.MovePower || catalog[2].ID != games.MoveAirCannon {
		t.Fatalf("unexpected move order: %#v", catalog)
	}
}
