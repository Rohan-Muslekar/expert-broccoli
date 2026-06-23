package feature

import (
	"math"
	"testing"

	"cheat-detection/game-server/telemetry"
)

func makeTelemetry(velX, velY, aimDelta float64) telemetry.PlayerTelemetry {
	return telemetry.PlayerTelemetry{
		PlayerID: "test-player",
		VelX:     velX,
		VelY:     velY,
		AimDelta: aimDelta,
	}
}

func TestNewPlayerProcessor(t *testing.T) {
	processor := NewPlayerProcessor("player-1")
	if processor.PlayerID != "player-1" {
		t.Errorf("expected player-1, got %s", processor.PlayerID)
	}
	if len(processor.buffer) != 0 {
		t.Errorf("expected empty buffer, got %d", len(processor.buffer))
	}
}

func TestProcessComputesSpeedFromVelocity(t *testing.T) {
	processor := NewPlayerProcessor("test")
	event := makeTelemetry(3.0, 4.0, 0)
	features := processor.Process(event)

	expectedSpeed := 5.0
	if math.Abs(features.SpeedMax1s-expectedSpeed) > 0.001 {
		t.Errorf("expected speed_max_1s=%f, got %f", expectedSpeed, features.SpeedMax1s)
	}
	if math.Abs(features.SpeedMean1s-expectedSpeed) > 0.001 {
		t.Errorf("expected speed_mean_1s=%f, got %f", expectedSpeed, features.SpeedMean1s)
	}
}

func TestProcessComputesAimDelta(t *testing.T) {
	processor := NewPlayerProcessor("test")

	for i := 0; i < 10; i++ {
		processor.Process(makeTelemetry(0, 0, 0.5))
	}
	features := processor.Process(makeTelemetry(0, 0, 2.0))

	if features.AimDeltaMax1s != 2.0 {
		t.Errorf("expected aim_delta_max_1s=2.0, got %f", features.AimDeltaMax1s)
	}
}

func TestWindowCapsAt30Seconds(t *testing.T) {
	processor := NewPlayerProcessor("test")

	for i := 0; i < Window30s+100; i++ {
		processor.Process(makeTelemetry(1, 0, 0.1))
	}

	if len(processor.buffer) != Window30s {
		t.Errorf("buffer should cap at %d, got %d", Window30s, len(processor.buffer))
	}
}

func TestHitRateComputation(t *testing.T) {
	processor := NewPlayerProcessor("test")

	for i := 0; i < 10; i++ {
		event := telemetry.PlayerTelemetry{
			PlayerID:   "test",
			IsShooting: true,
			HitTarget:  i < 7,
		}
		processor.Process(event)
	}

	features := processor.Process(telemetry.PlayerTelemetry{
		PlayerID:   "test",
		IsShooting: true,
		HitTarget:  false,
	})

	expectedRate := 7.0 / 11.0
	if math.Abs(features.HitRate5s-expectedRate) > 0.01 {
		t.Errorf("expected hit_rate_5s ~%f, got %f", expectedRate, features.HitRate5s)
	}
}

func TestShotsFiredCount(t *testing.T) {
	processor := NewPlayerProcessor("test")

	for i := 0; i < 5; i++ {
		processor.Process(telemetry.PlayerTelemetry{
			PlayerID:   "test",
			IsShooting: true,
		})
	}
	for i := 0; i < 3; i++ {
		processor.Process(telemetry.PlayerTelemetry{
			PlayerID:   "test",
			IsShooting: false,
		})
	}

	features := processor.Process(telemetry.PlayerTelemetry{PlayerID: "test"})

	if features.ShotsFired5s != 5 {
		t.Errorf("expected shots_fired_5s=5, got %d", features.ShotsFired5s)
	}
}

func TestDirectionChanges(t *testing.T) {
	processor := NewPlayerProcessor("test")

	processor.Process(makeTelemetry(1, 0, 0))
	processor.Process(makeTelemetry(-1, 0, 0))
	processor.Process(makeTelemetry(0, 1, 0))

	features := processor.Process(makeTelemetry(0, -1, 0))

	if features.DirectionChangeCount5s < 2 {
		t.Errorf("expected at least 2 direction changes, got %d", features.DirectionChangeCount5s)
	}
}

func TestEmptyProcessorReturnsZeros(t *testing.T) {
	processor := NewPlayerProcessor("test")
	features := processor.Process(telemetry.PlayerTelemetry{PlayerID: "test"})

	if features.HitRate5s != 0 {
		t.Errorf("expected hit_rate_5s=0 with no shots, got %f", features.HitRate5s)
	}
	if features.AimDeltaMean1s != 0 {
		t.Errorf("expected aim_delta_mean_1s=0, got %f", features.AimDeltaMean1s)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	corr := pearsonCorrelation(x, y)
	if math.Abs(corr-1.0) > 0.001 {
		t.Errorf("expected perfect correlation ~1.0, got %f", corr)
	}

	yInverse := []float64{10, 8, 6, 4, 2}
	corrInverse := pearsonCorrelation(x, yInverse)
	if math.Abs(corrInverse-(-1.0)) > 0.001 {
		t.Errorf("expected perfect inverse ~-1.0, got %f", corrInverse)
	}

	corrEmpty := pearsonCorrelation([]float64{}, []float64{})
	if corrEmpty != 0 {
		t.Errorf("expected 0 for empty, got %f", corrEmpty)
	}
}
