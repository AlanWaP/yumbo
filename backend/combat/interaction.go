package combat

type InteractionEffect string

const (
	EffectHit              InteractionEffect = "hit"
	EffectBlock            InteractionEffect = "block"
	EffectMutualOffset     InteractionEffect = "mutual_offset"
	EffectAttackerImmune   InteractionEffect = "attacker_immune"
	EffectCounterEliminate InteractionEffect = "counter_eliminate"
)

type InteractionRule struct {
	AttackerMove string
	DefenderMove string

	RequireMutualTarget bool
	AttackerTargetsDefender bool

	MinAttackerCount int
	MaxAttackerCount int

	Effect InteractionEffect
	Reason string
}

type CounterRule struct {
	CounterMove string
	TargetMove  string

	CounterMustTargetUser bool

	EliminateTarget               bool
	CounterImmuneToTargetAttack   bool
	Reason                        string
}

func (rule InteractionRule) matches(
	attack AttackInstance,
	defenderMove SubmittedMove,
	attacksOnDefender []AttackInstance,
) bool {
	if rule.AttackerMove != "" && attack.MoveID != rule.AttackerMove {
		return false
	}
	if rule.DefenderMove != "" && defenderMove.MoveID != rule.DefenderMove {
		return false
	}
	if rule.AttackerTargetsDefender && attack.TargetID != defenderMove.PlayerID {
		return false
	}
	if rule.RequireMutualTarget && defenderMove.TargetID != attack.AttackerID {
		return false
	}
	if rule.MinAttackerCount > 0 || rule.MaxAttackerCount > 0 {
		count := countAttacks(attacksOnDefender, rule.AttackerMove)
		if rule.MinAttackerCount > 0 && count < rule.MinAttackerCount {
			return false
		}
		if rule.MaxAttackerCount > 0 && count > rule.MaxAttackerCount {
			return false
		}
	}
	return true
}

func countAttacks(attacks []AttackInstance, moveID string) int {
	count := 0
	for _, attack := range attacks {
		if moveID == "" || attack.MoveID == moveID {
			count++
		}
	}
	return count
}
