package games

import "github.com/AlanWaP/yumbo/backend/combat"

const (
	MovePower      = "power"
	MoveDefense    = "defense"
	MoveWave       = "wave"
	MoveSuperBlast = "super_blast"
	MoveAirCannon  = "air_cannon"
)

var PowerDefenseWave = combat.GameDefinition{
	MoveOrder: []string{
		MovePower,
		MoveDefense,
		MoveAirCannon,
		MoveWave,
		MoveSuperBlast,
	},
	Health: combat.HealthSpec{
		Starting: 1,
		Max:      1,
	},
	VulnerableWhileCharging: []string{MovePower},
	VulnerableWhileUsing:    []string{MoveAirCannon},
	Moves: map[string]combat.MoveSpec{
		MovePower: {
			ID:           MovePower,
			Category:     combat.MoveGainEnergy,
			EnergyGain:   1,
			TargetScope:  combat.TargetNone,
			ResetsStreak: []string{MoveDefense},
		},
		MoveDefense: {
			ID:          MoveDefense,
			Category:    combat.MoveDefend,
			TargetScope: combat.TargetNone,
			UsageLimit: &combat.UsageLimit{
				MaxConsecutive: 2,
			},
		},
		MoveWave: {
			ID:           MoveWave,
			Category:     combat.MoveAttack,
			EnergyCost:   1,
			TargetScope:  combat.TargetSingleEnemy,
			ResetsStreak: []string{MoveDefense},
			ActionMessage: "sent a wave",
		},
		MoveSuperBlast: {
			ID:               MoveSuperBlast,
			Category:         combat.MoveAttack,
			EnergyCost:       3,
			TargetScope:      combat.TargetAllEnemies,
			ResetsStreak:     []string{MoveDefense},
			ImmuneWhileUsing: true,
			ActionMessage:    "used super blast",
		},
		MoveAirCannon: {
			ID:            MoveAirCannon,
			Category:      combat.MoveAttack,
			TargetScope:   combat.TargetSingleEnemy,
			ResetsStreak:  []string{MoveDefense},
			CounterOnly:   true,
			ActionMessage: "aimed air cannon",
		},
	},
	Interactions: []combat.InteractionRule{
		{
			AttackerMove: "super_blast",
			DefenderMove: "super_blast",
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove:        "wave",
			DefenderMove:        "wave",
			RequireMutualTarget: true,
			Effect:              combat.EffectMutualOffset,
		},
		{
			AttackerMove: "wave",
			DefenderMove: "defense",
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove:     "super_blast",
			DefenderMove:     "defense",
			MinAttackerCount: 1,
			MaxAttackerCount: 1,
			Effect:           combat.EffectBlock,
		},
		{
			AttackerMove: "super_blast",
			DefenderMove: "wave",
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: "wave",
			DefenderMove: "wave",
			Effect:       combat.EffectHit,
		},
	},
	Counters: []combat.CounterRule{
		{
			CounterMove:                 MoveAirCannon,
			TargetMove:                  MoveSuperBlast,
			CounterMustTargetUser:       true,
			EliminateTarget:             true,
			CounterImmuneToTargetAttack: true,
			Reason:                      "hit by %s's air cannon",
		},
	},
}
