package main

import "fmt"

const (
	gameTypePowerDefenseWave = "power_defense_wave"

	moveTypePower      = "power"
	moveTypeDefense    = "defense"
	moveTypeWave       = "wave"
	moveTypeSuperBlast = "super_blast"
	moveTypeAirCannon  = "air_cannon"
)

type powerDefenseWaveAttack struct {
	AttackerID string
	MoveType   string
}

func applyPowerDefenseWaveRules(rules gameRules) gameRules {
	rules.StartingHealth = 1
	rules.StartingPower = 0
	return rules
}

func (g *gameSession) validatePowerDefenseWaveMove(currentPlayer *gamePlayer, move submittedMove) error {
	switch move.Type {
	case moveTypePower:
		if move.TargetID != "" {
			return fmt.Errorf("power does not use a target")
		}
	case moveTypeDefense:
		if move.TargetID != "" {
			return fmt.Errorf("defense does not use a target")
		}
		if currentPlayer.DefenseStreak >= 2 {
			return fmt.Errorf("players cannot use defense three rounds in a row")
		}
	case moveTypeWave:
		if currentPlayer.Power < g.Rules.WaveCost {
			return fmt.Errorf("wave requires %d power", g.Rules.WaveCost)
		}
		if err := g.validateEnemyTarget(currentPlayer, move.TargetID, "wave target"); err != nil {
			return err
		}
	case moveTypeSuperBlast:
		if move.TargetID != "" {
			return fmt.Errorf("super blast does not use a target")
		}
		if currentPlayer.Power < g.Rules.SuperBlastCost {
			return fmt.Errorf("super blast requires %d power", g.Rules.SuperBlastCost)
		}
	case moveTypeAirCannon:
		if err := g.validateEnemyTarget(currentPlayer, move.TargetID, "air cannon target"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown move type: %s", move.Type)
	}

	return nil
}

func (g *gameSession) validateEnemyTarget(currentPlayer *gamePlayer, targetID string, label string) error {
	target := g.Players[targetID]
	if target == nil || !target.Alive {
		return fmt.Errorf("%s must be an alive player", label)
	}
	if target.ID == currentPlayer.ID {
		return fmt.Errorf("players cannot target themselves")
	}
	if target.TeamID == currentPlayer.TeamID {
		return fmt.Errorf("players cannot target teammates")
	}
	return nil
}

func (g *gameSession) resolvePowerDefenseWaveRound() {
	aliveAtRoundStart := map[string]bool{}
	moves := map[string]submittedMove{}
	incomingAttacks := map[string][]powerDefenseWaveAttack{}
	eliminatedPlayers := map[string]string{}
	superBlasters := []string{}
	results := []roundResult{}

	for playerID, player := range g.Players {
		aliveAtRoundStart[playerID] = player.Alive
	}
	for playerID, move := range g.PendingMoves {
		if aliveAtRoundStart[playerID] {
			moves[playerID] = move
		}
	}

	for playerID, move := range moves {
		player := g.Players[playerID]
		switch move.Type {
		case moveTypePower:
			player.Power += g.Rules.GainPowerAmount
			player.DefenseStreak = 0
			results = append(results, roundResult{
				PlayerID: playerID,
				MoveType: move.Type,
				Message:  fmt.Sprintf("gained %d power", g.Rules.GainPowerAmount),
			})
		case moveTypeDefense:
			player.DefenseStreak++
			results = append(results, roundResult{
				PlayerID: playerID,
				MoveType: move.Type,
				Message:  "defended",
			})
		case moveTypeWave:
			player.Power -= g.Rules.WaveCost
			player.DefenseStreak = 0
			incomingAttacks[move.TargetID] = append(incomingAttacks[move.TargetID], powerDefenseWaveAttack{
				AttackerID: playerID,
				MoveType:   moveTypeWave,
			})
			results = append(results, roundResult{
				PlayerID: playerID,
				MoveType: move.Type,
				TargetID: move.TargetID,
				Message:  "sent a wave",
			})
		case moveTypeSuperBlast:
			player.Power -= g.Rules.SuperBlastCost
			player.DefenseStreak = 0
			superBlasters = append(superBlasters, playerID)
			results = append(results, roundResult{
				PlayerID: playerID,
				MoveType: move.Type,
				Message:  "used super blast",
			})
		case moveTypeAirCannon:
			player.DefenseStreak = 0
			results = append(results, roundResult{
				PlayerID: playerID,
				MoveType: move.Type,
				TargetID: move.TargetID,
				Message:  "aimed air cannon",
			})
		}
	}

	for _, blasterID := range superBlasters {
		blaster := g.Players[blasterID]
		if blaster == nil {
			continue
		}
		for targetID, target := range g.Players {
			if targetID == blasterID || !aliveAtRoundStart[targetID] || target.TeamID == blaster.TeamID {
				continue
			}
			incomingAttacks[targetID] = append(incomingAttacks[targetID], powerDefenseWaveAttack{
				AttackerID: blasterID,
				MoveType:   moveTypeSuperBlast,
			})
		}
	}

	for playerID, move := range moves {
		if move.Type != moveTypeAirCannon {
			continue
		}
		targetMove, exists := moves[move.TargetID]
		if exists && targetMove.Type == moveTypeSuperBlast {
			eliminatedPlayers[move.TargetID] = fmt.Sprintf("hit by %s's air cannon", playerID)
		}
	}

	for playerID, attacks := range incomingAttacks {
		if len(attacks) == 0 || !aliveAtRoundStart[playerID] {
			continue
		}
		move := moves[playerID]
		if move.Type == moveTypeDefense && len(superBlasters) <= 1 {
			continue
		}

		reason := g.powerDefenseWaveEliminationReason(move, attacks, superBlasters)
		if reason == "" {
			continue
		}
		eliminatedPlayers[playerID] = reason
	}

	for playerID, reason := range eliminatedPlayers {
		player := g.Players[playerID]
		if player == nil || !player.Alive {
			continue
		}
		player.Alive = false
		player.Health = 0
		results = append(results, roundResult{
			PlayerID: playerID,
			Message:  "eliminated: " + reason,
		})
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

func (g *gameSession) powerDefenseWaveEliminationReason(move submittedMove, attacks []powerDefenseWaveAttack, superBlasters []string) string {
	hasSuperBlastAttack := false
	for _, attack := range attacks {
		if attack.MoveType == moveTypeSuperBlast {
			hasSuperBlastAttack = true
			break
		}
	}

	switch move.Type {
	case moveTypePower:
		if len(attacks) > 0 {
			return "powered up while attacked"
		}
	case moveTypeDefense:
		if len(superBlasters) > 1 {
			return "defense was broken by multiple super blasts"
		}
	case moveTypeWave:
		if hasSuperBlastAttack {
			return "wave was overpowered by super blast"
		}
		for _, attack := range attacks {
			if attack.MoveType == moveTypeWave && move.TargetID != attack.AttackerID {
				return "wave did not offset incoming wave"
			}
		}
	case moveTypeSuperBlast:
		if hasSuperBlastAttack {
			return "hit by super blast"
		}
	case moveTypeAirCannon:
		if g.hasUnblockedAirCannonAttack(move, attacks) {
			return "attacked while using air cannon"
		}
	default:
		if len(attacks) > 0 {
			return "attacked"
		}
	}

	return ""
}

func (g *gameSession) hasUnblockedAirCannonAttack(move submittedMove, attacks []powerDefenseWaveAttack) bool {
	for _, attack := range attacks {
		if attack.AttackerID == move.TargetID && attack.MoveType == moveTypeSuperBlast {
			continue
		}
		return true
	}

	return false
}
