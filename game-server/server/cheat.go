package server

import (
	"math"

	"cheat-detection/game-server/telemetry"
)

// ApplyAimbot overrides the player's aim to snap to the nearest VISIBLE enemy.
func ApplyAimbot(p *Player, others []*Player, walls []telemetry.Wall) {
	if !p.Cheats.Aimbot {
		return
	}

	var nearest *Player
	bestDist := math.MaxFloat64

	for _, other := range others {
		if other.ID == p.ID || !other.Alive {
			continue
		}
		if !HasLineOfSight(walls, p.X, p.Y, other.X, other.Y) {
			continue
		}
		dist := Distance(p.X, p.Y, other.X, other.Y)
		if dist < bestDist {
			bestDist = dist
			nearest = other
		}
	}

	if nearest != nil {
		p.AimAngle = AngleBetween(p.X, p.Y, nearest.X, nearest.Y)
	}
}

// ApplySpeedHack multiplies velocity by 2.5.
func ApplySpeedHack(p *Player) {
	if !p.Cheats.SpeedHack {
		return
	}
	p.VX *= 2.5
	p.VY *= 2.5
}

// ApplyWallHack slowly interpolates aim toward the nearest enemy (including
// through walls) at 10% per tick.
func ApplyWallHack(p *Player, others []*Player) {
	if !p.Cheats.WallHack {
		return
	}

	var nearest *Player
	bestDist := math.MaxFloat64

	for _, other := range others {
		if other.ID == p.ID || !other.Alive {
			continue
		}
		dist := Distance(p.X, p.Y, other.X, other.Y)
		if dist < bestDist {
			bestDist = dist
			nearest = other
		}
	}

	if nearest == nil {
		return
	}

	targetAngle := AngleBetween(p.X, p.Y, nearest.X, nearest.Y)

	// Interpolate 10% toward target each tick
	diff := targetAngle - p.AimAngle
	// Normalize to [-pi, pi]
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	p.AimAngle += diff * 0.1
}

// ApplyTriggerBot auto-fires if aim is within 0.1 rad of any visible enemy.
func ApplyTriggerBot(p *Player, others []*Player, walls []telemetry.Wall) {
	if !p.Cheats.TriggerBot {
		return
	}

	for _, other := range others {
		if other.ID == p.ID || !other.Alive {
			continue
		}
		if !HasLineOfSight(walls, p.X, p.Y, other.X, other.Y) {
			continue
		}
		angle := AngleBetween(p.X, p.Y, other.X, other.Y)
		if AngleDiff(p.AimAngle, angle) < 0.1 {
			p.Shooting = true
			return
		}
	}
}

// ApplyCheats applies all cheat types in order: speed, aim, wall, trigger.
func ApplyCheats(p *Player, others []*Player, walls []telemetry.Wall) {
	ApplySpeedHack(p)
	ApplyAimbot(p, others, walls)
	ApplyWallHack(p, others)
	ApplyTriggerBot(p, others, walls)
}
