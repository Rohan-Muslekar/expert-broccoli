package engine

import (
	"math"
)

const (
	Window1s  = 60
	Window5s  = 300
	Window30s = 1800
)

type PlayerProcessor struct {
	PlayerID string
	buffer   []PlayerTelemetry
}

func NewPlayerProcessor(playerID string) *PlayerProcessor {
	return &PlayerProcessor{
		PlayerID: playerID,
		buffer:   make([]PlayerTelemetry, 0, Window30s),
	}
}

func (p *PlayerProcessor) Process(ev PlayerTelemetry) FeatureVector {
	if len(p.buffer) >= Window30s {
		p.buffer = p.buffer[1:]
	}
	p.buffer = append(p.buffer, ev)

	w1 := p.window(Window1s)
	w5 := p.window(Window5s)
	w30 := p.window(Window30s)

	fv := FeatureVector{
		Timestamp:           ev.Timestamp,
		PlayerID:            ev.PlayerID,
		Tick:                ev.Tick,
		PosX:                ev.PosX,
		PosY:                ev.PosY,
		VelX:                ev.VelX,
		VelY:                ev.VelY,
		AimAngle:            ev.AimAngle,
		AimDelta:            ev.AimDelta,
		IsShooting:          ev.IsShooting,
		HitTarget:           ev.HitTarget,
		Health:              ev.Health,
		IsAlive:             ev.IsAlive,
		NearestEnemyDist:    ev.NearestEnemyDist,
		NearestEnemyAngle:   ev.NearestEnemyAngle,
		NearestEnemyVisible: ev.NearestEnemyVisible,
		AimToEnemyOffset:    ev.AimToEnemyOffset,
		TimeSinceVisible:    ev.TimeSinceVisible,
		EnemiesVisible:      ev.EnemiesVisible,
		CheatLabel:          ev.CheatLabel,
	}

	// Aim dynamics
	fv.AimDeltaMean1s = meanFloat(w1, func(e PlayerTelemetry) float64 { return e.AimDelta })
	fv.AimDeltaMean5s = meanFloat(w5, func(e PlayerTelemetry) float64 { return e.AimDelta })
	fv.AimDeltaMax1s = maxFloat(w1, func(e PlayerTelemetry) float64 { return e.AimDelta })
	fv.AimSnapCount5s = countWhere(w5, func(e PlayerTelemetry) bool { return e.AimDelta > 0.5 })
	fv.AimToEnemyOffsetMean5s = meanFloat(w5, func(e PlayerTelemetry) float64 { return e.AimToEnemyOffset })

	// Combat stats
	fv.HitRate5s = hitRate(w5)
	fv.HitRate30s = hitRate(w30)
	fv.ShotsFired5s = countWhere(w5, func(e PlayerTelemetry) bool { return e.IsShooting })
	fv.KillsPer30s = countKills(w30)
	fv.TimeToKillMean30s = meanKillSequenceLength(w30)

	// Movement profile
	fv.SpeedMean1s = meanFloat(w1, func(e PlayerTelemetry) float64 { return speed(e) })
	fv.SpeedMean5s = meanFloat(w5, func(e PlayerTelemetry) float64 { return speed(e) })
	fv.SpeedMax1s = maxFloat(w1, func(e PlayerTelemetry) float64 { return speed(e) })
	fv.DirectionChangeCount5s = directionChanges(w5)

	// Spatial correlation
	fv.AimLockRatio5s = aimLockRatio(w5)
	fv.PrefireRatio5s = prefireRatio(w5)
	fv.ReactionTimeMean5s = reactionTimeMean(w5)
	fv.EnemyTrackingScore5s = enemyTrackingScore(w5)

	return fv
}

func (p *PlayerProcessor) window(size int) []PlayerTelemetry {
	if len(p.buffer) <= size {
		return p.buffer
	}
	return p.buffer[len(p.buffer)-size:]
}

func speed(e PlayerTelemetry) float64 {
	return math.Sqrt(e.VelX*e.VelX + e.VelY*e.VelY)
}

func meanFloat(data []PlayerTelemetry, f func(PlayerTelemetry) float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, e := range data {
		sum += f(e)
	}
	return sum / float64(len(data))
}

func maxFloat(data []PlayerTelemetry, f func(PlayerTelemetry) float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := f(data[0])
	for _, e := range data[1:] {
		v := f(e)
		if v > m {
			m = v
		}
	}
	return m
}

func countWhere(data []PlayerTelemetry, pred func(PlayerTelemetry) bool) int {
	n := 0
	for _, e := range data {
		if pred(e) {
			n++
		}
	}
	return n
}

func hitRate(data []PlayerTelemetry) float64 {
	shots := 0
	hits := 0
	for _, e := range data {
		if e.IsShooting {
			shots++
			if e.HitTarget {
				hits++
			}
		}
	}
	if shots == 0 {
		return 0
	}
	return float64(hits) / float64(shots)
}

func countKills(data []PlayerTelemetry) int {
	kills := 0
	inHitSequence := false
	for _, e := range data {
		if e.HitTarget && !inHitSequence {
			inHitSequence = true
		} else if !e.HitTarget && inHitSequence {
			kills++
			inHitSequence = false
		}
	}
	if inHitSequence {
		kills++
	}
	return kills
}

func meanKillSequenceLength(data []PlayerTelemetry) float64 {
	var lengths []int
	current := 0
	for _, e := range data {
		if e.HitTarget && e.IsShooting {
			current++
		} else if current > 0 {
			lengths = append(lengths, current)
			current = 0
		}
	}
	if current > 0 {
		lengths = append(lengths, current)
	}
	if len(lengths) == 0 {
		return 0
	}
	sum := 0
	for _, l := range lengths {
		sum += l
	}
	return float64(sum) / float64(len(lengths))
}

func directionChanges(data []PlayerTelemetry) int {
	if len(data) < 2 {
		return 0
	}
	count := 0
	for i := 1; i < len(data); i++ {
		prev := math.Atan2(data[i-1].VelY, data[i-1].VelX)
		curr := math.Atan2(data[i].VelY, data[i].VelX)
		prevSpeed := speed(data[i-1])
		currSpeed := speed(data[i])
		if prevSpeed < 0.1 || currSpeed < 0.1 {
			continue
		}
		diff := math.Abs(curr - prev)
		if diff > math.Pi {
			diff = 2*math.Pi - diff
		}
		if diff > math.Pi/2 {
			count++
		}
	}
	return count
}

func aimLockRatio(data []PlayerTelemetry) float64 {
	visibleTicks := 0
	lockedTicks := 0
	for _, e := range data {
		if e.NearestEnemyVisible {
			visibleTicks++
			if e.AimToEnemyOffset < 0.1 {
				lockedTicks++
			}
		}
	}
	if visibleTicks == 0 {
		return 0
	}
	return float64(lockedTicks) / float64(visibleTicks)
}

func prefireRatio(data []PlayerTelemetry) float64 {
	shootingTicks := 0
	prefireTicks := 0
	for _, e := range data {
		if e.IsShooting {
			shootingTicks++
			if !e.NearestEnemyVisible {
				prefireTicks++
			}
		}
	}
	if shootingTicks == 0 {
		return 0
	}
	return float64(prefireTicks) / float64(shootingTicks)
}

func reactionTimeMean(data []PlayerTelemetry) float64 {
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
	sum := 0
	for _, t := range reactionTimes {
		sum += t
	}
	return float64(sum) / float64(len(reactionTimes))
}

func enemyTrackingScore(data []PlayerTelemetry) float64 {
	if len(data) < 3 {
		return 0
	}
	var dAim, dEnemy []float64
	for i := 1; i < len(data); i++ {
		if !data[i].NearestEnemyVisible || !data[i-1].NearestEnemyVisible {
			continue
		}
		dAim = append(dAim, data[i].AimAngle-data[i-1].AimAngle)
		dEnemy = append(dEnemy, data[i].NearestEnemyAngle-data[i-1].NearestEnemyAngle)
	}
	if len(dAim) < 2 {
		return 0
	}
	return pearson(dAim, dEnemy)
}

func pearson(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) {
		return 0
	}
	mx, my := 0.0, 0.0
	for i := 0; i < n; i++ {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)

	num, dx, dy := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		a := x[i] - mx
		b := y[i] - my
		num += a * b
		dx += a * a
		dy += b * b
	}
	denom := math.Sqrt(dx * dy)
	if denom < 1e-12 {
		return 0
	}
	return num / denom
}
