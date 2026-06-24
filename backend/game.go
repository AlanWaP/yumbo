package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

const (
	gameModeFreeForAll = "free_for_all"
	gameModeTeam       = "team"

	gamePhaseWaitingForMoves = "waiting_for_moves"
	gamePhaseFinished        = "finished"

	moveTypeAttack    = "attack"
	moveTypeDefend    = "defend"
	moveTypeGainPower = "gain_power"
)

type gameRules struct {
	StartingHealth  int `json:"startingHealth"`
	StartingPower   int `json:"startingPower"`
	AttackCost      int `json:"attackCost"`
	AttackDamage    int `json:"attackDamage"`
	GainPowerAmount int `json:"gainPowerAmount"`
	WaveCost        int `json:"waveCost"`
	SuperBlastCost  int `json:"superBlastCost"`
	RoundSeconds    int `json:"roundSeconds"`
}

type gameSession struct {
	ID           string                   `json:"id"`
	GameType     string                   `json:"gameType"`
	Mode         string                   `json:"mode"`
	Round        int                      `json:"round"`
	Phase        string                   `json:"phase"`
	Players      map[string]*gamePlayer   `json:"players"`
	Teams        map[string][]string      `json:"teams"`
	PendingMoves map[string]submittedMove `json:"-"`
	Rules        gameRules                `json:"rules"`
	MoveCatalog  []combat.MoveCatalogEntry `json:"moveCatalog"`
	Deadline     *time.Time               `json:"deadline,omitempty"`
	WinnerTeamID string                   `json:"winnerTeamId,omitempty"`
	Winners      []string                 `json:"winners,omitempty"`
	LastResults  []roundResult            `json:"lastResults,omitempty"`
}

func (g *gameSession) MarshalJSON() ([]byte, error) {
	type gameSessionPublic gameSession
	return json.Marshal(struct {
		gameSessionPublic
		SubmittedPlayers []string `json:"submittedPlayers,omitempty"`
	}{
		gameSessionPublic: gameSessionPublic(*g),
		SubmittedPlayers:  g.submittedPlayers(),
	})
}

type gamePlayer struct {
	ID            string         `json:"id"`
	TeamID        string         `json:"teamId"`
	Health        int            `json:"health"`
	Power         int            `json:"power"`
	DefenseStreak int            `json:"defenseStreak"`
	UsageStreaks  map[string]int `json:"usageStreaks,omitempty"`
	BannedMoves   []string       `json:"bannedMoves,omitempty"`
	MoveUses      map[string]int `json:"moveUses,omitempty"`
	Alive         bool           `json:"alive"`
}

type submittedMove struct {
	PlayerID string `json:"playerId"`
	Type     string `json:"moveType"`
	TargetID string `json:"targetId,omitempty"`
}

type gameMovePayload struct {
	MoveType string `json:"moveType"`
	TargetID string `json:"targetId,omitempty"`
}

type gameMoveReceipt struct {
	PlayerID         string   `json:"playerId"`
	Round            int      `json:"round"`
	SubmittedPlayers []string `json:"submittedPlayers"`
	NeededPlayers    []string `json:"neededPlayers"`
}

type roundResult struct {
	PlayerID string `json:"playerId,omitempty"`
	MoveType string `json:"moveType"`
	TargetID string `json:"targetId,omitempty"`
	Damage   int    `json:"damage,omitempty"`
	Blocked  bool   `json:"blocked,omitempty"`
	Message  string `json:"message"`
}

func defaultGameRules() gameRules {
	return gameRules{
		StartingHealth:  10,
		StartingPower:   1,
		AttackCost:      1,
		AttackDamage:    2,
		GainPowerAmount: 1,
		WaveCost:        1,
		SuperBlastCost:  3,
		RoundSeconds:    0,
	}
}

func gameRulesForType(gameType string) gameRules {
	rules := defaultGameRules()
	switch gameType {
	case gameTypePowerDefenseWave:
		return applyPowerDefenseWaveRules(rules)
	case gameTypeChaosOfTheBabyCity:
		return applyChaosOfTheBabyCityRules(rules)
	}
	return rules
}

func moveCatalogForType(gameType string, rules gameRules) []combat.MoveCatalogEntry {
	definition := games.Generic
	switch gameType {
	case gameTypePowerDefenseWave:
		definition = games.PowerDefenseWave
	case gameTypeChaosOfTheBabyCity:
		definition = games.ChaosOfTheBabyCity
	}

	entries := combat.BuildMoveCatalog(definition)
	for index := range entries {
		switch entries[index].ID {
		case games.MoveAttack:
			entries[index].EnergyCost = rules.AttackCost
		case games.MoveGainPower:
			entries[index].EnergyGain = rules.GainPowerAmount
		case games.MovePower:
			entries[index].EnergyGain = rules.GainPowerAmount
		case games.MoveWave:
			entries[index].EnergyCost = rules.WaveCost
		case games.MoveSuperBlast:
			entries[index].EnergyCost = rules.SuperBlastCost
		}
	}

	return entries
}

func normalizeGameMode(requestedMode string, playerCount int) (string, int, error) {
	if requestedMode == "" || requestedMode == gameModeFreeForAll {
		return gameModeFreeForAll, playerCount, nil
	}
	if requestedMode != gameModeTeam {
		return "", 0, fmt.Errorf("gameMode must be %q or %q", gameModeFreeForAll, gameModeTeam)
	}
	if playerCount%2 != 0 {
		return "", 0, fmt.Errorf("team games require an even player count")
	}
	return gameModeTeam, 2, nil
}

func newGameSession(roomID string, gameType string, playerIDs []string, mode string, teamCount int) *gameSession {
	rules := gameRulesForType(gameType)
	session := &gameSession{
		ID:           roomID,
		GameType:     gameType,
		Mode:         mode,
		Round:        1,
		Phase:        gamePhaseWaitingForMoves,
		Players:      map[string]*gamePlayer{},
		Teams:        map[string][]string{},
		PendingMoves: map[string]submittedMove{},
		Rules:        rules,
		MoveCatalog:  moveCatalogForType(gameType, rules),
	}

	if rules.RoundSeconds > 0 {
		deadline := time.Now().Add(time.Duration(rules.RoundSeconds) * time.Second)
		session.Deadline = &deadline
	}

	for index, playerID := range playerIDs {
		teamID := playerID
		if mode == gameModeTeam {
			teamID = fmt.Sprintf("team_%d", index%teamCount+1)
		}

		session.Players[playerID] = &gamePlayer{
			ID:     playerID,
			TeamID: teamID,
			Health: rules.StartingHealth,
			Power:  rules.StartingPower,
			Alive:  true,
		}
		session.Teams[teamID] = append(session.Teams[teamID], playerID)
	}

	return session
}

func (g *gameSession) submitMove(playerID string, payload json.RawMessage) (*gameMoveReceipt, bool, error) {
	if g == nil {
		return nil, false, fmt.Errorf("game has not started")
	}
	if g.Phase == gamePhaseFinished {
		return nil, false, fmt.Errorf("game is already finished")
	}

	currentPlayer := g.Players[playerID]
	if currentPlayer == nil {
		return nil, false, fmt.Errorf("player is not in this game")
	}
	if !currentPlayer.Alive {
		return nil, false, fmt.Errorf("eliminated players cannot move")
	}
	if _, exists := g.PendingMoves[playerID]; exists {
		return nil, false, fmt.Errorf("player already moved this round")
	}

	var movePayload gameMovePayload
	if err := json.Unmarshal(payload, &movePayload); err != nil {
		return nil, false, fmt.Errorf("invalid game move payload")
	}

	move := submittedMove{
		PlayerID: playerID,
		Type:     movePayload.MoveType,
		TargetID: movePayload.TargetID,
	}
	if err := g.validateMove(currentPlayer, move); err != nil {
		return nil, false, err
	}

	g.PendingMoves[playerID] = move
	receipt := &gameMoveReceipt{
		PlayerID:         playerID,
		Round:            g.Round,
		SubmittedPlayers: g.submittedPlayers(),
		NeededPlayers:    g.alivePlayers(),
	}

	return receipt, g.hasAllMoves(), nil
}

func (g *gameSession) cancelMove(playerID string) (*gameMoveReceipt, error) {
	if g == nil {
		return nil, fmt.Errorf("game has not started")
	}
	if g.Phase == gamePhaseFinished {
		return nil, fmt.Errorf("game is already finished")
	}

	currentPlayer := g.Players[playerID]
	if currentPlayer == nil {
		return nil, fmt.Errorf("player is not in this game")
	}
	if !currentPlayer.Alive {
		return nil, fmt.Errorf("eliminated players cannot cancel moves")
	}
	if _, exists := g.PendingMoves[playerID]; !exists {
		return nil, fmt.Errorf("player has not moved this round")
	}
	if g.hasAllMoves() {
		return nil, fmt.Errorf("all players have moved; round is resolving")
	}

	delete(g.PendingMoves, playerID)
	return &gameMoveReceipt{
		PlayerID:         playerID,
		Round:            g.Round,
		SubmittedPlayers: g.submittedPlayers(),
		NeededPlayers:    g.alivePlayers(),
	}, nil
}

func (g *gameSession) validateMove(currentPlayer *gamePlayer, move submittedMove) error {
	switch g.GameType {
	case gameTypePowerDefenseWave:
		return g.validatePowerDefenseWaveMove(currentPlayer, move)
	case gameTypeChaosOfTheBabyCity:
		return g.validateChaosOfTheBabyCityMove(currentPlayer, move)
	}

	switch move.Type {
	case moveTypeAttack:
		if currentPlayer.Power < g.Rules.AttackCost {
			return fmt.Errorf("attack requires %d power", g.Rules.AttackCost)
		}
		target := g.Players[move.TargetID]
		if target == nil || !target.Alive {
			return fmt.Errorf("attack target must be an alive player")
		}
		if target.ID == currentPlayer.ID {
			return fmt.Errorf("players cannot attack themselves")
		}
		if target.TeamID == currentPlayer.TeamID {
			return fmt.Errorf("players cannot attack teammates")
		}
	case moveTypeDefend, moveTypeGainPower:
		if move.TargetID != "" {
			return fmt.Errorf("%s does not use a target", move.Type)
		}
	default:
		return fmt.Errorf("unknown move type: %s", move.Type)
	}

	return nil
}

func (g *gameSession) resolveRound() {
	switch g.GameType {
	case gameTypePowerDefenseWave:
		g.resolvePowerDefenseWaveRound()
		return
	case gameTypeChaosOfTheBabyCity:
		g.resolveChaosOfTheBabyCityRound()
		return
	}

	aliveAtRoundStart := map[string]bool{}
	defendingPlayers := map[string]bool{}
	pendingDamage := map[string]int{}
	results := []roundResult{}

	for playerID, player := range g.Players {
		aliveAtRoundStart[playerID] = player.Alive
	}

	for _, move := range g.PendingMoves {
		if move.Type == moveTypeDefend {
			defendingPlayers[move.PlayerID] = true
			results = append(results, roundResult{
				PlayerID: move.PlayerID,
				MoveType: move.Type,
				Message:  "defended",
			})
		}
	}

	for _, move := range g.PendingMoves {
		if move.Type != moveTypeGainPower {
			continue
		}
		player := g.Players[move.PlayerID]
		if player == nil || !player.Alive {
			continue
		}
		player.Power += g.Rules.GainPowerAmount
		results = append(results, roundResult{
			PlayerID: move.PlayerID,
			MoveType: move.Type,
			Message:  fmt.Sprintf("gained %d power", g.Rules.GainPowerAmount),
		})
	}

	for _, move := range g.PendingMoves {
		if move.Type != moveTypeAttack {
			continue
		}
		attacker := g.Players[move.PlayerID]
		target := g.Players[move.TargetID]
		if attacker == nil || target == nil || !aliveAtRoundStart[attacker.ID] || !aliveAtRoundStart[target.ID] {
			continue
		}

		attacker.Power -= g.Rules.AttackCost
		result := roundResult{
			PlayerID: move.PlayerID,
			MoveType: move.Type,
			TargetID: move.TargetID,
			Message:  "attacked",
		}
		if defendingPlayers[target.ID] {
			result.Blocked = true
			result.Message = "attack blocked"
		} else {
			pendingDamage[target.ID] += g.Rules.AttackDamage
			result.Damage = g.Rules.AttackDamage
		}
		results = append(results, result)
	}

	for playerID, damage := range pendingDamage {
		player := g.Players[playerID]
		if player == nil {
			continue
		}
		player.Health -= damage
		if player.Health <= 0 {
			player.Health = 0
			player.Alive = false
			for index := range results {
				if results[index].TargetID == playerID && results[index].Damage > 0 {
					results[index].Message = "attack eliminated target"
				}
			}
		}
	}

	g.LastResults = results
	g.PendingMoves = map[string]submittedMove{}

	winnerTeamID, winners, finished := g.winner()
	if finished {
		g.Phase = gamePhaseFinished
		g.WinnerTeamID = winnerTeamID
		g.Winners = winners
		g.Deadline = nil
		return
	}

	g.startNextRound()
}

func (g *gameSession) startNextRound() {
	g.Round++
	if g.Rules.RoundSeconds > 0 {
		deadline := time.Now().Add(time.Duration(g.Rules.RoundSeconds) * time.Second)
		g.Deadline = &deadline
	}
}

func (g *gameSession) hasAllMoves() bool {
	return len(g.PendingMoves) == len(g.alivePlayers())
}

func (g *gameSession) alivePlayers() []string {
	players := []string{}
	for playerID, player := range g.Players {
		if player.Alive {
			players = append(players, playerID)
		}
	}
	return players
}

func (g *gameSession) submittedPlayers() []string {
	players := []string{}
	for playerID := range g.PendingMoves {
		players = append(players, playerID)
	}
	return players
}

func (g *gameSession) winner() (string, []string, bool) {
	aliveTeams := map[string]bool{}
	for _, player := range g.Players {
		if player.Alive {
			aliveTeams[player.TeamID] = true
		}
	}
	if len(aliveTeams) > 1 {
		return "", nil, false
	}

	for teamID := range aliveTeams {
		winners := []string{}
		for _, playerID := range g.Teams[teamID] {
			if player := g.Players[playerID]; player != nil && player.Alive {
				winners = append(winners, playerID)
			}
		}
		return teamID, winners, true
	}

	return "", nil, true
}

func (g *gameSession) finishDueToPlayerDeparture(departedPlayerID string) {
	if g == nil || g.Phase == gamePhaseFinished {
		return
	}

	if departedPlayer := g.Players[departedPlayerID]; departedPlayer != nil {
		departedPlayer.Alive = false
		departedPlayer.Health = 0
	}

	g.PendingMoves = map[string]submittedMove{}
	g.LastResults = append(g.LastResults, roundResult{
		PlayerID: departedPlayerID,
		Message:  "left the game",
	})

	winnerTeamID, winners, _ := g.winner()
	g.Phase = gamePhaseFinished
	g.WinnerTeamID = winnerTeamID
	g.Winners = winners
	g.Deadline = nil
}

func marshalPayload(payload any) json.RawMessage {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return bytes
}

func pendingMoveForPlayer(g *gameSession, playerID string) json.RawMessage {
	if g == nil {
		return nil
	}
	move, ok := g.PendingMoves[playerID]
	if !ok {
		return nil
	}
	return marshalPayload(gameMovePayload{
		MoveType: move.Type,
		TargetID: move.TargetID,
	})
}
