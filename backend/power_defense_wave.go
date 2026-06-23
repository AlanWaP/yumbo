package main

import (
	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

const gameTypePowerDefenseWave = "power_defense_wave"

const (
	moveTypePower      = games.MovePower
	moveTypeDefense    = games.MoveDefense
	moveTypeWave       = games.MoveWave
	moveTypeSuperBlast = games.MoveSuperBlast
	moveTypeAirCannon  = games.MoveAirCannon
)

func applyPowerDefenseWaveRules(rules gameRules) gameRules {
	rules.StartingHealth = games.PowerDefenseWave.Health.Starting
	rules.StartingPower = 0
	return rules
}

func (g *gameSession) validatePowerDefenseWaveMove(currentPlayer *gamePlayer, move submittedMove) error {
	combatants := g.powerDefenseWaveCombatants()
	combatant := combatants[currentPlayer.ID]
	combatMove := combat.SubmittedMove{
		PlayerID: move.PlayerID,
		MoveID:   move.Type,
		TargetID: move.TargetID,
	}
	return combat.ValidateMove(games.PowerDefenseWave, combatants, combatant, combatMove)
}

func (g *gameSession) resolvePowerDefenseWaveRound() {
	input := g.powerDefenseWaveRoundInput()
	output := games.PowerDefenseWave.ResolveRound(input)
	g.applyPowerDefenseWaveOutput(input.Combatants, output)
	g.finishPowerDefenseWaveRound()
}

func (g *gameSession) powerDefenseWaveCombatants() map[string]*combat.Combatant {
	combatants := map[string]*combat.Combatant{}
	for playerID, player := range g.Players {
		combatants[playerID] = &combat.Combatant{
			ID:     player.ID,
			TeamID: player.TeamID,
			Energy: player.Power,
			Health: player.Health,
			Alive:  player.Alive,
			Usage:  powerDefenseWaveUsage(player),
		}
	}
	return combatants
}

func powerDefenseWaveUsage(player *gamePlayer) combat.UsageTracker {
	usage := combat.NewUsageTracker()
	if player.DefenseStreak > 0 {
		usage.Consecutive[games.MoveDefense] = player.DefenseStreak
	}
	return usage
}

func (g *gameSession) powerDefenseWaveRoundInput() combat.RoundInput {
	combatants := g.powerDefenseWaveCombatants()
	aliveAtStart := map[string]bool{}
	moves := map[string]combat.SubmittedMove{}

	for playerID, player := range g.Players {
		aliveAtStart[playerID] = player.Alive
	}

	for playerID, move := range g.PendingMoves {
		if !aliveAtStart[playerID] {
			continue
		}
		moves[playerID] = combat.SubmittedMove{
			PlayerID: playerID,
			MoveID:   move.Type,
			TargetID: move.TargetID,
		}
	}

	return combat.RoundInput{
		Combatants:   combatants,
		Moves:        moves,
		AliveAtStart: aliveAtStart,
	}
}

func (g *gameSession) applyPowerDefenseWaveOutput(combatants map[string]*combat.Combatant, output combat.RoundOutput) {
	results := []roundResult{}
	for _, event := range output.Events {
		results = append(results, roundResult{
			PlayerID: event.PlayerID,
			MoveType: event.MoveID,
			TargetID: event.TargetID,
			Message:  event.Message,
		})
	}

	for playerID, combatant := range combatants {
		player := g.Players[playerID]
		if player == nil {
			continue
		}
		player.Power = combatant.Energy
		player.Health = combatant.Health
		player.Alive = combatant.Alive
		player.DefenseStreak = combatant.Usage.Consecutive[games.MoveDefense]
	}

	g.LastResults = results
	g.PendingMoves = map[string]submittedMove{}
}

func (g *gameSession) finishPowerDefenseWaveRound() {
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
