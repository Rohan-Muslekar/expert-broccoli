package server

import (
	"math"
	"math/rand"
	"time"

	"cheat-detection/game-server/telemetry"
)

// Cheat type indices for cycling
const (
	cheatAimbot    = 0
	cheatSpeedHack = 1
	cheatWallHack  = 2
	cheatTriggerBot = 3
	cheatTypeCount  = 4
)

type BotAI struct {
	player *Player

	// Movement
	moveDir   Vec2
	moveTimer time.Duration

	// Aiming
	aimTarget *Player
	aimDelay  time.Duration
	aimReady  bool

	// Cheat cycling
	cheatsEnabled    bool
	cheatTimer       time.Duration
	cheatActiveTimer time.Duration
	cheatActive      bool
	currentCheat     int
}

// NewBotAI creates a bot AI controller for the given player.
func NewBotAI(player *Player, cheatsEnabled bool) *BotAI {
	return &BotAI{
		player:        player,
		moveDir:       randomDirection(),
		moveTimer:     randomDuration(1*time.Second, 3*time.Second),
		cheatsEnabled: cheatsEnabled,
		cheatTimer:    randomDuration(30*time.Second, 60*time.Second),
		currentCheat:  rand.Intn(cheatTypeCount),
	}
}

// Update runs all bot AI subsystems and sets the player's LatestInput.
func (b *BotAI) Update(dt time.Duration, others []*Player, walls []telemetry.Wall) {
	b.updateMovement(dt)
	b.updateAiming(dt, others, walls)
	b.updateShooting(others, walls)
	b.updateCheatCycle(dt)

	// Build input from bot state
	input := &telemetry.InputState{}

	// Set movement keys based on moveDir
	if b.moveDir.Y < -0.1 {
		input.Keys.W = true
	}
	if b.moveDir.Y > 0.1 {
		input.Keys.S = true
	}
	if b.moveDir.X < -0.1 {
		input.Keys.A = true
	}
	if b.moveDir.X > 0.1 {
		input.Keys.D = true
	}

	// Set mouse position based on aim
	aimDist := 200.0
	input.Mouse.X = b.player.X + math.Cos(b.player.AimAngle)*aimDist
	input.Mouse.Y = b.player.Y + math.Sin(b.player.AimAngle)*aimDist

	input.Shooting = b.player.Shooting

	input.Cheats = telemetry.CheatFlags{
		Aimbot:     b.player.Cheats.Aimbot,
		SpeedHack:  b.player.Cheats.SpeedHack,
		WallHack:   b.player.Cheats.WallHack,
		TriggerBot: b.player.Cheats.TriggerBot,
	}

	b.player.LatestInput = input
}

// updateMovement changes bot direction every 1-3 seconds.
func (b *BotAI) updateMovement(dt time.Duration) {
	b.moveTimer -= dt
	if b.moveTimer <= 0 {
		b.moveDir = randomDirection()
		b.moveTimer = randomDuration(1*time.Second, 3*time.Second)
	}
}

// updateAiming finds the nearest visible enemy and aims with reaction delay and inaccuracy.
func (b *BotAI) updateAiming(dt time.Duration, others []*Player, walls []telemetry.Wall) {
	// Find nearest visible enemy
	var nearest *Player
	bestDist := math.MaxFloat64

	for _, other := range others {
		if other.ID == b.player.ID || !other.Alive {
			continue
		}
		if !HasLineOfSight(walls, b.player.X, b.player.Y, other.X, other.Y) {
			continue
		}
		dist := Distance(b.player.X, b.player.Y, other.X, other.Y)
		if dist < bestDist {
			bestDist = dist
			nearest = other
		}
	}

	if nearest == nil {
		b.aimTarget = nil
		b.aimReady = false
		return
	}

	// New target acquired: apply reaction delay
	if b.aimTarget == nil || b.aimTarget.ID != nearest.ID {
		b.aimTarget = nearest
		b.aimDelay = randomDuration(150*time.Millisecond, 250*time.Millisecond)
		b.aimReady = false
	}

	b.aimDelay -= dt
	if b.aimDelay <= 0 {
		b.aimReady = true
	}

	if b.aimReady {
		// Compute ideal angle with inaccuracy offset (25-35% accuracy via +/-0.3 rad)
		idealAngle := AngleBetween(b.player.X, b.player.Y, nearest.X, nearest.Y)
		offset := (rand.Float64() - 0.5) * 0.6 // +/- 0.3 rad
		targetAngle := idealAngle + offset

		// Interpolate aim toward target (smooth tracking)
		diff := targetAngle - b.player.AimAngle
		for diff > math.Pi {
			diff -= 2 * math.Pi
		}
		for diff < -math.Pi {
			diff += 2 * math.Pi
		}
		b.player.AimAngle += diff * 0.3
	}
}

// updateShooting fires when the bot has a target aimed and visible.
func (b *BotAI) updateShooting(others []*Player, walls []telemetry.Wall) {
	b.player.Shooting = false

	if !b.aimReady || b.aimTarget == nil || !b.aimTarget.Alive {
		return
	}

	// Check LOS to current target
	if !HasLineOfSight(walls, b.player.X, b.player.Y, b.aimTarget.X, b.aimTarget.Y) {
		return
	}

	// Check if aim is close enough to target
	angle := AngleBetween(b.player.X, b.player.Y, b.aimTarget.X, b.aimTarget.Y)
	if AngleDiff(b.player.AimAngle, angle) < 0.4 {
		b.player.Shooting = true
	}
}

// updateCheatCycle cycles through cheat types when cheats are enabled.
// Active for 10-15s, cooldown 30-60s.
func (b *BotAI) updateCheatCycle(dt time.Duration) {
	if !b.cheatsEnabled {
		return
	}

	b.cheatTimer -= dt

	if b.cheatActive {
		b.cheatActiveTimer -= dt
		if b.cheatActiveTimer <= 0 {
			// Deactivate cheat, start cooldown
			b.cheatActive = false
			b.clearCheats()
			b.currentCheat = (b.currentCheat + 1) % cheatTypeCount
			b.cheatTimer = randomDuration(30*time.Second, 60*time.Second)
		}
	} else if b.cheatTimer <= 0 {
		// Activate next cheat
		b.cheatActive = true
		b.cheatActiveTimer = randomDuration(10*time.Second, 15*time.Second)
		b.applyCurrentCheat()
	}
}

func (b *BotAI) applyCurrentCheat() {
	b.clearCheats()
	switch b.currentCheat {
	case cheatAimbot:
		b.player.Cheats.Aimbot = true
	case cheatSpeedHack:
		b.player.Cheats.SpeedHack = true
	case cheatWallHack:
		b.player.Cheats.WallHack = true
	case cheatTriggerBot:
		b.player.Cheats.TriggerBot = true
	}
}

func (b *BotAI) clearCheats() {
	b.player.Cheats = CheatState{}
}

func randomDirection() Vec2 {
	angle := rand.Float64() * 2 * math.Pi
	return Vec2{
		X: math.Cos(angle),
		Y: math.Sin(angle),
	}
}

func randomDuration(min, max time.Duration) time.Duration {
	spread := max - min
	return min + time.Duration(rand.Int63n(int64(spread)))
}
