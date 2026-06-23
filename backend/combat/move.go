package combat

type MoveCategory string

const (
	MoveGainEnergy MoveCategory = "gain_energy"
	MoveDefend     MoveCategory = "defend"
	MoveAttack     MoveCategory = "attack"
	MoveHeal       MoveCategory = "heal"
	MoveReposition MoveCategory = "reposition"
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

type MoveSpec struct {
	ID           string
	Category     MoveCategory
	EnergyCost   int
	EnergyGain   int
	TargetScope  TargetScope
	UsageLimit   *UsageLimit
	ResetsStreak []string
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
