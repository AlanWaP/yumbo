package combat

type MoveCategory string

const (
	MoveGainEnergy MoveCategory = "gain_energy"
	MoveDefend     MoveCategory = "defend"
	MoveAttack     MoveCategory = "attack"
	MoveHeal       MoveCategory = "heal"
	MoveReposition MoveCategory = "reposition"
	MoveSeal       MoveCategory = "seal"
)

type TargetScope string

const (
	TargetNone        TargetScope = "none"
	TargetSingleEnemy TargetScope = "single_enemy"
	TargetAllEnemies  TargetScope = "all_enemies"
	TargetAllPlayers  TargetScope = "all_players"
)

type UsageLimit struct {
	MaxConsecutive int
	CooldownRounds int
	MaxUsesPerGame int
}

type HealthSpec struct {
	Starting int
	Max      int
}

type SealSpec struct {
	ImmuneIfTargetUses string
	NoBanIfTargetUses  string
}

type MoveSpec struct {
	ID           string
	Category     MoveCategory
	EnergyCost   int
	EnergyGain   int
	TargetScope  TargetScope
	UsageLimit   *UsageLimit
	ResetsStreak []string
	// BlocksAttacks limits defend-style moves to specific incoming attacks.
	BlocksAttacks []string
	// GainEnergyFromAttackCost grants the blocked attack's energy cost to the defender.
	GainEnergyFromAttackCost bool
	// ImmuneWhileUsing prevents elimination from incoming attacks while this move is active.
	ImmuneWhileUsing bool
	// ActionMessage overrides the default round log message for this move.
	ActionMessage string
	// SealSpec configures one-time seal behavior for MoveSeal moves.
	SealSpec *SealSpec
	// CounterOnly marks attacks that do not create incoming hits on their own.
	// They only take effect through explicit CounterRule entries.
	CounterOnly bool
}

type SubmittedMove struct {
	PlayerID string
	MoveID   string
	TargetID string
}

type AttackInstance struct {
	AttackerID string
	TargetID   string
	MoveID     string
}
