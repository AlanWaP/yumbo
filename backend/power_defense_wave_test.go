package main

import (
	"encoding/json"
	"testing"
)

func TestPowerDefenseWaveSimultaneousMoveRules(t *testing.T) {
	tests := []struct {
		name        string
		playerIDs   []string
		power       map[string]int
		moves       []submittedMove
		wantAlive   map[string]bool
		description string
	}{
		{
			name:      "power loses to any incoming attack",
			playerIDs: []string{"player_one", "player_two"},
			power:     map[string]int{"player_two": 1},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypePower},
				{PlayerID: "player_two", Type: moveTypeWave, TargetID: "player_one"},
			},
			wantAlive: map[string]bool{
				"player_one": false,
				"player_two": true,
			},
			description: "choosing Power is unsafe when another move attacks you in the same round",
		},
		{
			name:      "mutual waves offset each other",
			playerIDs: []string{"player_one", "player_two"},
			power: map[string]int{
				"player_one": 1,
				"player_two": 1,
			},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeWave, TargetID: "player_two"},
				{PlayerID: "player_two", Type: moveTypeWave, TargetID: "player_one"},
			},
			wantAlive: map[string]bool{
				"player_one": true,
				"player_two": true,
			},
			description: "Wave only offsets an incoming Wave when both players target each other",
		},
		{
			name:      "wave aimed elsewhere does not offset incoming wave",
			playerIDs: []string{"player_one", "player_two", "player_three"},
			power: map[string]int{
				"player_one":   1,
				"player_two":   1,
				"player_three": 1,
			},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeWave, TargetID: "player_two"},
				{PlayerID: "player_two", Type: moveTypeWave, TargetID: "player_three"},
				{PlayerID: "player_three", Type: moveTypeWave, TargetID: "player_one"},
			},
			wantAlive: map[string]bool{
				"player_one":   false,
				"player_two":   false,
				"player_three": false,
			},
			description: "simultaneous Waves do not form a generic shield; target choice matters",
		},
		{
			name:      "defense blocks one super blast",
			playerIDs: []string{"player_one", "player_two"},
			power:     map[string]int{"player_two": 3},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeDefense},
				{PlayerID: "player_two", Type: moveTypeSuperBlast},
			},
			wantAlive: map[string]bool{
				"player_one": true,
				"player_two": true,
			},
			description: "Defense survives a single Super Blast in the same round",
		},
		{
			name:      "multiple super blasts break defense but do not eliminate each other",
			playerIDs: []string{"player_one", "player_two", "player_three"},
			power: map[string]int{
				"player_two":   3,
				"player_three": 3,
			},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeDefense},
				{PlayerID: "player_two", Type: moveTypeSuperBlast},
				{PlayerID: "player_three", Type: moveTypeSuperBlast},
			},
			wantAlive: map[string]bool{
				"player_one":   false,
				"player_two":   true,
				"player_three": true,
			},
			description: "Super Blast users are not killed by other Super Blasts",
		},
		{
			name:      "air cannon counters targeted super blast",
			playerIDs: []string{"player_one", "player_two", "player_three"},
			power:     map[string]int{"player_two": 3},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeAirCannon, TargetID: "player_two"},
				{PlayerID: "player_two", Type: moveTypeSuperBlast},
				{PlayerID: "player_three", Type: moveTypePower},
			},
			wantAlive: map[string]bool{
				"player_one":   true,
				"player_two":   false,
				"player_three": false,
			},
			description: "Air Cannon eliminates its Super Blast target and survives that target's blast",
		},
		{
			name:      "air cannon does not block unrelated incoming attacks",
			playerIDs: []string{"player_one", "player_two", "player_three"},
			power:     map[string]int{"player_three": 1},
			moves: []submittedMove{
				{PlayerID: "player_one", Type: moveTypeAirCannon, TargetID: "player_two"},
				{PlayerID: "player_two", Type: moveTypePower},
				{PlayerID: "player_three", Type: moveTypeWave, TargetID: "player_one"},
			},
			wantAlive: map[string]bool{
				"player_one":   false,
				"player_two":   true,
				"player_three": true,
			},
			description: "Air Cannon only protects against the targeted Super Blast, not other attacks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newGameSession("room_one", gameTypePowerDefenseWave, tt.playerIDs, gameModeFreeForAll, len(tt.playerIDs))
			for playerID, power := range tt.power {
				session.Players[playerID].Power = power
			}

			for _, move := range tt.moves {
				submitTestMove(t, session, move.PlayerID, move.Type, move.TargetID)
			}
			session.resolveRound()

			for playerID, wantAlive := range tt.wantAlive {
				if gotAlive := session.Players[playerID].Alive; gotAlive != wantAlive {
					t.Fatalf("%s: %s alive=%v, want %v", tt.description, playerID, gotAlive, wantAlive)
				}
			}
		})
	}
}

func TestPowerDefenseWavePowerLosesWhenAttacked(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", moveTypePower, "")
	submitTestMove(t, session, "player_two", moveTypeWave, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive {
		t.Fatal("expected player_one to be eliminated after powering up while attacked")
	}
	if !session.Players["player_two"].Alive {
		t.Fatal("expected attacking player to remain alive")
	}
}

func TestPowerDefenseWaveMutualWavesOffset(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 1
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", moveTypeWave, "player_two")
	submitTestMove(t, session, "player_two", moveTypeWave, "player_one")
	session.resolveRound()

	if !session.Players["player_one"].Alive || !session.Players["player_two"].Alive {
		t.Fatalf("expected mutual waves to offset, got player_one=%v player_two=%v", session.Players["player_one"].Alive, session.Players["player_two"].Alive)
	}
}

func TestPowerDefenseWaveOnlyTargetedWaveOffsetsIncomingWave(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_one"].Power = 1
	session.Players["player_two"].Power = 1
	session.Players["player_three"].Power = 1

	submitTestMove(t, session, "player_one", moveTypeWave, "player_two")
	submitTestMove(t, session, "player_two", moveTypeWave, "player_three")
	submitTestMove(t, session, "player_three", moveTypeWave, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive || session.Players["player_two"].Alive || session.Players["player_three"].Alive {
		t.Fatalf(
			"expected wave chain to eliminate all players, got player_one=%v player_two=%v player_three=%v",
			session.Players["player_one"].Alive,
			session.Players["player_two"].Alive,
			session.Players["player_three"].Alive,
		)
	}
}

func TestPowerDefenseWaveDefenseSurvivesIncomingWave(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_one"].Power = 1
	session.Players["player_three"].Power = 1

	submitTestMove(t, session, "player_one", moveTypeWave, "player_two")
	submitTestMove(t, session, "player_two", moveTypeDefense, "")
	submitTestMove(t, session, "player_three", moveTypeWave, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive {
		t.Fatal("expected player_one to be eliminated by player_three's wave")
	}
	if !session.Players["player_two"].Alive {
		t.Fatal("expected defense to survive incoming wave")
	}
	if !session.Players["player_three"].Alive {
		t.Fatal("expected player_three to survive")
	}
}

func TestPowerDefenseWaveRejectsThirdConsecutiveDefense(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)

	submitTestMove(t, session, "player_one", moveTypeDefense, "")
	submitTestMove(t, session, "player_two", moveTypeDefense, "")
	session.resolveRound()
	submitTestMove(t, session, "player_one", moveTypeDefense, "")
	submitTestMove(t, session, "player_two", moveTypeDefense, "")
	session.resolveRound()

	payload, err := json.Marshal(gameMovePayload{MoveType: moveTypeDefense})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.submitMove("player_one", payload); err == nil {
		t.Fatal("expected third consecutive defense to be rejected")
	}
}

func TestPowerDefenseWaveDefenseBlocksSingleSuperBlast(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_two"].Power = 3

	submitTestMove(t, session, "player_one", moveTypeDefense, "")
	submitTestMove(t, session, "player_two", moveTypeSuperBlast, "")
	submitTestMove(t, session, "player_three", moveTypePower, "")
	session.resolveRound()

	if !session.Players["player_one"].Alive {
		t.Fatal("expected defense to block one super blast")
	}
	if session.Players["player_three"].Alive {
		t.Fatal("expected non-defending player to lose to super blast")
	}
}

func TestPowerDefenseWaveMultipleSuperBlastsBreakDefense(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_two"].Power = 3
	session.Players["player_three"].Power = 3

	submitTestMove(t, session, "player_one", moveTypeDefense, "")
	submitTestMove(t, session, "player_two", moveTypeSuperBlast, "")
	submitTestMove(t, session, "player_three", moveTypeSuperBlast, "")
	session.resolveRound()

	if session.Players["player_one"].Alive {
		t.Fatal("expected multiple super blasts to break defense")
	}
}

func TestPowerDefenseWaveSuperBlastsDoNotEliminateEachOther(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 3
	session.Players["player_two"].Power = 3

	submitTestMove(t, session, "player_one", moveTypeSuperBlast, "")
	submitTestMove(t, session, "player_two", moveTypeSuperBlast, "")
	session.resolveRound()

	if !session.Players["player_one"].Alive || !session.Players["player_two"].Alive {
		t.Fatalf(
			"expected super blasts not to eliminate each other, got player_one=%v player_two=%v",
			session.Players["player_one"].Alive,
			session.Players["player_two"].Alive,
		)
	}
}

func TestPowerDefenseWaveAirCannonEliminatesSuperBlastButDoesNotCancelIt(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_two"].Power = 3

	submitTestMove(t, session, "player_one", moveTypeAirCannon, "player_two")
	submitTestMove(t, session, "player_two", moveTypeSuperBlast, "")
	submitTestMove(t, session, "player_three", moveTypePower, "")
	session.resolveRound()

	if session.Players["player_two"].Alive {
		t.Fatal("expected air cannon target using super blast to be eliminated")
	}
	if !session.Players["player_one"].Alive {
		t.Fatal("expected air cannon user to survive the targeted super blast")
	}
	if session.Players["player_three"].Alive {
		t.Fatal("expected super blast to still attack other players")
	}
}
