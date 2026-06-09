package server

import (
	"fmt"
	"math"
	"strings"
	"time"

	"cheat-detection/game-server/telemetry"
)

const (
	PlayerRadius    = 15
	MaxSpeed        = 5.0
	MoveAccel       = 1.5
	DamagePerHit    = 20
	MaxHP           = 100
	ProjectileSpeed = 15.0
)

type CheatState struct {
	Aimbot     bool
	SpeedHack  bool
	WallHack   bool
	TriggerBot bool
}

type Player struct {
	ID       string
	X, Y     float64
	VX, VY   float64
	AimAngle float64
	PrevAim  float64
	HP       int
	Alive    bool
	Shooting bool
	IsBot    bool
	IsHuman  bool

	RespawnTimer time.Duration
	RespawnDelay time.Duration

	Cheats CheatState

	// Spatial context
	NearestEnemyDist    float64
	NearestEnemyAngle   float64
	NearestEnemyVisible bool
	AimToEnemyOffset    float64
	TimeSinceVisible    float64
	EnemiesVisible      int

	// Tracking fields for visibility timing
	lastVisibleTime time.Time
	wasVisible      bool

	LatestInput *telemetry.InputState
}

// NewPlayer creates a player at the given spawn point.
func NewPlayer(id string, spawn SpawnPoint, isBot bool, respawnDelay time.Duration) *Player {
	return &Player{
		ID:           id,
		X:            spawn.X,
		Y:            spawn.Y,
		HP:           MaxHP,
		Alive:        true,
		IsBot:        isBot,
		IsHuman:      !isBot,
		RespawnDelay: respawnDelay,
	}
}

// CheatLabel returns a comma-separated string of active cheats, or "none".
func (p *Player) CheatLabel() string {
	var labels []string
	if p.Cheats.Aimbot {
		labels = append(labels, "aimbot")
	}
	if p.Cheats.SpeedHack {
		labels = append(labels, "speedhack")
	}
	if p.Cheats.WallHack {
		labels = append(labels, "wallhack")
	}
	if p.Cheats.TriggerBot {
		labels = append(labels, "triggerbot")
	}
	if len(labels) == 0 {
		return "none"
	}
	return strings.Join(labels, ",")
}

// ApplyInput reads LatestInput and updates velocity and aim.
func (p *Player) ApplyInput(dt float64) {
	if p.LatestInput == nil {
		return
	}

	input := p.LatestInput

	// Compute desired velocity from WASD
	ax, ay := 0.0, 0.0
	if input.Keys.W {
		ay -= MoveAccel
	}
	if input.Keys.S {
		ay += MoveAccel
	}
	if input.Keys.A {
		ax -= MoveAccel
	}
	if input.Keys.D {
		ax += MoveAccel
	}

	p.VX += ax
	p.VY += ay

	// Cap speed
	maxSpd := MaxSpeed
	if p.Cheats.SpeedHack {
		maxSpd *= 2.5
	}

	speed := math.Sqrt(p.VX*p.VX + p.VY*p.VY)
	if speed > maxSpd {
		scale := maxSpd / speed
		p.VX *= scale
		p.VY *= scale
	}

	// Apply friction/damping
	p.VX *= 0.85
	p.VY *= 0.85

	// Update aim from mouse coordinates
	p.PrevAim = p.AimAngle
	p.AimAngle = math.Atan2(input.Mouse.Y-p.Y, input.Mouse.X-p.X)

	p.Shooting = input.Shooting
}

// ComputeSpatialContext finds nearest enemy and computes spatial awareness fields.
func (p *Player) ComputeSpatialContext(others []*Player, walls []telemetry.Wall, now time.Time) {
	p.NearestEnemyDist = math.MaxFloat64
	p.NearestEnemyAngle = 0
	p.NearestEnemyVisible = false
	p.AimToEnemyOffset = math.Pi
	p.EnemiesVisible = 0

	anyVisible := false

	for _, other := range others {
		if other.ID == p.ID || !other.Alive {
			continue
		}

		dist := Distance(p.X, p.Y, other.X, other.Y)
		angle := AngleBetween(p.X, p.Y, other.X, other.Y)
		visible := HasLineOfSight(walls, p.X, p.Y, other.X, other.Y)

		if visible {
			p.EnemiesVisible++
			anyVisible = true
		}

		if dist < p.NearestEnemyDist {
			p.NearestEnemyDist = dist
			p.NearestEnemyAngle = angle
			p.NearestEnemyVisible = visible
			p.AimToEnemyOffset = AngleDiff(p.AimAngle, angle)
		}
	}

	// Track time since any enemy was visible
	if anyVisible {
		p.lastVisibleTime = now
		p.wasVisible = true
		p.TimeSinceVisible = 0
	} else if p.wasVisible {
		p.TimeSinceVisible = now.Sub(p.lastVisibleTime).Seconds()
	} else {
		p.TimeSinceVisible = -1 // never seen
	}
}

// Die marks the player as dead and starts the respawn timer.
func (p *Player) Die() {
	p.Alive = false
	p.HP = 0
	p.RespawnTimer = p.RespawnDelay
}

// Respawn resets the player at the given spawn point.
func (p *Player) Respawn(spawn SpawnPoint) {
	p.X = spawn.X
	p.Y = spawn.Y
	p.VX = 0
	p.VY = 0
	p.HP = MaxHP
	p.Alive = true
	p.RespawnTimer = 0
}

// ToTelemetry converts the player state into a telemetry event.
func (p *Player) ToTelemetry(tick int64, now time.Time) telemetry.PlayerTelemetry {
	return telemetry.PlayerTelemetry{
		Timestamp:           now.UnixMilli(),
		PlayerID:            p.ID,
		Tick:                tick,
		PosX:                p.X,
		PosY:                p.Y,
		VelX:                p.VX,
		VelY:                p.VY,
		AimAngle:            p.AimAngle,
		AimDelta:            AngleDiff(p.AimAngle, p.PrevAim),
		IsShooting:          p.Shooting,
		HitTarget:           false, // set by game loop on hit
		Health:              p.HP,
		IsAlive:             p.Alive,
		NearestEnemyDist:    p.NearestEnemyDist,
		NearestEnemyAngle:   p.NearestEnemyAngle,
		NearestEnemyVisible: p.NearestEnemyVisible,
		AimToEnemyOffset:    p.AimToEnemyOffset,
		TimeSinceVisible:    p.TimeSinceVisible,
		EnemiesVisible:      p.EnemiesVisible,
		CheatLabel:          p.CheatLabel(),
	}
}

// ToState converts the player to a broadcast-ready PlayerState.
func (p *Player) ToState() telemetry.PlayerState {
	return telemetry.PlayerState{
		ID:       p.ID,
		X:        p.X,
		Y:        p.Y,
		Aim:      p.AimAngle,
		HP:       p.HP,
		Alive:    p.Alive,
		Shooting: p.Shooting,
	}
}

// NewBotID returns a bot identifier like "bot-0", "bot-1", etc.
func NewBotID(index int) string {
	return fmt.Sprintf("bot-%d", index)
}
