package feature

import (
	"math"

	"cheat-detection/game-server/telemetry"
)

const (
	Window1s  = 60
	Window5s  = 300
	Window30s = 1800
)

type FeatureVector struct {
	telemetry.PlayerTelemetry

	AimDeltaMean1s         float64 `json:"aim_delta_mean_1s"`
	AimDeltaMean5s         float64 `json:"aim_delta_mean_5s"`
	AimDeltaMax1s          float64 `json:"aim_delta_max_1s"`
	AimSnapCount5s         int     `json:"aim_snap_count_5s"`
	AimToEnemyOffsetMean5s float64 `json:"aim_to_enemy_offset_mean_5s"`

	HitRate5s         float64 `json:"hit_rate_5s"`
	HitRate30s        float64 `json:"hit_rate_30s"`
	ShotsFired5s      int     `json:"shots_fired_5s"`
	KillsPer30s       int     `json:"kills_per_30s"`
	TimeToKillMean30s float64 `json:"time_to_kill_mean_30s"`

	SpeedMean1s            float64 `json:"speed_mean_1s"`
	SpeedMean5s            float64 `json:"speed_mean_5s"`
	SpeedMax1s             float64 `json:"speed_max_1s"`
	DirectionChangeCount5s int     `json:"direction_change_count_5s"`

	AimLockRatio5s       float64 `json:"aim_lock_ratio_5s"`
	PrefireRatio5s       float64 `json:"prefire_ratio_5s"`
	ReactionTimeMean5s   float64 `json:"reaction_time_mean_5s"`
	EnemyTrackingScore5s float64 `json:"enemy_tracking_score_5s"`
}

type PlayerProcessor struct {
	PlayerID string
	buffer   []telemetry.PlayerTelemetry
}

func NewPlayerProcessor(playerID string) *PlayerProcessor {
	return &PlayerProcessor{
		PlayerID: playerID,
		buffer:   make([]telemetry.PlayerTelemetry, 0, Window30s),
	}
}

func (p *PlayerProcessor) Process(event telemetry.PlayerTelemetry) FeatureVector {
	if len(p.buffer) >= Window30s {
		p.buffer = p.buffer[1:]
	}
	p.buffer = append(p.buffer, event)

	window1s := p.window(Window1s)
	window5s := p.window(Window5s)
	window30s := p.window(Window30s)

	features := FeatureVector{PlayerTelemetry: event}

	features.AimDeltaMean1s = meanFloat(window1s, func(entry telemetry.PlayerTelemetry) float64 { return entry.AimDelta })
	features.AimDeltaMean5s = meanFloat(window5s, func(entry telemetry.PlayerTelemetry) float64 { return entry.AimDelta })
	features.AimDeltaMax1s = maxFloat(window1s, func(entry telemetry.PlayerTelemetry) float64 { return entry.AimDelta })
	features.AimSnapCount5s = countWhere(window5s, func(entry telemetry.PlayerTelemetry) bool { return entry.AimDelta > 0.5 })
	features.AimToEnemyOffsetMean5s = meanFloat(window5s, func(entry telemetry.PlayerTelemetry) float64 { return entry.AimToEnemyOffset })

	features.HitRate5s = hitRate(window5s)
	features.HitRate30s = hitRate(window30s)
	features.ShotsFired5s = countWhere(window5s, func(entry telemetry.PlayerTelemetry) bool { return entry.IsShooting })
	features.KillsPer30s = countKills(window30s)
	features.TimeToKillMean30s = meanKillSequenceLength(window30s)

	features.SpeedMean1s = meanFloat(window1s, func(entry telemetry.PlayerTelemetry) float64 { return playerSpeed(entry) })
	features.SpeedMean5s = meanFloat(window5s, func(entry telemetry.PlayerTelemetry) float64 { return playerSpeed(entry) })
	features.SpeedMax1s = maxFloat(window1s, func(entry telemetry.PlayerTelemetry) float64 { return playerSpeed(entry) })
	features.DirectionChangeCount5s = directionChanges(window5s)

	features.AimLockRatio5s = aimLockRatio(window5s)
	features.PrefireRatio5s = prefireRatio(window5s)
	features.ReactionTimeMean5s = reactionTimeMean(window5s)
	features.EnemyTrackingScore5s = enemyTrackingScore(window5s)

	return features
}

func (p *PlayerProcessor) window(size int) []telemetry.PlayerTelemetry {
	if len(p.buffer) <= size {
		return p.buffer
	}
	return p.buffer[len(p.buffer)-size:]
}

func playerSpeed(entry telemetry.PlayerTelemetry) float64 {
	return math.Sqrt(entry.VelX*entry.VelX + entry.VelY*entry.VelY)
}

func meanFloat(data []telemetry.PlayerTelemetry, extract func(telemetry.PlayerTelemetry) float64) float64 {
	if len(data) == 0 {
		return 0
	}
	total := 0.0
	for _, entry := range data {
		total += extract(entry)
	}
	return total / float64(len(data))
}

func maxFloat(data []telemetry.PlayerTelemetry, extract func(telemetry.PlayerTelemetry) float64) float64 {
	if len(data) == 0 {
		return 0
	}
	maxVal := extract(data[0])
	for _, entry := range data[1:] {
		val := extract(entry)
		if val > maxVal {
			maxVal = val
		}
	}
	return maxVal
}

func countWhere(data []telemetry.PlayerTelemetry, predicate func(telemetry.PlayerTelemetry) bool) int {
	count := 0
	for _, entry := range data {
		if predicate(entry) {
			count++
		}
	}
	return count
}

func hitRate(data []telemetry.PlayerTelemetry) float64 {
	shotCount := 0
	hitCount := 0
	for _, entry := range data {
		if entry.IsShooting {
			shotCount++
			if entry.HitTarget {
				hitCount++
			}
		}
	}
	if shotCount == 0 {
		return 0
	}
	return float64(hitCount) / float64(shotCount)
}

func countKills(data []telemetry.PlayerTelemetry) int {
	killCount := 0
	inHitSequence := false
	for _, entry := range data {
		if entry.HitTarget && !inHitSequence {
			inHitSequence = true
		} else if !entry.HitTarget && inHitSequence {
			killCount++
			inHitSequence = false
		}
	}
	if inHitSequence {
		killCount++
	}
	return killCount
}

func meanKillSequenceLength(data []telemetry.PlayerTelemetry) float64 {
	var sequenceLengths []int
	currentLength := 0
	for _, entry := range data {
		if entry.HitTarget && entry.IsShooting {
			currentLength++
		} else if currentLength > 0 {
			sequenceLengths = append(sequenceLengths, currentLength)
			currentLength = 0
		}
	}
	if currentLength > 0 {
		sequenceLengths = append(sequenceLengths, currentLength)
	}
	if len(sequenceLengths) == 0 {
		return 0
	}
	total := 0
	for _, length := range sequenceLengths {
		total += length
	}
	return float64(total) / float64(len(sequenceLengths))
}

func directionChanges(data []telemetry.PlayerTelemetry) int {
	if len(data) < 2 {
		return 0
	}
	changeCount := 0
	for i := 1; i < len(data); i++ {
		prevAngle := math.Atan2(data[i-1].VelY, data[i-1].VelX)
		currAngle := math.Atan2(data[i].VelY, data[i].VelX)
		prevSpeed := playerSpeed(data[i-1])
		currSpeed := playerSpeed(data[i])
		if prevSpeed < 0.1 || currSpeed < 0.1 {
			continue
		}
		angleDiff := math.Abs(currAngle - prevAngle)
		if angleDiff > math.Pi {
			angleDiff = 2*math.Pi - angleDiff
		}
		if angleDiff > math.Pi/2 {
			changeCount++
		}
	}
	return changeCount
}

func aimLockRatio(data []telemetry.PlayerTelemetry) float64 {
	visibleTicks := 0
	lockedTicks := 0
	for _, entry := range data {
		if entry.NearestEnemyVisible {
			visibleTicks++
			if entry.AimToEnemyOffset < 0.1 {
				lockedTicks++
			}
		}
	}
	if visibleTicks == 0 {
		return 0
	}
	return float64(lockedTicks) / float64(visibleTicks)
}

func prefireRatio(data []telemetry.PlayerTelemetry) float64 {
	shootingTicks := 0
	prefireTicks := 0
	for _, entry := range data {
		if entry.IsShooting {
			shootingTicks++
			if !entry.NearestEnemyVisible {
				prefireTicks++
			}
		}
	}
	if shootingTicks == 0 {
		return 0
	}
	return float64(prefireTicks) / float64(shootingTicks)
}

func reactionTimeMean(data []telemetry.PlayerTelemetry) float64 {
	if len(data) < 2 {
		return 0
	}
	var reactionTimes []int
	ticksSinceVisible := -1

	for i := 0; i < len(data); i++ {
		if i > 0 && data[i].NearestEnemyVisible && !data[i-1].NearestEnemyVisible {
			ticksSinceVisible = 0
		}
		if ticksSinceVisible >= 0 {
			ticksSinceVisible++
			if data[i].IsShooting {
				reactionTimes = append(reactionTimes, ticksSinceVisible)
				ticksSinceVisible = -1
			}
		}
	}
	if len(reactionTimes) == 0 {
		return 0
	}
	total := 0
	for _, reactionTime := range reactionTimes {
		total += reactionTime
	}
	return float64(total) / float64(len(reactionTimes))
}

func enemyTrackingScore(data []telemetry.PlayerTelemetry) float64 {
	if len(data) < 3 {
		return 0
	}
	var aimDeltas, enemyAngleDeltas []float64
	for i := 1; i < len(data); i++ {
		if !data[i].NearestEnemyVisible || !data[i-1].NearestEnemyVisible {
			continue
		}
		aimDeltas = append(aimDeltas, data[i].AimAngle-data[i-1].AimAngle)
		enemyAngleDeltas = append(enemyAngleDeltas, data[i].NearestEnemyAngle-data[i-1].NearestEnemyAngle)
	}
	if len(aimDeltas) < 2 {
		return 0
	}
	return pearsonCorrelation(aimDeltas, enemyAngleDeltas)
}

func pearsonCorrelation(x, y []float64) float64 {
	count := len(x)
	if count == 0 || count != len(y) {
		return 0
	}
	meanX, meanY := 0.0, 0.0
	for i := 0; i < count; i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(count)
	meanY /= float64(count)

	numerator, sumSquaredDeviationX, sumSquaredDeviationY := 0.0, 0.0, 0.0
	for i := 0; i < count; i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		sumSquaredDeviationX += diffX * diffX
		sumSquaredDeviationY += diffY * diffY
	}
	denominator := math.Sqrt(sumSquaredDeviationX * sumSquaredDeviationY)
	if denominator < 1e-12 {
		return 0
	}
	return numerator / denominator
}
