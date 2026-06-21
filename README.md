# Real-Time Cheat Detection System

A real-time multiplayer game cheat detection pipeline built for ENGR 5785G (Real-Time Data Analytics for IoT). The system ingests game telemetry via Kafka, computes behavioral features, and uses an ML ensemble (XGBoost + LSTM autoencoder) to detect cheating in real time.

## Architecture

```
Game Client (HTML5)
    |  WebSocket
    v
Game Server (Go, 60 Hz tick)
    |  Kafka
    |---> telemetry.raw       (raw player state)
    |---> events.kills        (kill events)
    |
    v
Feature Engine (Go, embedded)
    |---> features.computed   (18-feature vectors)
    |---> alerts.detections   (rule-based alerts)
    |
    v
ML Service (Python, FastAPI)
    |  XGBoost classifier + LSTM autoencoder ensemble
    |---> alerts.detections   (ML-based alerts)
    |
    v
Prometheus + Grafana          (observability dashboards)
```

## Components

| Component | Language | Port | Description |
|-----------|----------|------|-------------|
| [game-server](game-server/) | Go | 8080 | Multiplayer FPS with bot AI, feature engine, and Kafka telemetry |
| [game-client](game-client/) | JavaScript | 80 | HTML5 Canvas game client with fog-of-war rendering |
| [ml-service](ml-service/) | Python | 8000 | FastAPI service with XGBoost + LSTM ensemble inference |
| Kafka | - | 9092 | Apache Kafka 3.7.0 in KRaft mode (single broker) |
| Prometheus | - | 9091 | Metrics collection from game-server and ml-service |
| Grafana | - | 3000 | Pipeline health and detection analytics dashboards |

See each component's README for detailed documentation.

## Quick Start

### Prerequisites

- Docker and Docker Compose
- (Optional) Go 1.23+ for local game-server development
- (Optional) Python 3.11+ for local ML service development

### Run the Full Stack

```bash
make up          # builds and starts all services
make logs        # tail logs from all containers
make down        # stop all services
make clean       # stop and remove volumes
```

Or with Docker Compose directly:

```bash
docker compose up --build -d
```

### Access Points

- **Game Client:** http://localhost:80
- **Game Server API:** http://localhost:8080
- **ML Service API:** http://localhost:8000
- **Grafana Dashboards:** http://localhost:3000
- **Prometheus:** http://localhost:9091

### Verify the Pipeline

```bash
# Check all services are running
docker compose ps

# List Kafka topics
make kafka-topics

# Check ML service health
curl http://localhost:8000/health

# Check ML service status (model info, sample counts)
curl http://localhost:8000/status
```

## Kafka Topics

| Topic | Partitions | Producer | Consumer | Payload |
|-------|-----------|----------|----------|---------|
| `telemetry.raw` | 4 | game-server | (archive) | Player position, velocity, aim, health per tick |
| `events.kills` | 4 | game-server | (archive) | Kill events with reaction time |
| `features.computed` | 4 | feature-engine | ml-service | 18-feature behavioral vectors per player |
| `alerts.detections` | 4 | feature-engine, ml-service | (external) | Cheat detection alerts with confidence scores |

## ML Models

The detection ensemble uses two models:

1. **XGBoost Classifier** (supervised): Classifies player behavior into cheat categories (none, cheater, aimbot, speedhack, wallhack, triggerbot) using 18 behavioral features.

2. **LSTM Autoencoder** (unsupervised): Trained on clean player sequences to detect anomalous behavior via reconstruction error.

Models can be trained on:
- **Live data** collected from the running game (`POST /train`)
- **CS2CD dataset** (Counter-Strike 2 match recordings in parquet format) (`POST /train/cs2cd`)

Pre-trained models are included in `ml-service/saved_models/`.

## Project Structure

```
cheat-detection/
  game-server/        # Go game server + feature engine
  game-client/        # HTML5 Canvas frontend
  ml-service/         # Python ML service (FastAPI)
  infra/              # Kafka, Prometheus, Grafana configs
  datasets/           # CS2CD dataset (not committed)
  docs/               # Design specs and plans
  scripts/            # Utility scripts
  docker-compose.yml  # Full stack orchestration
  Makefile            # Build and run shortcuts
```
