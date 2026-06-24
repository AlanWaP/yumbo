package main

import (
	"github.com/AlanWaP/yumbo/backend/combat"
	"github.com/AlanWaP/yumbo/backend/games"
)

const gameTypeChaosOfTheBabyCity = "chaos_of_the_baby_city"

func applyChaosOfTheBabyCityRules(rules gameRules) gameRules {
	rules.StartingHealth = games.ChaosOfTheBabyCity.Health.Starting
	rules.StartingPower = 0
	return rules
}

func (g *gameSession) validateChaosOfTheBabyCityMove(currentPlayer *gamePlayer, move submittedMove) error {
	combatants := g.chaosOfTheBabyCityCombatants()
	combatant := combatants[currentPlayer.ID]
	combatMove := combat.SubmittedMove{
		PlayerID: move.PlayerID,
		MoveID:   move.Type,
		TargetID: move.TargetID,
	}
	return combat.ValidateMove(games.ChaosOfTheBabyCity, combatants, combatant, combatMove)
}

func (g *gameSession) resolveChaosOfTheBabyCityRound() {
	input := g.chaosOfTheBabyCityRoundInput()
	output := games.ChaosOfTheBabyCity.ResolveRound(input)
	g.applyChaosOfTheBabyCityOutput(input.Combatants, output)
	g.finishChaosOfTheBabyCityRound()
}

func (g *gameSession) chaosOfTheBabyCityCombatants() map[string]*combat.Combatant {
	combatants := map[string]*combat.Combatant{}
	for playerID, player := range g.Players {
		combatants[playerID] = &combat.Combatant{
			ID:          player.ID,
			TeamID:      player.TeamID,
			Energy:      player.Power,
			Health:      player.Health,
			Alive:       player.Alive,
			BannedMoves: append([]string(nil), player.BannedMoves...),
			Usage:       chaosOfTheBabyCityUsage(player),
		}
	}
	return combatants
}

func chaosOfTheBabyCityUsage(player *gamePlayer) combat.UsageTracker {
	usage := combat.NewUsageTracker()
	for moveID, streak := range player.UsageStreaks {
		usage.Consecutive[moveID] = streak
	}
	for moveID, uses := range player.MoveUses {
		usage.TotalUses[moveID] = uses
	}
	return usage
}

func (g *gameSession) chaosOfTheBabyCityRoundInput() combat.RoundInput {
	combatants := g.chaosOfTheBabyCityCombatants()
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

func (g *gameSession) applyChaosOfTheBabyCityOutput(combatants map[string]*combat.Combatant, output combat.RoundOutput) {
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
		player.BannedMoves = append([]string(nil), combatant.BannedMoves...)
		player.UsageStreaks = map[string]int{}
		for moveID, streak := range combatant.Usage.Consecutive {
			player.UsageStreaks[moveID] = streak
		}
		player.MoveUses = map[string]int{}
		for moveID, uses := range combatant.Usage.TotalUses {
			player.MoveUses[moveID] = uses
		}
	}

	g.LastResults = results
	g.PendingMoves = map[string]submittedMove{}
}

func (g *gameSession) finishChaosOfTheBabyCityRound() {
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
