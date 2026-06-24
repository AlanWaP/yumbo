package combat

import "fmt"

type GameDefinition struct {
	MoveOrder               []string
	Moves                   map[string]MoveSpec
	Health                  HealthSpec
	Interactions            []InteractionRule
	Counters                []CounterRule
	VulnerableWhileCharging []string
	VulnerableWhileUsing    []string
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

	if containsMoveID(combatant.BannedMoves, move.MoveID) {
		return fmt.Errorf("move %q is sealed and cannot be used", move.MoveID)
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

	nullified, invulnerable := def.applySeals(moves, input, output)
	attacks := def.expandAttacks(moves, input, nullified)
	immunities := def.applyCounters(moves, output, nullified)
	incoming := groupAttacksByTarget(attacks)
	def.applyAbsorbGains(moves, incoming, nullified, input, output, immunities)

	for playerID, move := range moves {
		if invulnerable[playerID] {
			continue
		}
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

func (def *GameDefinition) applySeals(
	moves map[string]SubmittedMove,
	input RoundInput,
	output RoundOutput,
) (nullified map[string]bool, invulnerable map[string]bool) {
	nullified = map[string]bool{}
	invulnerable = map[string]bool{}

	for _, move := range moves {
		spec, ok := def.Moves[move.MoveID]
		if !ok || spec.Category != MoveSeal || spec.SealSpec == nil {
			continue
		}

		targetMove, ok := moves[move.TargetID]
		if !ok {
			continue
		}
		if spec.SealSpec.ImmuneIfTargetUses != "" && targetMove.MoveID == spec.SealSpec.ImmuneIfTargetUses {
			continue
		}

		invulnerable[move.TargetID] = true
		nullified[move.TargetID] = true

		output.Events = append(output.Events, RoundEvent{
			PlayerID: move.PlayerID,
			MoveID:   move.MoveID,
			TargetID: move.TargetID,
			Message:  fmt.Sprintf("sealed %s's %s", move.TargetID, targetMove.MoveID),
		})

		if spec.SealSpec.NoBanIfTargetUses != "" && targetMove.MoveID == spec.SealSpec.NoBanIfTargetUses {
			continue
		}

		combatant := input.Combatants[move.TargetID]
		if combatant == nil || containsMoveID(combatant.BannedMoves, targetMove.MoveID) {
			continue
		}
		combatant.BannedMoves = append(combatant.BannedMoves, targetMove.MoveID)
	}

	return nullified, invulnerable
}

func (def *GameDefinition) expandAttacks(
	moves map[string]SubmittedMove,
	input RoundInput,
	nullified map[string]bool,
) []AttackInstance {
	attacks := []AttackInstance{}

	for playerID, move := range moves {
		if nullified[playerID] {
			continue
		}
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
	nullified map[string]bool,
) map[string]map[string]bool {
	immunities := map[string]map[string]bool{}

	for counterID, counterMove := range moves {
		if nullified[counterID] {
			continue
		}
		for targetID, targetMove := range moves {
			if nullified[targetID] {
				continue
			}
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

func (def *GameDefinition) applyAbsorbGains(
	moves map[string]SubmittedMove,
	incoming map[string][]AttackInstance,
	nullified map[string]bool,
	input RoundInput,
	output RoundOutput,
	immunities map[string]map[string]bool,
) {
	for playerID, move := range moves {
		if nullified[playerID] {
			continue
		}
		spec := def.Moves[move.MoveID]
		if !spec.GainEnergyFromAttackCost || len(spec.BlocksAttacks) == 0 {
			continue
		}

		combatant := input.Combatants[playerID]
		playerIncoming := incoming[playerID]
		unblocked := filterUnblockedAttacks(playerIncoming, immunities[playerID])

		for _, attack := range unblocked {
			if !blocksAttack(spec, attack.MoveID) {
				continue
			}
			if !def.isAttackBlocked(attack, move, playerIncoming) {
				continue
			}
			attackSpec := def.Moves[attack.MoveID]
			combatant.Energy += attackSpec.EnergyCost
			output.EnergyGained[playerID] += attackSpec.EnergyCost
		}
	}
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

	if containsMoveID(def.VulnerableWhileUsing, move.MoveID) {
		return vulnerableWhileUsingReason(move.MoveID)
	}

	if spec.ImmuneWhileUsing {
		return ""
	}

	if spec.Category == MoveDefend && len(spec.BlocksAttacks) == 0 {
		superBlastCount := countAttacks(unblocked, "super_blast")
		if superBlastCount >= 2 {
			return "defense was broken by multiple super blasts"
		}
		return ""
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
	if blocksAttack(defenderSpec, attack.MoveID) {
		return true
	}

	if defenderSpec.Category == MoveDefend && len(defenderSpec.BlocksAttacks) == 0 {
		return true
	}

	return false
}

func blocksAttack(spec MoveSpec, attackMoveID string) bool {
	if len(spec.BlocksAttacks) == 0 {
		return false
	}
	return containsMoveID(spec.BlocksAttacks, attackMoveID)
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
	case "prick":
		if attack.MoveID == "detonation" {
			return "prick was overpowered by detonation"
		}
		if attack.MoveID == "visa_ray" {
			return "prick was overpowered by visa ray"
		}
		if attack.MoveID == "clang_clang" {
			return "prick was overpowered by clang clang"
		}
		if attack.MoveID == "prick" {
			return "prick did not offset incoming prick"
		}
	case "clang_clang":
		switch attack.MoveID {
		case "detonation":
			return "clang clang was overpowered by detonation"
		case "visa_ray":
			return "clang clang was overpowered by visa ray"
		}
	case "visa_ray":
		if attack.MoveID == "detonation" {
			return "visa ray was overpowered by detonation"
		}
	}
	return "attacked"
}

func vulnerableWhileUsingReason(moveID string) string {
	switch moveID {
	case "air_cannon":
		return "attacked while using air cannon"
	case "knife":
		return "attacked while using knife"
	default:
		return "attacked while using " + moveID
	}
}

func (def *GameDefinition) actionMessage(move SubmittedMove, spec MoveSpec) string {
	if spec.ActionMessage != "" {
		return spec.ActionMessage
	}

	switch spec.Category {
	case MoveGainEnergy:
		return fmt.Sprintf("gained %d power", spec.EnergyGain)
	case MoveDefend:
		return "defended"
	case MoveSeal:
		return "used seal"
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
