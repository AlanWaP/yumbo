package main

import (
	"encoding/json"
	"testing"
)

func TestChaosOfTheBabyCityPowerLosesToPrick(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", gamesMovePower, "")
	submitTestMove(t, session, "player_two", gamesMovePrick, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive {
		t.Fatal("expected player_one to be eliminated after powering up while attacked")
	}
}

func TestChaosOfTheBabyCityMutualPricksOffset(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 1
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", gamesMovePrick, "player_two")
	submitTestMove(t, session, "player_two", gamesMovePrick, "player_one")
	session.resolveRound()

	if !session.Players["player_one"].Alive || !session.Players["player_two"].Alive {
		t.Fatal("expected mutual pricks to offset")
	}
}

func TestChaosOfTheBabyCityDefenseBlocksPrickOnly(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_one"].Power = 2
	session.Players["player_three"].Power = 3

	submitTestMove(t, session, "player_one", gamesMoveClangClang, "player_two")
	submitTestMove(t, session, "player_two", gamesMoveDefense, "")
	submitTestMove(t, session, "player_three", gamesMoveVisaRay, "player_two")
	session.resolveRound()

	if session.Players["player_two"].Alive {
		t.Fatal("expected defense not to block clang clang or visa ray")
	}
}

func TestChaosOfTheBabyCityCoverEarBlocksClangClang(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 2

	submitTestMove(t, session, "player_one", gamesMoveClangClang, "player_two")
	submitTestMove(t, session, "player_two", gamesMoveCoverEar, "")
	session.resolveRound()

	if !session.Players["player_two"].Alive {
		t.Fatal("expected cover ear to block clang clang")
	}
}

func TestChaosOfTheBabyCityAbsorbGainsEnergyAndBlocksPrick(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 1

	submitTestMove(t, session, "player_one", gamesMovePrick, "player_two")
	submitTestMove(t, session, "player_two", gamesMoveAbsorb, "")
	session.resolveRound()

	if !session.Players["player_two"].Alive {
		t.Fatal("expected absorb to block prick")
	}
	if session.Players["player_two"].Power != 1 {
		t.Fatalf("expected absorb to gain 1 energy, got %d", session.Players["player_two"].Power)
	}
}

func TestChaosOfTheBabyCityAbsorbLosesToDetonation(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_one"].Power = 4

	submitTestMove(t, session, "player_one", gamesMoveDetonation, "")
	submitTestMove(t, session, "player_two", gamesMoveAbsorb, "")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	if session.Players["player_two"].Alive {
		t.Fatal("expected absorb not to block detonation")
	}
	if session.Players["player_three"].Alive {
		t.Fatal("expected detonation to hit non-absorb players")
	}
}

func TestChaosOfTheBabyCityKnifeEliminatesAbsorb(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)

	submitTestMove(t, session, "player_one", gamesMoveKnife, "")
	submitTestMove(t, session, "player_two", gamesMoveAbsorb, "")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	if session.Players["player_two"].Alive {
		t.Fatal("expected knife to eliminate absorb user")
	}
	if !session.Players["player_one"].Alive {
		t.Fatal("expected knife user to survive when only absorb attacks back")
	}
}

func TestChaosOfTheBabyCityKnifeUserLosesToPrick(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", gamesMoveKnife, "")
	submitTestMove(t, session, "player_two", gamesMovePrick, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive {
		t.Fatal("expected knife user to lose to prick")
	}
}

func TestChaosOfTheBabyCitySealNullifiesMoveAndBansIt(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_two"].Power = 1

	submitTestMove(t, session, "player_one", gamesMoveSeal, "player_two")
	submitTestMove(t, session, "player_two", gamesMovePrick, "player_three")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	if !session.Players["player_two"].Alive {
		t.Fatal("expected sealed player to be invulnerable this round")
	}
	if !containsString(session.Players["player_two"].BannedMoves, gamesMovePrick) {
		t.Fatal("expected prick to be banned after seal")
	}

	session.startNextRound()
	payload, err := json.Marshal(gameMovePayload{MoveType: gamesMovePrick, TargetID: "player_three"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.submitMove("player_two", payload); err == nil {
		t.Fatal("expected banned prick to be rejected")
	}
}

func TestChaosOfTheBabyCitySealDoesNotBanDetonation(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_two"].Power = 4

	submitTestMove(t, session, "player_one", gamesMoveSeal, "player_two")
	submitTestMove(t, session, "player_two", gamesMoveDetonation, "")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	if !session.Players["player_three"].Alive {
		t.Fatal("expected sealed detonation to be cancelled without hurting anyone")
	}
	if !session.Players["player_two"].Alive {
		t.Fatal("expected sealed detonation user to survive")
	}
	if containsString(session.Players["player_two"].BannedMoves, gamesMoveDetonation) {
		t.Fatal("expected detonation not to be permanently banned")
	}

	session.Players["player_two"].Power = 4
	session.startNextRound()
	submitTestMove(t, session, "player_one", gamesMovePower, "")
	submitTestMove(t, session, "player_two", gamesMoveDetonation, "")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	if session.Players["player_three"].Alive {
		t.Fatal("expected detonation to work again on a later round")
	}
}

func TestChaosOfTheBabyCityPowerIgnoresSeal(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)
	session.Players["player_three"].Power = 1

	submitTestMove(t, session, "player_one", gamesMoveSeal, "player_two")
	submitTestMove(t, session, "player_two", gamesMovePower, "")
	submitTestMove(t, session, "player_three", gamesMovePrick, "player_two")
	session.resolveRound()

	if session.Players["player_two"].Alive {
		t.Fatal("expected power user to ignore seal and still be eliminated by prick")
	}
}

func TestChaosOfTheBabyCityRejectsSecondSeal(t *testing.T) {
	session := newGameSession("room_one", gameTypeChaosOfTheBabyCity, []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)

	submitTestMove(t, session, "player_one", gamesMoveSeal, "player_two")
	submitTestMove(t, session, "player_two", gamesMovePower, "")
	submitTestMove(t, session, "player_three", gamesMovePower, "")
	session.resolveRound()

	payload, err := json.Marshal(gameMovePayload{MoveType: gamesMoveSeal, TargetID: "player_three"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.submitMove("player_one", payload); err == nil {
		t.Fatal("expected second seal to be rejected")
	}
}

const (
	gamesMovePower      = "power"
	gamesMoveDefense    = "defense"
	gamesMoveCoverEar   = "cover_ear"
	gamesMoveAbsorb     = "absorb"
	gamesMoveKnife      = "knife"
	gamesMoveSeal       = "seal"
	gamesMovePrick      = "prick"
	gamesMoveClangClang = "clang_clang"
	gamesMoveVisaRay    = "visa_ray"
	gamesMoveDetonation = "detonation"
)

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
