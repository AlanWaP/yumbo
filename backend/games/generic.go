package games

import "github.com/AlanWaP/yumbo/backend/combat"

const (
	MoveAttack    = "attack"
	MoveDefend    = "defend"
	MoveGainPower = "gain_power"
)

var Generic = combat.GameDefinition{
	MoveOrder: []string{MoveAttack, MoveDefend, MoveGainPower},
	Moves: map[string]combat.MoveSpec{
		MoveAttack: {
			ID:          MoveAttack,
			Category:    combat.MoveAttack,
			EnergyCost:  1,
			TargetScope: combat.TargetSingleEnemy,
		},
		MoveDefend: {
			ID:          MoveDefend,
			Category:    combat.MoveDefend,
			TargetScope: combat.TargetNone,
		},
		MoveGainPower: {
			ID:           MoveGainPower,
			Category:     combat.MoveGainEnergy,
			EnergyGain:   1,
			TargetScope:  combat.TargetNone,
		},
	},
}
