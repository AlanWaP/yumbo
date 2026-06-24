package combat_test

import (
	"testing"

	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

func TestChaosOfTheBabyCityClangClangBeatsPrick(t *testing.T) {
	input := roundInput(
		[]string{"attacker", "target"},
		map[string]int{"attacker": 2},
		map[string]combat.SubmittedMove{
			"attacker": {PlayerID: "attacker", MoveID: games.MoveClangClang, TargetID: "target"},
			"target":   {PlayerID: "target", MoveID: games.MovePrick, TargetID: "attacker"},
		},
	)

	games.ChaosOfTheBabyCity.ResolveRound(input)

	if input.Combatants["target"].Alive {
		t.Fatal("expected prick user to lose to clang clang")
	}
	if !input.Combatants["attacker"].Alive {
		t.Fatal("expected clang clang user to survive mutual exchange")
	}
}

func TestChaosOfTheBabyCityVisaRayBeatsClangClang(t *testing.T) {
	input := roundInput(
		[]string{"attacker", "target"},
		map[string]int{"attacker": 3},
		map[string]combat.SubmittedMove{
			"attacker": {PlayerID: "attacker", MoveID: games.MoveVisaRay, TargetID: "target"},
			"target":   {PlayerID: "target", MoveID: games.MoveClangClang, TargetID: "attacker"},
		},
	)

	games.ChaosOfTheBabyCity.ResolveRound(input)

	if input.Combatants["target"].Alive {
		t.Fatal("expected clang clang user to lose to visa ray")
	}
}

func TestChaosOfTheBabyCityDetonationsDoNotEliminateEachOther(t *testing.T) {
	input := roundInput(
		[]string{"player_one", "player_two", "player_three"},
		map[string]int{"player_one": 4, "player_two": 4},
		map[string]combat.SubmittedMove{
			"player_one":   {PlayerID: "player_one", MoveID: games.MoveDetonation},
			"player_two":   {PlayerID: "player_two", MoveID: games.MoveDetonation},
			"player_three": {PlayerID: "player_three", MoveID: games.MovePower},
		},
	)

	games.ChaosOfTheBabyCity.ResolveRound(input)

	if !input.Combatants["player_one"].Alive || !input.Combatants["player_two"].Alive {
		t.Fatal("expected detonation users to survive each other")
	}
	if input.Combatants["player_three"].Alive {
		t.Fatal("expected detonation to eliminate other players")
	}
}
