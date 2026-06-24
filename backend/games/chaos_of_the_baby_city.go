package games

import "github.com/AlanWaP/yumbo/backend/combat"

const (
	MovePrick      = "prick"
	MoveClangClang = "clang_clang"
	MoveVisaRay    = "visa_ray"
	MoveDetonation = "detonation"
	MoveAbsorb     = "absorb"
	MoveKnife      = "knife"
	MoveSeal       = "seal"
	MoveCoverEar   = "cover_ear"
	MoveV          = "v"
)

var defendStreakMoves = []string{MoveDefense, MoveCoverEar, MoveV}

var ChaosOfTheBabyCity = combat.GameDefinition{
	MoveOrder: []string{
		MovePower,
		MoveDefense,
		MoveCoverEar,
		MoveV,
		MoveAbsorb,
		MoveKnife,
		MoveSeal,
		MovePrick,
		MoveClangClang,
		MoveVisaRay,
		MoveDetonation,
	},
	Health: combat.HealthSpec{
		Starting: 1,
		Max:      1,
	},
	VulnerableWhileCharging: []string{MovePower},
	VulnerableWhileUsing:    []string{MoveKnife},
	Moves: map[string]combat.MoveSpec{
		MovePower: {
			ID:           MovePower,
			Category:     combat.MoveGainEnergy,
			EnergyGain:   1,
			TargetScope:  combat.TargetNone,
			ResetsStreak: defendStreakMoves,
		},
		MoveDefense: {
			ID:            MoveDefense,
			Category:      combat.MoveDefend,
			TargetScope:   combat.TargetNone,
			BlocksAttacks: []string{MovePrick},
			UsageLimit: &combat.UsageLimit{
				MaxConsecutive: 2,
			},
		},
		MoveCoverEar: {
			ID:            MoveCoverEar,
			Category:      combat.MoveDefend,
			TargetScope:   combat.TargetNone,
			BlocksAttacks: []string{MoveClangClang},
			UsageLimit: &combat.UsageLimit{
				MaxConsecutive: 2,
			},
			ActionMessage: "covered ears",
		},
		MoveV: {
			ID:            MoveV,
			Category:      combat.MoveDefend,
			TargetScope:   combat.TargetNone,
			BlocksAttacks: []string{MoveVisaRay},
			UsageLimit: &combat.UsageLimit{
				MaxConsecutive: 2,
			},
			ActionMessage: "made a V sign",
		},
		MoveAbsorb: {
			ID:                       MoveAbsorb,
			Category:                 combat.MoveDefend,
			TargetScope:              combat.TargetNone,
			BlocksAttacks:            []string{MovePrick, MoveClangClang, MoveVisaRay},
			GainEnergyFromAttackCost: true,
			ActionMessage:            "absorbed",
		},
		MoveKnife: {
			ID:            MoveKnife,
			Category:      combat.MoveAttack,
			TargetScope:   combat.TargetAllEnemies,
			CounterOnly:   true,
			ActionMessage: "used knife",
		},
		MoveSeal: {
			ID:          MoveSeal,
			Category:    combat.MoveSeal,
			TargetScope: combat.TargetSingleEnemy,
			UsageLimit: &combat.UsageLimit{
				MaxUsesPerGame: 1,
			},
			SealSpec: &combat.SealSpec{
				ImmuneIfTargetUses: MovePower,
				NoBanIfTargetUses:  MoveDetonation,
			},
			ActionMessage: "used seal",
		},
		MovePrick: {
			ID:           MovePrick,
			Category:     combat.MoveAttack,
			EnergyCost:   1,
			TargetScope:  combat.TargetSingleEnemy,
			ResetsStreak: defendStreakMoves,
			ActionMessage: "used prick",
		},
		MoveClangClang: {
			ID:           MoveClangClang,
			Category:     combat.MoveAttack,
			EnergyCost:   2,
			TargetScope:  combat.TargetSingleEnemy,
			ResetsStreak: defendStreakMoves,
			ActionMessage: "used clang clang",
		},
		MoveVisaRay: {
			ID:           MoveVisaRay,
			Category:     combat.MoveAttack,
			EnergyCost:   3,
			TargetScope:  combat.TargetSingleEnemy,
			ResetsStreak: defendStreakMoves,
			ActionMessage: "used visa ray",
		},
		MoveDetonation: {
			ID:               MoveDetonation,
			Category:         combat.MoveAttack,
			EnergyCost:       4,
			TargetScope:      combat.TargetAllEnemies,
			ResetsStreak:     defendStreakMoves,
			ImmuneWhileUsing: true,
			ActionMessage:    "used detonation",
		},
	},
	Interactions: []combat.InteractionRule{
		{
			AttackerMove: MoveDetonation,
			DefenderMove: MoveDetonation,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove:        MovePrick,
			DefenderMove:        MovePrick,
			RequireMutualTarget: true,
			Effect:              combat.EffectMutualOffset,
		},
		{
			AttackerMove: MoveClangClang,
			DefenderMove: MovePrick,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MoveVisaRay,
			DefenderMove: MovePrick,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MoveVisaRay,
			DefenderMove: MoveClangClang,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MoveDetonation,
			DefenderMove: MovePrick,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MoveDetonation,
			DefenderMove: MoveClangClang,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MoveDetonation,
			DefenderMove: MoveVisaRay,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MovePrick,
			DefenderMove: MovePrick,
			Effect:       combat.EffectHit,
		},
		{
			AttackerMove: MovePrick,
			DefenderMove: MoveClangClang,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove: MovePrick,
			DefenderMove: MoveVisaRay,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove: MoveClangClang,
			DefenderMove: MoveVisaRay,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove: MovePrick,
			DefenderMove: MoveDetonation,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove: MoveClangClang,
			DefenderMove: MoveDetonation,
			Effect:       combat.EffectBlock,
		},
		{
			AttackerMove: MoveVisaRay,
			DefenderMove: MoveDetonation,
			Effect:       combat.EffectBlock,
		},
	},
	Counters: []combat.CounterRule{
		{
			CounterMove:     MoveKnife,
			TargetMove:      MoveAbsorb,
			EliminateTarget: true,
			Reason:          "hit by %s's knife",
		},
	},
}
