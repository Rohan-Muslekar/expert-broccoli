package feature

import (
	"testing"
)

func cleanFeatures() FeatureVector {
	fv := FeatureVector{
		SpeedMax1s:         3.0,
		AimDeltaMax1s:      0.3,
		ShotsFired5s:       15,
		HitRate5s:          0.40,
		AimLockRatio5s:     0.20,
		PrefireRatio5s:     0.10,
		ReactionTimeMean5s: 10.0,
	}
	fv.EnemiesVisible = 2
	return fv
}

func TestSpeedCapTriggersOnSpeedhack(t *testing.T) {
	features := cleanFeatures()
	features.SpeedMax1s = 12.5

	alerts := Evaluate(features)

	found := false
	for _, alert := range alerts {
		if alert.Rule == "speed_cap" {
			found = true
			if alert.CheatType != "speedhack" {
				t.Errorf("expected cheat_type=speedhack, got %s", alert.CheatType)
			}
			if alert.Value != 12.5 {
				t.Errorf("expected value=12.5, got %f", alert.Value)
			}
		}
	}
	if !found {
		t.Error("speed_cap rule did not trigger for speed 12.5")
	}
}

func TestSpeedCapDoesNotTriggerOnNormal(t *testing.T) {
	features := cleanFeatures()
	features.SpeedMax1s = 5.0

	alerts := Evaluate(features)
	for _, alert := range alerts {
		if alert.Rule == "speed_cap" {
			t.Error("speed_cap should not trigger for speed 5.0")
		}
	}
}

func TestAimSnapTriggersOnAimbot(t *testing.T) {
	features := cleanFeatures()
	features.AimDeltaMax1s = 4.0
	features.ShotsFired5s = 10

	alerts := Evaluate(features)

	found := false
	for _, alert := range alerts {
		if alert.Rule == "aim_snap" {
			found = true
			if alert.CheatType != "aimbot" {
				t.Errorf("expected cheat_type=aimbot, got %s", alert.CheatType)
			}
		}
	}
	if !found {
		t.Error("aim_snap rule did not trigger for aim delta 4.0")
	}
}

func TestAimSnapRequiresShotsThreshold(t *testing.T) {
	features := cleanFeatures()
	features.AimDeltaMax1s = 4.0
	features.ShotsFired5s = 3

	alerts := Evaluate(features)
	for _, alert := range alerts {
		if alert.Rule == "aim_snap" {
			t.Error("aim_snap should not trigger with only 3 shots fired")
		}
	}
}

func TestInhumanAccuracyTriggers(t *testing.T) {
	features := cleanFeatures()
	features.HitRate5s = 0.95
	features.ShotsFired5s = 40

	alerts := Evaluate(features)
	found := false
	for _, alert := range alerts {
		if alert.Rule == "inhuman_accuracy" {
			found = true
		}
	}
	if !found {
		t.Error("inhuman_accuracy did not trigger at 95% hit rate with 40 shots")
	}
}

func TestPrefireTriggersOnWallhack(t *testing.T) {
	features := cleanFeatures()
	features.PrefireRatio5s = 0.75
	features.ShotsFired5s = 25
	features.HitRate5s = 0.5

	alerts := Evaluate(features)
	found := false
	for _, alert := range alerts {
		if alert.Rule == "prefire" {
			found = true
			if alert.CheatType != "wallhack" {
				t.Errorf("expected cheat_type=wallhack, got %s", alert.CheatType)
			}
		}
	}
	if !found {
		t.Error("prefire rule did not trigger for prefire ratio 0.75")
	}
}

func TestTriggerbotTriggersOnLowReaction(t *testing.T) {
	features := cleanFeatures()
	features.ReactionTimeMean5s = 1.5
	features.HitRate5s = 0.6
	features.ShotsFired5s = 15

	alerts := Evaluate(features)
	found := false
	for _, alert := range alerts {
		if alert.Rule == "triggerbot_reaction" {
			found = true
			if alert.CheatType != "triggerbot" {
				t.Errorf("expected cheat_type=triggerbot, got %s", alert.CheatType)
			}
		}
	}
	if !found {
		t.Error("triggerbot_reaction did not trigger for reaction time 1.5")
	}
}

func TestCleanPlayerTriggersNoAlerts(t *testing.T) {
	features := cleanFeatures()
	alerts := Evaluate(features)
	if len(alerts) != 0 {
		t.Errorf("clean player triggered %d alerts: %v", len(alerts), alerts)
	}
}

func TestAlertEventFieldsPopulated(t *testing.T) {
	features := cleanFeatures()
	features.SpeedMax1s = 10.0
	features.PlayerTelemetry.PlayerID = "player-42"
	features.PlayerTelemetry.Timestamp = 1700000000

	alerts := Evaluate(features)
	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}
	alert := alerts[0]
	if alert.PlayerID != "player-42" {
		t.Errorf("expected player_id=player-42, got %s", alert.PlayerID)
	}
	if alert.Source != "rule-engine" {
		t.Errorf("expected source=rule-engine, got %s", alert.Source)
	}
	if alert.Timestamp != 1700000000 {
		t.Errorf("expected timestamp=1700000000, got %d", alert.Timestamp)
	}
	if alert.Threshold <= 0 {
		t.Errorf("expected positive threshold, got %f", alert.Threshold)
	}
}

func TestAimLockTriggers(t *testing.T) {
	features := cleanFeatures()
	features.AimLockRatio5s = 0.95
	features.PlayerTelemetry.EnemiesVisible = 2
	features.ShotsFired5s = 20

	alerts := Evaluate(features)
	found := false
	for _, alert := range alerts {
		if alert.Rule == "aim_lock" {
			found = true
		}
	}
	if !found {
		t.Error("aim_lock did not trigger at 95% lock ratio")
	}
}

func TestMultipleCheatsFireMultipleAlerts(t *testing.T) {
	features := cleanFeatures()
	features.SpeedMax1s = 12.0
	features.AimDeltaMax1s = 5.0
	features.ShotsFired5s = 35
	features.HitRate5s = 0.90

	alerts := Evaluate(features)
	if len(alerts) < 2 {
		t.Errorf("expected at least 2 alerts for combined cheats, got %d", len(alerts))
	}
}
