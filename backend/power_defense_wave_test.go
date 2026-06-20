package main

import (
	"encoding/json"
	"testing"
)

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
	if session.Players["player_three"].Alive {
		t.Fatal("expected super blast to still attack other players")
	}
}
