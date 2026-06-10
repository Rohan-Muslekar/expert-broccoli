package rules

import (
	"cheat-detection/feature-engine/engine"
)

type Rule struct {
	Name       string
	CheatType  string
	Confidence float64
	Threshold  float64
	Check      func(fv engine.FeatureVector) (triggered bool, value float64)
}

var All = []Rule{
	{
		Name:       "speed_cap",
		CheatType:  "speedhack",
		Confidence: 0.95,
		Threshold:  7.0,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.SpeedMax1s > 7.0, fv.SpeedMax1s
		},
	},
	{
		Name:       "aim_snap",
		CheatType:  "aimbot",
		Confidence: 0.95,
		Threshold:  2.0,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.AimDeltaMax1s > 2.0, fv.AimDeltaMax1s
		},
	},
	{
		Name:       "inhuman_accuracy",
		CheatType:  "aimbot",
		Confidence: 0.90,
		Threshold:  0.85,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.HitRate5s > 0.85 && fv.ShotsFired5s > 30, fv.HitRate5s
		},
	},
	{
		Name:       "aim_lock",
		CheatType:  "aimbot",
		Confidence: 0.90,
		Threshold:  0.90,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.AimLockRatio5s > 0.90 && fv.EnemiesVisible > 0, fv.AimLockRatio5s
		},
	},
	{
		Name:       "prefire",
		CheatType:  "wallhack",
		Confidence: 0.90,
		Threshold:  0.60,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.PrefireRatio5s > 0.60 && fv.ShotsFired5s > 20, fv.PrefireRatio5s
		},
	},
	{
		Name:       "triggerbot_reaction",
		CheatType:  "triggerbot",
		Confidence: 0.90,
		Threshold:  3.0,
		Check: func(fv engine.FeatureVector) (bool, float64) {
			return fv.ReactionTimeMean5s > 0 && fv.ReactionTimeMean5s < 3.0 && fv.ShotsFired5s > 10, fv.ReactionTimeMean5s
		},
	},
}

func Evaluate(fv engine.FeatureVector) []engine.AlertEvent {
	var alerts []engine.AlertEvent
	for _, r := range All {
		triggered, value := r.Check(fv)
		if triggered {
			alerts = append(alerts, engine.AlertEvent{
				Timestamp:  fv.Timestamp,
				PlayerID:   fv.PlayerID,
				Source:     "rule-engine",
				Rule:       r.Name,
				CheatType:  r.CheatType,
				Confidence: r.Confidence,
				Value:      value,
				Threshold:  r.Threshold,
			})
		}
	}
	return alerts
}
