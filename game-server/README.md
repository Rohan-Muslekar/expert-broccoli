# Game Server

Multiplayer FPS game server written in Go. Runs a 60 Hz tick-based game loop with WebSocket-connected players and AI bots. Includes an embedded feature engine that computes behavioral vectors and rule-based cheat detection in real time.

## How It Works

1. The server runs a fixed-rate game loop (60 ticks/second)
2. Each tick: process player input, update physics, perform raycast hit detection, compute spatial context
3. The telemetry producer publishes raw player state to Kafka (`telemetry.raw`)
4. The feature engine buffers per-player telemetry and computes 18 behavioral features over sliding windows (1s, 5s, 30s)
5. Feature vectors are published to Kafka (`features.computed`) for ML inference
6. Six rule-based detection rules fire alerts immediately to `alerts.detections`

## Features Computed (18 total)

| Category | Features |
|----------|----------|
| Aim | aim_delta_mean_1s, aim_delta_mean_5s, aim_delta_max_1s, aim_snap_count_5s, aim_to_enemy_offset_mean_5s |
| Combat | hit_rate_5s, hit_rate_30s, shots_fired_5s, kills_per_30s, time_to_kill_mean_30s |
| Movement | speed_mean_1s, speed_mean_5s, speed_max_1s, direction_change_count_5s |
| Advanced | aim_lock_ratio_5s, prefire_ratio_5s, reaction_time_mean_5s, enemy_tracking_score_5s |

## Rule-Based Detection

Six heuristic rules provide immediate detection alongside the ML pipeline:

1. **Speed cap** - flags movement speed exceeding physics limits
2. **Aim snap** - detects instantaneous aim jumps (inhuman flick speed)
3. **Inhuman accuracy** - flags sustained hit rates above human capability
4. **Aim lock** - detects aim locked onto enemy positions
5. **Prefire** - flags shooting before line-of-sight is established
6. **Triggerbot reaction** - detects reaction times below human threshold

## API

| Endpoint | Description |
|----------|-------------|
| `GET /ws` | WebSocket for game clients (game state + input) |
| `GET /dashboard-ws` | WebSocket for monitoring dashboards |
| `GET /metrics` | Prometheus metrics |

## Kafka Topics

| Topic | Direction | Content |
|-------|-----------|---------|
| `telemetry.raw` | produces | Raw player telemetry per tick |
| `events.kills` | produces | Kill events with metadata |
| `features.computed` | produces | 18-feature behavioral vectors |
| `alerts.detections` | produces | Rule-based cheat alerts |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP/WebSocket server port |
| `TICK_RATE` | `60` | Game ticks per second |
| `MAX_PLAYERS` | `16` | Maximum connected players |
| `BOT_COUNT` | `8` | Number of AI bot players |
| `BOT_CHEATS_ENABLED` | `true` | Enable cheat behaviors on bots |
| `KAFKA_ENABLED` | `true` | Enable Kafka telemetry publishing |
| `KAFKA_BROKERS` | `kafka:9092` | Kafka broker addresses |
| `PRODUCE_TOPIC` | `features.computed` | Feature output topic |
| `ALERTS_TOPIC` | `alerts.detections` | Alert output topic |
| `RESPAWN_DELAY_SEC` | `3` | Seconds before player respawn |
| `PLAYER_TIMEOUT_SEC` | `60` | Disconnect timeout for idle players |
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics endpoint |

## Running Locally

```bash
# With Kafka
go build -o game-server .
KAFKA_BROKERS=localhost:9092 ./game-server

# Without Kafka (standalone)
go build -o game-server .
KAFKA_ENABLED=false ./game-server
```

## Project Structure

```
game-server/
  main.go              # Entry point, game loop orchestration
  config/config.go     # Environment variable configuration
  server/
    game.go            # Core game logic, player management, physics
    server.go          # WebSocket server, connection handling
    player.go          # Player entity, movement, health, respawn
    bot.go             # Bot AI with cheat capabilities
    cheat.go           # Cheat behavior application
    maps.go            # Arena geometry (walls, spawn points)
  telemetry/
    schema.go          # Data structures (PlayerTelemetry, KillEvent, etc.)
    producer.go        # Kafka producer for telemetry and events
  feature/
    engine.go          # Feature pipeline orchestration
    processor.go       # 18-feature vector computation
    rules.go           # Rule-based detection logic
  metrics/
    metrics.go         # Prometheus metric definitions
```

## Bot Cheat Types

Bots can simulate these cheat behaviors for training data generation:

- **Aimbot**: Automatically snaps aim to nearest visible enemy
- **Speed hack**: Moves faster than normal physics allow
- **Wallhack**: Knows enemy positions through walls
- **Triggerbot**: Fires automatically when crosshair is on an enemy
