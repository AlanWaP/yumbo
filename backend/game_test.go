package main

import (
	"encoding/json"
	"testing"
)

func submitTestMove(t *testing.T, session *gameSession, playerID string, moveType string, targetID string) bool {
	t.Helper()

	payload, err := json.Marshal(gameMovePayload{
		MoveType: moveType,
		TargetID: targetID,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, shouldResolve, err := session.submitMove(playerID, payload)
	if err != nil {
		t.Fatalf("submit move failed: %v", err)
	}
	return shouldResolve
}

func TestGameSessionResolvesAttackDefendAndGainPower(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two", "player_three"}, gameModeFreeForAll, 3)

	if shouldResolve := submitTestMove(t, session, "player_one", moveTypeAttack, "player_two"); shouldResolve {
		t.Fatal("expected round to wait for remaining moves")
	}
	submitTestMove(t, session, "player_two", moveTypeDefend, "")
	if shouldResolve := submitTestMove(t, session, "player_three", moveTypeGainPower, ""); !shouldResolve {
		t.Fatal("expected round to resolve after all alive players move")
	}
	session.resolveRound()

	if got := session.Players["player_one"].Power; got != 0 {
		t.Fatalf("expected attacker to spend power, got %d", got)
	}
	if got := session.Players["player_two"].Health; got != session.Rules.StartingHealth {
		t.Fatalf("expected defender to block damage, got health %d", got)
	}
	if got := session.Players["player_three"].Power; got != 2 {
		t.Fatalf("expected gain power move to add power, got %d", got)
	}
	if got := session.Round; got != 2 {
		t.Fatalf("expected next round, got %d", got)
	}
}

func TestGameSessionRejectsInvalidAttack(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Players["player_one"].Power = 0

	payload, err := json.Marshal(gameMovePayload{
		MoveType: moveTypeAttack,
		TargetID: "player_two",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := session.submitMove("player_one", payload); err == nil {
		t.Fatal("expected attack without power to fail")
	}
}

func TestGameSessionFinishesWhenOneTeamRemains(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Rules.AttackDamage = 10

	submitTestMove(t, session, "player_one", moveTypeAttack, "player_two")
	submitTestMove(t, session, "player_two", moveTypeGainPower, "")
	session.resolveRound()

	if session.Phase != gamePhaseFinished {
		t.Fatalf("expected game to finish, got phase %q", session.Phase)
	}
	if len(session.Winners) != 1 || session.Winners[0] != "player_one" {
		t.Fatalf("expected player_one to win, got %v", session.Winners)
	}
}

func TestGameSessionResolvesAttacksSimultaneously(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two"}, gameModeFreeForAll, 2)
	session.Rules.AttackDamage = 10

	submitTestMove(t, session, "player_one", moveTypeAttack, "player_two")
	submitTestMove(t, session, "player_two", moveTypeAttack, "player_one")
	session.resolveRound()

	if session.Players["player_one"].Alive || session.Players["player_two"].Alive {
		t.Fatalf("expected both players to be eliminated, got player_one=%v player_two=%v", session.Players["player_one"].Alive, session.Players["player_two"].Alive)
	}
	if session.Phase != gamePhaseFinished {
		t.Fatalf("expected game to finish, got phase %q", session.Phase)
	}
	if len(session.Winners) != 0 {
		t.Fatalf("expected no winners after mutual elimination, got %v", session.Winners)
	}
}

func TestTeamModeAssignsTeamsAndRejectsTeamAttacks(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two", "player_three", "player_four"}, gameModeTeam, 2)

	if session.Players["player_one"].TeamID != session.Players["player_three"].TeamID {
		t.Fatal("expected alternating players to share team one")
	}
	if session.Players["player_one"].TeamID == session.Players["player_two"].TeamID {
		t.Fatal("expected adjacent players to be on different teams")
	}

	payload, err := json.Marshal(gameMovePayload{
		MoveType: moveTypeAttack,
		TargetID: "player_three",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := session.submitMove("player_one", payload); err == nil {
		t.Fatal("expected attacking a teammate to fail")
	}
}

func TestGameSessionCancelMove(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two"}, gameModeFreeForAll, 2)

	submitTestMove(t, session, "player_one", moveTypeGainPower, "")
	if len(session.PendingMoves) != 1 {
		t.Fatalf("expected one pending move, got %d", len(session.PendingMoves))
	}

	receipt, err := session.cancelMove("player_one")
	if err != nil {
		t.Fatalf("cancel move failed: %v", err)
	}
	if len(session.PendingMoves) != 0 {
		t.Fatalf("expected pending moves to clear, got %d", len(session.PendingMoves))
	}
	if len(receipt.SubmittedPlayers) != 0 {
		t.Fatalf("expected no submitted players after cancel, got %v", receipt.SubmittedPlayers)
	}

	if _, err := session.cancelMove("player_one"); err == nil {
		t.Fatal("expected cancelling again to fail")
	}
}

func TestGameSessionRejectsCancelAfterAllPlayersMove(t *testing.T) {
	session := newGameSession("room_one", "rps", []string{"player_one", "player_two"}, gameModeFreeForAll, 2)

	submitTestMove(t, session, "player_one", moveTypeGainPower, "")
	submitTestMove(t, session, "player_two", moveTypeDefend, "")

	if _, err := session.cancelMove("player_one"); err == nil {
		t.Fatal("expected cancel to fail once all players have moved")
	}
}

func TestGameSessionIncludesMoveCatalog(t *testing.T) {
	session := newGameSession("room_one", gameTypePowerDefenseWave, []string{"player_one", "player_two"}, gameModeFreeForAll, 2)

	payload, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		MoveCatalog []struct {
			ID                   string `json:"id"`
			RequiresTargetPlayer bool   `json:"requiresTargetPlayer"`
		} `json:"moveCatalog"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.MoveCatalog) != 5 {
		t.Fatalf("expected five catalog moves, got %d", len(decoded.MoveCatalog))
	}

	targeted := map[string]bool{}
	for _, entry := range decoded.MoveCatalog {
		targeted[entry.ID] = entry.RequiresTargetPlayer
	}
	if !targeted["wave"] || !targeted["air_cannon"] {
		t.Fatalf("expected targeted moves in catalog, got %v", targeted)
	}
}
