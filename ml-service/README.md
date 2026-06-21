# ML Service

Python service that performs real-time cheat detection using an ensemble of XGBoost (supervised classification) and LSTM autoencoder (unsupervised anomaly detection). Consumes feature vectors from Kafka, runs inference, and publishes detection alerts.

## How It Works

1. On startup, loads pre-trained models from `saved_models/`
2. A background Kafka consumer thread reads from `features.computed`
3. For each player, features are buffered into sequences of 60 timesteps
4. **XGBoost** classifies the latest feature vector into a cheat category
5. **LSTM autoencoder** computes reconstruction error on the full sequence
6. The **AlertCombiner** fuses both signals with confidence thresholds and cooldown logic
7. Alerts are published to Kafka (`alerts.detections`)

## Models

### XGBoost Classifier (Supervised)

Classifies player behavior into cheat categories using 18 behavioral features.

- **Input**: 18 features (aim, combat, movement, advanced behavioral metrics)
- **Classes**: none, cheater, aimbot, speedhack, wallhack, triggerbot
- **Hyperparameters**: max_depth=6, learning_rate=0.1, n_estimators=200

### LSTM Autoencoder (Unsupervised)

Detects anomalous behavior by measuring reconstruction error against a baseline learned from clean (non-cheating) player sequences.

- **Input**: 60-timestep sequences of 18 features
- **Architecture**: Encoder LSTM(18->64->32) | Decoder LSTM(32->32->64->18)
- **Anomaly threshold**: mean + 3 standard deviations of clean reconstruction error
- **Loss**: MSE

### Ensemble Fusion

The AlertCombiner produces alerts when:
- XGBoost predicts a cheat class with confidence >= 0.8, or
- Autoencoder anomaly score exceeds the calibrated threshold, or
- Both models agree (tagged as "ensemble")

A per-player cooldown (default 5 seconds) suppresses duplicate alerts.

## API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service status and mode (inference/collection) |
| `/status` | GET | Models loaded, samples collected, training metadata |
| `/train` | POST | Train on collected live samples (min 5000) |
| `/train/cs2cd` | POST | Train on CS2CD dataset (parquet match files) |
| `/metrics` | GET | Prometheus metrics |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `kafka:9092` | Kafka broker addresses |
| `CONSUME_TOPIC` | `features.computed` | Input feature topic |
| `ALERTS_TOPIC` | `alerts.detections` | Output alert topic |
| `CONSUMER_GROUP` | `ml-service` | Kafka consumer group ID |
| `MODEL_DIR` | `/app/saved_models` | Directory for model artifacts |
| `CS2CD_DATASET_PATH` | `/app/datasets/cs2cd` | Path to CS2CD dataset |
| `AUTO_TRAIN_THRESHOLD` | `0` | Auto-retrain after N samples (0 = disabled) |
| `ANOMALY_STD_MULTIPLIER` | `3.0` | Anomaly threshold = mean + N * std |
| `ALERT_COOLDOWN_SECONDS` | `5` | Suppress duplicate alerts within N seconds |
| `XGBOOST_CONFIDENCE_THRESHOLD` | `0.8` | Minimum confidence to fire XGBoost alert |

## Training

### From Live Data

The service can collect features from the running game and train on them:

```bash
# Start the service in collection mode (no pre-trained models)
# Play the game or let bots run to generate samples
# Trigger training when enough samples are collected:
curl -X POST http://localhost:8000/train
```

### From CS2CD Dataset

Train on Counter-Strike 2 match recordings (parquet + JSON format):

```bash
# Place CS2CD dataset at datasets/cs2cd/
# Directory structure:
#   cs2cd/
#     with_cheater_present/   (parquet + json files)
#     no_cheater_present/     (parquet + json files)

curl -X POST http://localhost:8000/train/cs2cd
```

The CS2CD parser processes matches one at a time with garbage collection between matches to stay within memory constraints.

### Training Pipeline

1. Split 80/20 by player ID (prevents player leakage across train/test)
2. Normalize features with StandardScaler
3. Train XGBoost on all labeled samples
4. Train LSTM autoencoder on clean (non-cheating) sequences only
5. Calibrate anomaly threshold on clean reconstruction errors
6. Save all artifacts to `saved_models/`

## Saved Model Artifacts

```
saved_models/
  xgboost_classifier.json     # XGBoost model weights
  lstm_autoencoder.pt          # LSTM autoencoder weights + threshold
  scaler.joblib                # Feature normalizer (StandardScaler)
  anomaly_threshold.json       # Anomaly detection threshold stats
  metadata.json                # Training metadata and metrics
```

## Running Locally

```bash
pip install -r requirements.txt
python -m uvicorn main:app --host 0.0.0.0 --port 8000
```

## Running Tests

```bash
python -m pytest tests/ -q
```

## Project Structure

```
ml-service/
  main.py                        # FastAPI app, model loading, routes
  config.py                      # Environment variable configuration
  consumer.py                    # Kafka consumer, inference loop, alert publishing
  models/
    xgboost_model.py             # XGBoost classifier (train/predict/save/load)
    autoencoder.py               # LSTM autoencoder (train/anomaly score/save/load)
    ensemble.py                  # AlertCombiner (fuses XGBoost + autoencoder)
  training/
    trainer.py                   # TrainingPipeline (end-to-end training)
    cs2_parser.py                # CS2CD dataset parser (parquet/JSON)
    normalizer.py                # Feature normalization (StandardScaler)
    collector.py                 # Live sample collector
  metrics/
    prometheus_metrics.py        # Prometheus metric definitions
  tests/                         # Unit tests
  saved_models/                  # Trained model artifacts
  requirements.txt               # Python dependencies
```
