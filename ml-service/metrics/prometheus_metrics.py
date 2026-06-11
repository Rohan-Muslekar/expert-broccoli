# ml-service/metrics/prometheus_metrics.py
from prometheus_client import Counter, Histogram, Gauge

predictions_total = Counter(
    "ml_predictions_total",
    "Total ML predictions by cheat type",
    ["cheat_type"],
)

alerts_published_total = Counter(
    "ml_alerts_published_total",
    "Total ML alerts published by model source",
    ["model"],
)

xgboost_inference_duration = Histogram(
    "ml_xgboost_inference_duration_seconds",
    "XGBoost prediction latency in seconds",
    buckets=[0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05],
)

autoencoder_inference_duration = Histogram(
    "ml_autoencoder_inference_duration_seconds",
    "LSTM autoencoder inference latency in seconds",
    buckets=[0.001, 0.005, 0.01, 0.05, 0.1, 0.5],
)

autoencoder_anomaly_score = Histogram(
    "ml_autoencoder_anomaly_score",
    "Distribution of autoencoder anomaly scores",
    buckets=[0.1, 0.5, 1.0, 2.0, 3.0, 5.0, 10.0],
)

training_samples_collected = Gauge(
    "ml_training_samples_collected",
    "Number of labeled samples accumulated for training",
)

model_loaded = Gauge(
    "ml_model_loaded",
    "Whether a model is loaded (1) or not (0)",
    ["model"],
)
