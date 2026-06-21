# Real-Time Cheat Detection System - End-to-End Design

**Course:** ENGR 5785G - Real-Time Data Analytics for IoT
**Date:** 2026-06-17

## 1. System Overview

A real-time cheat detection pipeline for multiplayer games. A 2D arena shooter generates player telemetry at 60Hz, streams it through Apache Kafka, computes behavioral features with sliding windows, and runs dual-paradigm ML detection (supervised XGBoost + unsupervised LSTM autoencoder). Detection results are visualized in Grafana dashboards.

The system detects four cheat types: **aimbot**, **speedhack**, **wallhack**, and **triggerbot**.

## 2. Architecture

```
                          Docker Compose (7 containers)

  Browser (:80)            Game Server (:8080)               Apache Kafka
 +--------------+   WS   +----------------------+          +---------------+
 | HTML5 Canvas |<------->| WebSocket Handler    |          |               |
 | game.js      | input/  | Game Loop (60 Hz)    |--------->| telemetry.raw |
 | F5-F8 cheats | state   | 8 Bot AI + Humans    |--------->| events.kills  |
 +--------------+         | Cheat Simulation     |          |               |
                          +----------+-----------+          +-------+-------+
                                     |                              |
                          +----------v-----------+                  |
                          | Feature Engine (Go)  |                  |
                          | Sliding Windows      |          +-------v-------+
                          | 6 Detection Rules    |--------->|features.comput|
                          | (human players only) |--------->|alerts.detect..|
                          +----------------------+          +-------+-------+
                                                                    |
                          +----------------------+          +-------v-------+
                          | ML Service (:8000)   |<---------|               |
                          | XGBoost Classifier   |--------->|alerts.detect..|
                          | LSTM Autoencoder     |          +---------------+
                          | Alert Combiner       |
                          +----------------------+

                          +----------------------+          +---------------+
                          | Prometheus (:9091)   |<---------| Scrape :8080  |
                          |                      |<---------| Scrape :8000  |
                          +----------+-----------+          +---------------+
                                     |
                          +----------v-----------+
                          | Grafana (:3000)      |
                          | Pipeline Health      |
                          | Detection Analytics  |
                          +----------------------+
```

## 3. Data Flow (End to End)

### Step 1: Player Input (Browser to Game Server)

The HTML5 game client sends input state every frame via WebSocket:

```json
{
  "keys": {"w": true, "a": false, "s": false, "d": true},
  "mouse": {"x": 450, "y": 300},
  "shooting": true,
  "cheats": {"aimbot": false, "speedhack": false, "wallhack": false, "triggerbot": false}
}
```

- **WASD** controls movement velocity (acceleration-based with friction damping)
- **Mouse position** determines aim angle (server computes `atan2(mouse.y - player.y, mouse.x - player.x)`)
- **Left click** fires hitscan rays along aim direction
- **F5/F6/F7/F8** toggle cheat flags (sent to server, applied server-side)

Cheat flags are only applied to human players. Bots never have cheats enabled.

### Step 2: Game Server Tick (60Hz Authoritative Loop)

Each tick (16.6ms), the game server executes in order:

1. **Bot AI Update** - Bots choose movement direction, track nearest visible enemy, decide whether to shoot
2. **Apply Input** - Read WASD keys to set velocity, apply friction (0.85 damping), update position (`X += VX`, `Y += VY`), compute aim angle from mouse coords
3. **Apply Cheats** - For any player with active cheat flags:
   - *Aimbot*: Snap aim to nearest visible enemy
   - *Speedhack*: Multiply velocity by 2.5x (bypasses normal 5.0 speed cap)
   - *Wallhack*: Slowly track nearest enemy through walls (10% interpolation/tick)
   - *Triggerbot*: Auto-fire when aim is within 0.1 rad of any visible enemy
4. **Wall Collision** - Push players out of wall rectangles (AABB resolution)
5. **Combat** - Raycast from shooter along aim angle, check circle intersection with other players. Hits deal 20 HP damage. Death at 0 HP, respawn after 3 seconds.
6. **Spatial Context** - For each player, compute:
   - Distance and angle to nearest enemy
   - Line-of-sight check (ray vs walls)
   - Aim-to-enemy angular offset
   - Time since any enemy was visible
   - Count of visible enemies
7. **Publish Telemetry** - Emit `PlayerTelemetry` (18 fields) per player to Kafka `telemetry.raw`
8. **Feature Engine** - Human player telemetry is also routed to the in-process feature engine (bot telemetry is excluded to prevent false positives)
9. **Broadcast State** - Send authoritative game state to all connected WebSocket clients

### Step 3: Telemetry Schema

Every tick, per player, the server produces:

| Field | Type | Description |
|-------|------|-------------|
| `ts` | int64 | Unix milliseconds |
| `pid` | string | Player ID (e.g., "player-0", "bot-3") |
| `tick` | int64 | Server tick number |
| `x`, `y` | float64 | Position in arena (1200x800 pixels) |
| `vx`, `vy` | float64 | Velocity components |
| `aim` | float64 | Aim angle in radians |
| `aim_delta` | float64 | Change in aim since last tick |
| `shooting` | bool | Currently firing |
| `hit` | bool | Shot hit a target this tick |
| `hp` | int | Health (0-100) |
| `alive` | bool | Is alive |
| `nearest_enemy_dist` | float64 | Distance to closest living enemy |
| `nearest_enemy_angle` | float64 | Angle to closest living enemy |
| `nearest_enemy_visible` | bool | Line-of-sight to nearest enemy |
| `aim_enemy_offset` | float64 | Angular difference between aim and nearest enemy direction |
| `time_since_visible` | float64 | Seconds since any enemy was visible |
| `enemies_visible` | int | Count of enemies with clear line of sight |
| `cheat_label` | string | Ground truth: "none", "aimbot", "speedhack", "wallhack", "triggerbot" |

At 60 ticks/sec with 9 players (8 bots + 1 human), this produces ~540 telemetry events/sec to Kafka.

### Step 4: Feature Engine (Sliding Window Computation)

The feature engine maintains a per-player ring buffer (up to 1800 entries = 30 seconds at 60Hz). On each incoming telemetry event, it computes 18 derived features across three time windows:

**Windows:**
- 1-second window = last 60 ticks
- 5-second window = last 300 ticks
- 30-second window = last 1800 ticks

**Computed Features (18 total):**

| Category | Feature | Window | Formula |
|----------|---------|--------|---------|
| **Aim Dynamics** | `aim_delta_mean_1s` | 1s | Mean of `aim_delta` |
| | `aim_delta_mean_5s` | 5s | Mean of `aim_delta` |
| | `aim_delta_max_1s` | 1s | Max `aim_delta` (detects sudden snaps) |
| | `aim_snap_count_5s` | 5s | Count of ticks where `aim_delta > 0.5 rad` |
| | `aim_to_enemy_offset_mean_5s` | 5s | Mean angular offset from nearest enemy |
| **Combat** | `hit_rate_5s` | 5s | Hits / shots fired |
| | `hit_rate_30s` | 30s | Hits / shots fired (longer baseline) |
| | `shots_fired_5s` | 5s | Count of shooting ticks |
| | `kills_per_30s` | 30s | Kill count (hit sequences ending) |
| | `time_to_kill_mean_30s` | 30s | Mean consecutive-hit-tick count per kill |
| **Movement** | `speed_mean_1s` | 1s | Mean of `sqrt(vx^2 + vy^2)` |
| | `speed_mean_5s` | 5s | Mean speed |
| | `speed_max_1s` | 1s | Max speed (detects speed cap violation) |
| | `direction_change_count_5s` | 5s | Velocity angle changes > 90 degrees |
| **Spatial** | `aim_lock_ratio_5s` | 5s | Fraction of visible-enemy ticks with `offset < 0.1 rad` |
| | `prefire_ratio_5s` | 5s | Fraction of shots fired while enemy NOT visible |
| | `reaction_time_mean_5s` | 5s | Mean ticks from enemy-becomes-visible to first shot |
| | `enemy_tracking_score_5s` | 5s | Pearson correlation between aim angle changes and enemy angle changes |

The feature vector (original 18 telemetry + 18 computed = 36 fields) is published to Kafka `features.computed`.

### Step 5: Rule-Based Detection (Instant Alerts)

Six threshold rules run on every feature vector, providing sub-second detection:

| Rule | Condition | Cheat Type | Confidence |
|------|-----------|------------|------------|
| `speed_cap` | `speed_max_1s > 7.0` | speedhack | 0.95 |
| `aim_snap` | `aim_delta_max_1s > 2.0 rad` | aimbot | 0.95 |
| `inhuman_accuracy` | `hit_rate_5s > 85%` AND `shots_fired_5s > 30` | aimbot | 0.90 |
| `aim_lock` | `aim_lock_ratio_5s > 90%` AND `enemies_visible > 0` | aimbot | 0.90 |
| `prefire` | `prefire_ratio_5s > 60%` AND `shots_fired_5s > 20` | wallhack | 0.90 |
| `triggerbot_reaction` | `reaction_time_mean_5s < 3 ticks` AND `shots_fired_5s > 10` | triggerbot | 0.90 |

**How each rule maps to cheat behavior:**

- **speed_cap**: Normal max speed is 5.0 units/tick. Speedhack multiplies velocity by 2.5x, pushing max to ~12.5. Threshold at 7.0 catches any speedhack usage within 1 second.
- **aim_snap**: Aimbot snaps aim directly to enemy, causing aim_delta > 2.0 rad in a single tick. Normal mouse movement rarely exceeds 0.5 rad/tick.
- **inhuman_accuracy**: Aimbot produces hit rates above 85% over sustained fire. Normal players average 20-40%.
- **aim_lock**: With aimbot active, aim stays within 0.1 rad of enemy position >90% of the time the enemy is visible. Normal tracking is much noisier.
- **prefire**: Wallhack causes aim tracking through walls, leading to shots fired before enemies are visible. >60% prefire ratio is not achievable by legitimate play.
- **triggerbot_reaction**: Triggerbot fires within 1-2 ticks of enemy appearing. Humans need ~10-20 ticks (166-333ms) reaction time.

Alerts are published to Kafka `alerts.detections` with `source: "rule-engine"`.

### Step 6: ML-Based Detection (XGBoost + LSTM)

The Python ML service consumes from `features.computed` and runs two models:

#### XGBoost Classifier (per-tick, supervised)

- **Input**: Single normalized feature vector (18 features)
- **Output**: Multi-class prediction + per-class probabilities
- **Classes**: `none`, `cheater`, `aimbot`, `speedhack`, `wallhack`, `triggerbot`
- **Training data**: CS2CD dataset (795 real CS2 matches, 451 with cheaters, 344 clean)
- **Alert condition**: Predicted class != "none" AND confidence >= 0.8

XGBoost catches **known cheat patterns** that match what it was trained on.

#### LSTM Autoencoder (per-second, unsupervised)

- **Input**: Sequence of 60 consecutive feature vectors (1 second of play)
- **Architecture**: Encoder LSTM(64) -> LSTM(32) -> latent -> Decoder LSTM(32) -> LSTM(64) -> Dense(18)
- **Training data**: Clean players only (learns "what normal looks like")
- **Anomaly score**: Reconstruction error (MSE between input sequence and reconstructed output)
- **Alert condition**: Anomaly score > `mean + 3 * std` (calibrated from training distribution)
- **Runs every 60 ticks** (once per second), not every tick

The autoencoder catches **novel/unknown cheat types** that XGBoost was never trained on, because any behavior deviating from normal play patterns produces high reconstruction error.

#### Alert Combiner

Merges signals from both models:

| Scenario | Alert `model` field | Confidence |
|----------|--------------------|----|
| XGBoost alone triggers | `"xgboost"` | XGBoost probability |
| Autoencoder alone triggers | `"autoencoder"` | Mapped from anomaly score (std deviations / 5) |
| Both trigger simultaneously | `"ensemble"` | max(xgboost, autoencoder confidence) |

Per-player cooldown of 5 seconds prevents alert flooding.

### Step 7: Observability (Prometheus + Grafana)

Prometheus scrapes metrics from game-server (:8080) and ml-service (:8000) every 5 seconds.

#### Pipeline Health Dashboard

| Panel | Metric | What it shows |
|-------|--------|---------------|
| Active Players | `game_players_active` | Total players (bots + humans). Green >= 4. |
| WebSocket Connections | `game_websocket_connections` | Human players connected via browser. |
| Tick Duration | `game_tick_duration_seconds` | Game loop latency. Should be ~16ms. |
| Telemetry Rate | `telemetry_messages_published_total` | Events/sec flowing to Kafka by topic. |
| Features Published Rate | `feature_engine_features_published_total` | Feature vectors computed/sec. |
| Kafka Errors | `kafka_publish_errors_total` | Red if > 0. |
| Feature Processing Latency | `feature_engine_processing_duration_seconds` | p99 feature computation time. |
| Feature Active Players | `feature_engine_active_players` | Players being monitored by the feature engine. |
| XGBoost Model | `ml_model_loaded{model="xgboost"}` | Green "Loaded" / Red "Not Loaded". |
| Autoencoder Model | `ml_model_loaded{model="autoencoder"}` | Green "Loaded" / Red "Not Loaded". |

#### Detection Analytics Dashboard

| Panel | Metric | What it shows |
|-------|--------|---------------|
| Rule Engine Alerts | `feature_engine_alerts_fired_total` by rule | Time series of rule-based detections. Flat at zero = clean play. Spikes = cheat detected. Each line is a rule name (speed_cap, aim_snap, etc.). |
| ML Alerts by Model | `ml_alerts_published_total` by model | Time series of ML-based detections. Lines: xgboost, autoencoder, ensemble. |
| Total Alert Rate | Sum of rule + ML alert rates | Single number. Green = 0 (clean). Yellow > 0.5/s. Red > 2/s. |
| XGBoost Latency | `ml_xgboost_inference_duration_seconds` | p99 XGBoost prediction time. |
| Autoencoder Latency | `ml_autoencoder_inference_duration_seconds` | p99 LSTM inference time. |
| Predictions by Type | `ml_predictions_total` by cheat_type | Pie chart of XGBoost classification distribution. Mostly "none" during clean play. |
| Anomaly Score Distribution | `ml_autoencoder_anomaly_score` | Heatmap of reconstruction errors over time. Normal play clusters low. Cheating shifts distribution right. |
| Training Samples | `ml_training_samples_collected` | How many samples the collector has accumulated. |
| Service Mode | `ml_model_loaded{model="xgboost"}` | "Collection" (blue) = gathering training data. "Inference" (green) = models loaded, running predictions. |

## 4. Kafka Topics

| Topic | Partitions | Publisher | Consumer | Key | Retention |
|-------|-----------|-----------|----------|-----|-----------|
| `telemetry.raw` | 4 | Game Server | (raw storage) | player_id | 1 hour |
| `events.kills` | 4 | Game Server | (raw storage) | killer_id | 1 hour |
| `features.computed` | 4 | Feature Engine | ML Service | player_id | 1 hour |
| `alerts.detections` | 4 | Feature Engine + ML Service | (dashboard) | player_id | 1 hour |

Kafka runs in KRaft mode (no ZooKeeper), single broker, created via a one-shot `kafka-init` container.

## 5. Technology Stack

| Component | Technology | Port |
|-----------|-----------|------|
| Game Client | HTML5 Canvas, vanilla JS, nginx | :80 |
| Game Server | Go 1.22, gorilla/websocket, segmentio/kafka-go | :8080 |
| Message Broker | Apache Kafka 3.7 (KRaft mode) | :9092 |
| ML Service | Python, FastAPI, XGBoost, PyTorch (LSTM), scikit-learn | :8000 |
| Metrics | Prometheus 2.51 | :9091 |
| Dashboards | Grafana 10.4 | :3000 |

## 6. Training Pipeline

```
CS2CD Dataset (795 real CS2 matches, ~440MB parquet)
    |
    v
CS2 Parser (cs2_parser.py)
    - Reads CSV.gz + JSON per match
    - Maps CS2 fields to our telemetry schema (X, Y, yaw, velocity, etc.)
    - Labels: "cheater" (VAC-banned) or "none" (clean)
    |
    v
Feature Extraction (feature_extraction.py)
    - Python mirror of Go feature engine
    - Same sliding windows (1s/5s/30s at 60Hz)
    - Same 18 computed features
    - Ensures training features match inference features exactly
    |
    v
Normalization (normalizer.py)
    - Z-score normalization fitted on training data
    - Saved as scaler.joblib for inference-time use
    |
    v
Model Training
    +-- XGBoost: All labeled data (clean + cheater), 80/20 split by player ID
    +-- LSTM Autoencoder: Clean player sequences only, 50 epochs
    |
    v
Saved Models (ml-service/saved_models/)
    - xgboost_classifier.json
    - lstm_autoencoder.pt
    - scaler.joblib
    - anomaly_stats.json (mean + std of clean reconstruction errors)
    - metadata.json (training metrics, sample counts)
```

## 7. Demo Walkthrough

### Setup
1. `docker compose up -d` - starts all 7 containers
2. Open `http://localhost:80` - game client
3. Open `http://localhost:3000` - Grafana (admin/admin)

### Showing Clean Play
1. Play the game normally with WASD + mouse for ~1 minute
2. Show **Pipeline Health** dashboard:
   - Active Players shows 9 (8 bots + you)
   - WebSocket Connections shows 1
   - Telemetry Rate is non-zero (pipeline is flowing)
   - Both ML models show "Loaded" (green)
3. Show **Detection Analytics** dashboard:
   - Rule Engine Alerts: flat at zero
   - ML Alerts: flat at zero
   - Total Alert Rate: green (0)
   - Predictions by Type: nearly all "none"

### Demonstrating Cheat Detection
1. Press **F5** (aimbot) - your aim snaps to nearest visible enemy
   - Detection Analytics: `aim_snap` and `aim_lock` rules fire (Rule Engine Alerts panel)
   - ML Alerts: xgboost line spikes (classifies as "aimbot")
   - Predictions pie chart: "aimbot" slice appears
   - Total Alert Rate turns yellow/red
2. Turn off F5, press **F6** (speedhack) - you move 2.5x faster
   - `speed_cap` rule fires
   - Predictions pie chart: "speedhack" slice appears
3. Turn off F6, press **F7** (wallhack) - your aim slowly tracks enemies through walls
   - `prefire` rule may fire (shooting while enemy not visible)
   - Autoencoder anomaly scores increase (abnormal temporal pattern)
4. Turn off F7, press **F8** (triggerbot) - auto-fires when aimed at enemy
   - `triggerbot_reaction` rule fires (reaction time < 3 ticks)
5. Turn everything off
   - All alerts drop back to zero within ~5 seconds
   - Predictions pie chart shifts back toward "none"

### Key Talking Points
- **Rule engine** provides instant, explainable detection (you can show the exact threshold violated)
- **XGBoost** classifies the specific cheat type (trained on 795 real CS2 matches)
- **LSTM autoencoder** catches behavioral anomalies (would detect novel cheat types not in training data)
- **Ensemble** combines both signals for higher confidence
- Detection runs in **real-time** (<5 seconds from cheat activation to dashboard alert), unlike post-match analysis systems

## 8. File Structure

```
cheat-detection/
  docker-compose.yml              # 7 services: kafka, kafka-init, game-server,
                                  #   game-client, ml-service, prometheus, grafana
  game-client/
    index.html                    # Canvas game UI with cheat HUD
    game.js                       # Rendering, input, WebSocket, fog of war
    style.css
    Dockerfile                    # nginx

  game-server/
    main.go                       # Entry point, Kafka producer, feature engine
    server/
      server.go                   # WebSocket handler, broadcast
      game.go                     # Game loop, tick processing, combat
      player.go                   # Player state, input processing, cheat flag handling
      bot.go                      # Bot AI (movement, aiming, shooting)
      cheat.go                    # Cheat simulation (aimbot, speedhack, wallhack, triggerbot)
      maps.go                     # Wall layout, spawn points, LOS raycasting
    feature/
      engine.go                   # Kafka consumer/producer, per-player routing
      processor.go                # Sliding window feature computation (18 features)
      rules.go                    # 6 threshold-based detection rules
    telemetry/
      schema.go                   # PlayerTelemetry, KillEvent, InputState structs
      producer.go                 # Async Kafka writer
    metrics/
      metrics.go                  # Prometheus metric definitions
    config/
      config.go                   # Environment variable parsing
    Dockerfile                    # Multi-stage Go build

  ml-service/
    main.py                       # FastAPI app, model loading, /train endpoint
    consumer.py                   # Kafka consumer, per-player buffer, inference loop
    config.py                     # Service configuration
    models/
      xgboost_model.py            # XGBoost wrapper (train, predict, save, load)
      autoencoder.py              # LSTM autoencoder (train, anomaly_score, save, load)
      ensemble.py                 # Alert combiner (merges XGBoost + autoencoder signals)
    training/
      cs2_parser.py               # CS2CD dataset parser (CSV.gz + JSON)
      feature_extraction.py       # Python mirror of Go feature engine
      normalizer.py               # Z-score normalization
      collector.py                # Live sample accumulator
      trainer.py                  # Training orchestrator
      download_cs2cd.py           # Dataset download script
    metrics/
      prometheus_metrics.py       # ML-specific Prometheus metrics
    saved_models/                 # Persisted model artifacts (Docker volume)
    Dockerfile

  infra/
    kafka/
      create-topics.sh            # One-shot topic creation (4 topics, 4 partitions each)
    prometheus/
      prometheus.yml              # Scrape config (game-server + ml-service)
    grafana/
      provisioning/               # Datasource + dashboard provider config
      dashboards/
        pipeline-health.json      # Infrastructure monitoring dashboard
        detection-analytics.json  # Cheat detection dashboard

  datasets/
    cs2cd/                        # CS2CD dataset files (gitignored)
```
