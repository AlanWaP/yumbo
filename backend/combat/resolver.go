package combat

import "fmt"

type GameDefinition struct {
	Moves                   map[string]MoveSpec
	Health                  HealthSpec
	Interactions            []InteractionRule
	Counters                []CounterRule
	VulnerableWhileCharging []string
}

type RoundInput struct {
	Combatants   map[string]*Combatant
	Moves        map[string]SubmittedMove
	AliveAtStart map[string]bool
}

type RoundEvent struct {
	PlayerID string
	MoveID   string
	TargetID string
	Message  string
}

type RoundOutput struct {
	Events       []RoundEvent
	Eliminated   map[string]string
	EnergySpent  map[string]int
	EnergyGained map[string]int
}

func ValidateMove(
	def GameDefinition,
	combatants map[string]*Combatant,
	combatant *Combatant,
	move SubmittedMove,
) error {
	spec, ok := def.Moves[move.MoveID]
	if !ok {
		return fmt.Errorf("unknown move type: %s", move.MoveID)
	}

	if err := combatant.Usage.CanUse(move.MoveID, spec.UsageLimit); err != nil {
		return err
	}

	switch spec.TargetScope {
	case TargetNone:
		if move.TargetID != "" {
			return fmt.Errorf("%s does not use a target", move.MoveID)
		}
	case TargetSingleEnemy:
		if move.TargetID == "" {
			return fmt.Errorf("%s requires a target", move.MoveID)
		}
		target := combatants[move.TargetID]
		if target == nil || !target.Alive {
			return fmt.Errorf("%s target must be an alive player", move.MoveID)
		}
		if target.ID == combatant.ID {
			return fmt.Errorf("players cannot target themselves")
		}
		if target.TeamID == combatant.TeamID {
			return fmt.Errorf("players cannot target teammates")
		}
	case TargetAllEnemies, TargetAllPlayers:
		if move.TargetID != "" {
			return fmt.Errorf("%s does not use a target", move.MoveID)
		}
	}

	if spec.EnergyCost > 0 && combatant.Energy < spec.EnergyCost {
		return fmt.Errorf("%s requires %d energy", move.MoveID, spec.EnergyCost)
	}

	return nil
}

func (def *GameDefinition) ResolveRound(input RoundInput) RoundOutput {
	output := RoundOutput{
		Eliminated:   map[string]string{},
		EnergySpent:  map[string]int{},
		EnergyGained: map[string]int{},
	}

	moves := map[string]SubmittedMove{}
	for playerID, move := range input.Moves {
		if input.AliveAtStart[playerID] {
			moves[playerID] = move
		}
	}

	for playerID, move := range moves {
		spec := def.Moves[move.MoveID]
		combatant := input.Combatants[playerID]

		if spec.EnergyCost > 0 {
			combatant.Energy -= spec.EnergyCost
			output.EnergySpent[playerID] += spec.EnergyCost
		}
		if spec.EnergyGain > 0 {
			combatant.Energy += spec.EnergyGain
			output.EnergyGained[playerID] += spec.EnergyGain
		}

		combatant.Usage.RecordUse(move.MoveID, spec, def.Moves)

		output.Events = append(output.Events, RoundEvent{
			PlayerID: playerID,
			MoveID:   move.MoveID,
			TargetID: move.TargetID,
			Message:  def.actionMessage(move, spec),
		})
	}

	attacks := def.expandAttacks(moves, input)
	immunities := def.applyCounters(moves, output)
	incoming := groupAttacksByTarget(attacks)

	for playerID, move := range moves {
		reason := def.eliminationReason(move, incoming[playerID], immunities, output.Eliminated)
		if reason == "" {
			continue
		}
		output.Eliminated[playerID] = reason
	}

	for playerID, reason := range output.Eliminated {
		combatant := input.Combatants[playerID]
		if combatant == nil || !combatant.Alive {
			continue
		}
		combatant.Alive = false
		combatant.Health = 0
		output.Events = append(output.Events, RoundEvent{
			PlayerID: playerID,
			Message:  "eliminated: " + reason,
		})
	}

	return output
}

func (def *GameDefinition) expandAttacks(moves map[string]SubmittedMove, input RoundInput) []AttackInstance {
	attacks := []AttackInstance{}

	for playerID, move := range moves {
		spec := def.Moves[move.MoveID]
		if spec.Category != MoveAttack || spec.CounterOnly {
			continue
		}

		switch spec.TargetScope {
		case TargetSingleEnemy:
			attacks = append(attacks, AttackInstance{
				AttackerID: playerID,
				TargetID:   move.TargetID,
				MoveID:     move.MoveID,
			})
		case TargetAllEnemies:
			attacker := input.Combatants[playerID]
			for targetID, target := range input.Combatants {
				if targetID == playerID || !input.AliveAtStart[targetID] || target.TeamID == attacker.TeamID {
					continue
				}
				attacks = append(attacks, AttackInstance{
					AttackerID: playerID,
					TargetID:   targetID,
					MoveID:     move.MoveID,
				})
			}
		case TargetAllPlayers:
			for targetID := range input.Combatants {
				if targetID == playerID || !input.AliveAtStart[targetID] {
					continue
				}
				attacks = append(attacks, AttackInstance{
					AttackerID: playerID,
					TargetID:   targetID,
					MoveID:     move.MoveID,
				})
			}
		}
	}

	return attacks
}

func (def *GameDefinition) applyCounters(
	moves map[string]SubmittedMove,
	output RoundOutput,
) map[string]map[string]bool {
	immunities := map[string]map[string]bool{}

	for counterID, counterMove := range moves {
		for targetID, targetMove := range moves {
			for _, rule := range def.Counters {
				if counterMove.MoveID != rule.CounterMove || targetMove.MoveID != rule.TargetMove {
					continue
				}
				if rule.CounterMustTargetUser && counterMove.TargetID != targetID {
					continue
				}

				if rule.EliminateTarget {
					output.Eliminated[targetID] = fmt.Sprintf(rule.Reason, counterID)
				}
				if rule.CounterImmuneToTargetAttack {
					if immunities[counterID] == nil {
						immunities[counterID] = map[string]bool{}
					}
					immunities[counterID][targetID] = true
				}
			}
		}
	}

	return immunities
}

func (def *GameDefinition) eliminationReason(
	move SubmittedMove,
	incoming []AttackInstance,
	immunities map[string]map[string]bool,
	counterEliminated map[string]string,
) string {
	if reason, already := counterEliminated[move.PlayerID]; already {
		return reason
	}

	spec, ok := def.Moves[move.MoveID]
	if !ok {
		return ""
	}

	unblocked := filterUnblockedAttacks(incoming, immunities[move.PlayerID])
	if len(unblocked) == 0 {
		return ""
	}

	if containsMoveID(def.VulnerableWhileCharging, move.MoveID) {
		return "powered up while attacked"
	}

	if spec.Category == MoveDefend {
		superBlastCount := countAttacks(unblocked, "super_blast")
		if superBlastCount >= 2 {
			return "defense was broken by multiple super blasts"
		}
		return ""
	}

	if move.MoveID == "super_blast" {
		return ""
	}

	if move.MoveID == "air_cannon" {
		return "attacked while using air cannon"
	}

	for _, attack := range unblocked {
		if def.isAttackBlocked(attack, move, unblocked) {
			continue
		}
		return def.hitReason(attack, move)
	}

	return ""
}

func (def *GameDefinition) isAttackBlocked(
	attack AttackInstance,
	defenderMove SubmittedMove,
	allIncoming []AttackInstance,
) bool {
	for _, rule := range def.Interactions {
		if !rule.matches(attack, defenderMove, allIncoming) {
			continue
		}
		switch rule.Effect {
		case EffectBlock, EffectMutualOffset, EffectAttackerImmune:
			return true
		case EffectHit:
			return false
		}
	}

	defenderSpec := def.Moves[defenderMove.MoveID]
	if defenderSpec.Category == MoveDefend {
		return true
	}

	return false
}

func (def *GameDefinition) hitReason(attack AttackInstance, defenderMove SubmittedMove) string {
	switch defenderMove.MoveID {
	case "wave":
		if attack.MoveID == "super_blast" {
			return "wave was overpowered by super blast"
		}
		if attack.MoveID == "wave" {
			return "wave did not offset incoming wave"
		}
	}
	return "attacked"
}

func (def *GameDefinition) actionMessage(move SubmittedMove, spec MoveSpec) string {
	switch spec.Category {
	case MoveGainEnergy:
		return fmt.Sprintf("gained %d power", spec.EnergyGain)
	case MoveDefend:
		return "defended"
	case MoveAttack:
		switch spec.TargetScope {
		case TargetSingleEnemy:
			switch move.MoveID {
			case "wave":
				return "sent a wave"
			case "air_cannon":
				return "aimed air cannon"
			default:
				return "attacked"
			}
		case TargetAllEnemies:
			return "used super blast"
		default:
			return "attacked"
		}
	default:
		return move.MoveID
	}
}

func groupAttacksByTarget(attacks []AttackInstance) map[string][]AttackInstance {
	grouped := map[string][]AttackInstance{}
	for _, attack := range attacks {
		grouped[attack.TargetID] = append(grouped[attack.TargetID], attack)
	}
	return grouped
}

func filterUnblockedAttacks(incoming []AttackInstance, immuneTo map[string]bool) []AttackInstance {
	filtered := []AttackInstance{}
	for _, attack := range incoming {
		if immuneTo != nil && immuneTo[attack.AttackerID] {
			continue
		}
		filtered = append(filtered, attack)
	}
	return filtered
}

func containsMoveID(moveIDs []string, moveID string) bool {
	for _, candidate := range moveIDs {
		if candidate == moveID {
			return true
		}
	}
	return false
}
