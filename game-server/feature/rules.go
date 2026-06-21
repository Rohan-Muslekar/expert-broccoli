package feature

type AlertEvent struct {
	Timestamp  int64   `json:"ts"`
	PlayerID   string  `json:"player_id"`
	Source     string  `json:"source"`
	Rule       string  `json:"rule"`
	CheatType  string  `json:"cheat_type"`
	Confidence float64 `json:"confidence"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
}

type Rule struct {
	Name       string
	CheatType  string
	Confidence float64
	Threshold  float64
	Check      func(features FeatureVector) (triggered bool, value float64)
}

var allRules = []Rule{
	{
		Name:       "speed_cap",
		CheatType:  "speedhack",
		Confidence: 0.95,
		Threshold:  7.0,
		Check: func(features FeatureVector) (bool, float64) {
			return features.SpeedMax1s > 7.0, features.SpeedMax1s
		},
	},
	{
		Name:       "aim_snap",
		CheatType:  "aimbot",
		Confidence: 0.95,
		Threshold:  2.0,
		Check: func(features FeatureVector) (bool, float64) {
			return features.AimDeltaMax1s > 2.0, features.AimDeltaMax1s
		},
	},
	{
		Name:       "inhuman_accuracy",
		CheatType:  "aimbot",
		Confidence: 0.90,
		Threshold:  0.85,
		Check: func(features FeatureVector) (bool, float64) {
			return features.HitRate5s > 0.85 && features.ShotsFired5s > 30, features.HitRate5s
		},
	},
	{
		Name:       "aim_lock",
		CheatType:  "aimbot",
		Confidence: 0.90,
		Threshold:  0.90,
		Check: func(features FeatureVector) (bool, float64) {
			return features.AimLockRatio5s > 0.90 && features.EnemiesVisible > 0, features.AimLockRatio5s
		},
	},
	{
		Name:       "prefire",
		CheatType:  "wallhack",
		Confidence: 0.90,
		Threshold:  0.60,
		Check: func(features FeatureVector) (bool, float64) {
			return features.PrefireRatio5s > 0.60 && features.ShotsFired5s > 20, features.PrefireRatio5s
		},
	},
	{
		Name:       "triggerbot_reaction",
		CheatType:  "triggerbot",
		Confidence: 0.90,
		Threshold:  3.0,
		Check: func(features FeatureVector) (bool, float64) {
			return features.ReactionTimeMean5s > 0 && features.ReactionTimeMean5s < 3.0 && features.ShotsFired5s > 10, features.ReactionTimeMean5s
		},
	},
}

func Evaluate(features FeatureVector) []AlertEvent {
	var alerts []AlertEvent
	for _, rule := range allRules {
		triggered, value := rule.Check(features)
		if triggered {
			alerts = append(alerts, AlertEvent{
				Timestamp:  features.Timestamp,
				PlayerID:   features.PlayerID,
				Source:     "rule-engine",
				Rule:       rule.Name,
				CheatType:  rule.CheatType,
				Confidence: rule.Confidence,
				Value:      value,
				Threshold:  rule.Threshold,
			})
		}
	}
	return alerts
}
