package combat_test

import (
	"testing"

	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

func TestPowerDefenseWaveCounterOnlyAirCannonDoesNotHitPowerTarget(t *testing.T) {
	input := roundInput(
		[]string{"attacker", "target", "waver"},
		map[string]int{"waver": 1},
		map[string]combat.SubmittedMove{
			"attacker": {PlayerID: "attacker", MoveID: games.MoveAirCannon, TargetID: "target"},
			"target":   {PlayerID: "target", MoveID: games.MovePower},
			"waver":    {PlayerID: "waver", MoveID: games.MoveWave, TargetID: "attacker"},
		},
	)

	output := games.PowerDefenseWave.ResolveRound(input)

	if !input.Combatants["target"].Alive {
		t.Fatal("air cannon should not eliminate a non-super-blast target")
	}
	if input.Combatants["attacker"].Alive {
		t.Fatal("air cannon user should lose to unrelated wave attacks")
	}
	if !input.Combatants["waver"].Alive {
		t.Fatal("wave attacker should survive")
	}
	if len(output.Eliminated) != 1 || output.Eliminated["attacker"] == "" {
		t.Fatalf("expected only attacker eliminated, got %v", output.Eliminated)
	}
}

func TestPowerDefenseWaveAirCannonCountersSuperBlastWithoutLevelMath(t *testing.T) {
	input := roundInput(
		[]string{"cannon", "blaster", "bystander"},
		map[string]int{"blaster": 3},
		map[string]combat.SubmittedMove{
			"cannon":    {PlayerID: "cannon", MoveID: games.MoveAirCannon, TargetID: "blaster"},
			"blaster":   {PlayerID: "blaster", MoveID: games.MoveSuperBlast},
			"bystander": {PlayerID: "bystander", MoveID: games.MovePower},
		},
	)

	output := games.PowerDefenseWave.ResolveRound(input)

	if input.Combatants["blaster"].Alive {
		t.Fatal("air cannon should eliminate targeted super blast user")
	}
	if !input.Combatants["cannon"].Alive {
		t.Fatal("air cannon user should survive the targeted super blast")
	}
	if input.Combatants["bystander"].Alive {
		t.Fatal("super blast should still hit other players")
	}
	if output.Eliminated["blaster"] == "" {
		t.Fatal("expected counter elimination reason for blaster")
	}
}

func TestPowerDefenseWaveMutualWavesOffset(t *testing.T) {
	input := roundInput(
		[]string{"player_one", "player_two"},
		map[string]int{"player_one": 1, "player_two": 1},
		map[string]combat.SubmittedMove{
			"player_one": {PlayerID: "player_one", MoveID: games.MoveWave, TargetID: "player_two"},
			"player_two": {PlayerID: "player_two", MoveID: games.MoveWave, TargetID: "player_one"},
		},
	)

	games.PowerDefenseWave.ResolveRound(input)

	if !input.Combatants["player_one"].Alive || !input.Combatants["player_two"].Alive {
		t.Fatal("mutual waves should offset through interaction rules, not attack levels")
	}
}

func TestUsageLimitBlocksThirdDefense(t *testing.T) {
	usage := combat.NewUsageTracker()
	limit := &combat.UsageLimit{MaxConsecutive: 2}

	usage.Consecutive[games.MoveDefense] = 0
	if err := usage.CanUse(games.MoveDefense, limit); err != nil {
		t.Fatal(err)
	}
	usage.RecordUse(games.MoveDefense, games.PowerDefenseWave.Moves[games.MoveDefense], games.PowerDefenseWave.Moves)

	usage.Consecutive[games.MoveDefense] = 1
	if err := usage.CanUse(games.MoveDefense, limit); err != nil {
		t.Fatal(err)
	}
	usage.RecordUse(games.MoveDefense, games.PowerDefenseWave.Moves[games.MoveDefense], games.PowerDefenseWave.Moves)

	usage.Consecutive[games.MoveDefense] = 2
	if err := usage.CanUse(games.MoveDefense, limit); err == nil {
		t.Fatal("expected third consecutive defense to be rejected")
	}
}

func roundInput(
	playerIDs []string,
	power map[string]int,
	moves map[string]combat.SubmittedMove,
) combat.RoundInput {
	combatants := map[string]*combat.Combatant{}
	aliveAtStart := map[string]bool{}

	for index, playerID := range playerIDs {
		teamID := playerID
		if len(playerIDs) > 2 {
			teamID = playerID
		}
		_ = index

		energy := power[playerID]
		combatants[playerID] = &combat.Combatant{
			ID:     playerID,
			TeamID: teamID,
			Energy: energy,
			Health: 1,
			Alive:  true,
			Usage:  combat.NewUsageTracker(),
		}
		aliveAtStart[playerID] = true
	}

	return combat.RoundInput{
		Combatants:   combatants,
		Moves:        moves,
		AliveAtStart: aliveAtStart,
	}
}
