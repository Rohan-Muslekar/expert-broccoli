package server

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"cheat-detection/game-server/config"
	"cheat-detection/game-server/metrics"
	"cheat-detection/game-server/telemetry"
)

type Game struct {
	mu      sync.RWMutex
	Players []*Player
	Bots    []*BotAI
	Walls   []telemetry.Wall
	Spawns  []SpawnPoint
	Tick    int64
	Cfg     config.Config

	kills       []telemetry.KillFeedEntry
	killsCh     chan telemetry.KillEvent
	telemetryCh chan telemetry.PlayerTelemetry
}

func NewGame(cfg config.Config) *Game {
	walls := KillboxWalls()
	spawns := KillboxSpawns()
	respawnDelay := time.Duration(cfg.RespawnDelaySec) * time.Second

	g := &Game{
		Walls:       walls,
		Spawns:      spawns,
		Cfg:         cfg,
		killsCh:     make(chan telemetry.KillEvent, 100),
		telemetryCh: make(chan telemetry.PlayerTelemetry, 1000),
	}

	for i := 0; i < cfg.BotCount; i++ {
		spawn := spawns[i%len(spawns)]
		p := NewPlayer(NewBotID(i), spawn, true, respawnDelay)
		g.Players = append(g.Players, p)
		g.Bots = append(g.Bots, NewBotAI(p, cfg.BotCheatsEnabled))
	}

	metrics.PlayersActive.Set(float64(len(g.Players)))
	return g
}

func (g *Game) AddHumanPlayer() *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, p := range g.Players {
		if p.IsBot {
			spawn := g.Spawns[i%len(g.Spawns)]
			human := NewPlayer(
				fmt.Sprintf("player-%d", i),
				spawn,
				false,
				time.Duration(g.Cfg.RespawnDelaySec)*time.Second,
			)
			g.Players[i] = human
			for j, b := range g.Bots {
				if b.player.ID == p.ID {
					g.Bots = append(g.Bots[:j], g.Bots[j+1:]...)
					break
				}
			}
			metrics.WebSocketConns.Inc()
			return human
		}
	}

	if len(g.Players) >= g.Cfg.MaxPlayers {
		return nil
	}

	spawn := g.Spawns[len(g.Players)%len(g.Spawns)]
	human := NewPlayer(
		fmt.Sprintf("player-%d", len(g.Players)),
		spawn,
		false,
		time.Duration(g.Cfg.RespawnDelaySec)*time.Second,
	)
	g.Players = append(g.Players, human)
	metrics.PlayersActive.Set(float64(len(g.Players)))
	metrics.WebSocketConns.Inc()
	return human
}

func (g *Game) RemoveHumanPlayer(playerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, p := range g.Players {
		if p.ID == playerID && p.IsHuman {
			spawn := g.Spawns[i%len(g.Spawns)]
			botID := NewBotID(i)
			bot := NewPlayer(botID, spawn, true, time.Duration(g.Cfg.RespawnDelaySec)*time.Second)
			g.Players[i] = bot
			g.Bots = append(g.Bots, NewBotAI(bot, g.Cfg.BotCheatsEnabled))
			metrics.WebSocketConns.Dec()
			return
		}
	}
}

func (g *Game) RunTick(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Tick++
	dt := time.Duration(1000/g.Cfg.TickRate) * time.Millisecond

	// 1. Update bot AI
	for _, b := range g.Bots {
		b.Update(dt, g.Players, g.Walls)
	}

	// 2. Apply inputs + cheats + movement
	for _, p := range g.Players {
		if !p.Alive {
			p.RespawnTimer -= dt
			if p.RespawnTimer <= 0 {
				spawn := g.Spawns[rand.Intn(len(g.Spawns))]
				p.Respawn(spawn)
			}
			continue
		}

		p.ApplyInput(1.0)
		ApplyCheats(p, g.Players, g.Walls)
		p.X, p.Y = ResolveWallCollision(g.Walls, p.X, p.Y, PlayerRadius)

		p.X = math.Max(PlayerRadius, math.Min(ArenaW-PlayerRadius, p.X))
		p.Y = math.Max(PlayerRadius, math.Min(ArenaH-PlayerRadius, p.Y))
	}

	// 3. Combat: raycast hit detection
	for _, p := range g.Players {
		if !p.Alive || !p.Shooting {
			continue
		}

		hitPlayer, hitDist := g.raycastHit(p)
		if hitPlayer != nil {
			hitPlayer.HP -= DamagePerHit
			telem := p.ToTelemetry(g.Tick, now)
			telem.HitTarget = true

			if hitPlayer.HP <= 0 {
				hitPlayer.Die()
				kill := telemetry.KillEvent{
					Timestamp:      now.UnixNano(),
					KillerID:       p.ID,
					VictimID:       hitPlayer.ID,
					Distance:       hitDist,
					ReactionTimeMs: p.TimeSinceVisible * 1000,
				}
				select {
				case g.killsCh <- kill:
				default:
				}
				g.kills = append(g.kills, telemetry.KillFeedEntry{
					Killer: p.ID,
					Victim: hitPlayer.ID,
					Ts:     now.UnixNano(),
				})
				if len(g.kills) > 5 {
					g.kills = g.kills[len(g.kills)-5:]
				}
			}
		}
	}

	// 4. Compute spatial context + generate telemetry
	for _, p := range g.Players {
		p.ComputeSpatialContext(g.Players, g.Walls, now)

		telem := p.ToTelemetry(g.Tick, now)
		select {
		case g.telemetryCh <- telem:
		default:
		}
	}
}

func (g *Game) raycastHit(shooter *Player) (*Player, float64) {
	rayLen := 800.0
	endX := shooter.X + math.Cos(shooter.AimAngle)*rayLen
	endY := shooter.Y + math.Sin(shooter.AimAngle)*rayLen

	var closest *Player
	closestDist := math.MaxFloat64

	for _, p := range g.Players {
		if p.ID == shooter.ID || !p.Alive {
			continue
		}

		dist := rayCircleIntersect(shooter.X, shooter.Y, endX, endY, p.X, p.Y, PlayerRadius)
		if dist >= 0 && dist < closestDist {
			hitX := shooter.X + math.Cos(shooter.AimAngle)*dist
			hitY := shooter.Y + math.Sin(shooter.AimAngle)*dist
			if HasLineOfSight(g.Walls, shooter.X, shooter.Y, hitX, hitY) {
				closestDist = dist
				closest = p
			}
		}
	}
	return closest, closestDist
}

func rayCircleIntersect(x1, y1, x2, y2, cx, cy, r float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	fx := x1 - cx
	fy := y1 - cy

	a := dx*dx + dy*dy
	b := 2 * (fx*dx + fy*dy)
	c := fx*fx + fy*fy - r*r

	disc := b*b - 4*a*c
	if disc < 0 {
		return -1
	}

	disc = math.Sqrt(disc)
	t := (-b - disc) / (2 * a)
	if t >= 0 && t <= 1 {
		return t * math.Sqrt(a)
	}
	return -1
}

func (g *Game) GetState() telemetry.GameStateMsg {
	g.mu.RLock()
	defer g.mu.RUnlock()

	players := make([]telemetry.PlayerState, len(g.Players))
	for i, p := range g.Players {
		players[i] = p.ToState()
	}

	return telemetry.GameStateMsg{
		Tick:    g.Tick,
		Players: players,
		Kills:   g.kills,
	}
}

func (g *Game) TelemetryCh() <-chan telemetry.PlayerTelemetry {
	return g.telemetryCh
}

func (g *Game) KillsCh() <-chan telemetry.KillEvent {
	return g.killsCh
}
