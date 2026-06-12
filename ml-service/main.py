import json
import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from prometheus_client import make_asgi_app

from config import Config
from consumer import InferenceConsumer
from models.xgboost_model import XGBoostClassifier, FEATURE_NAMES
from models.autoencoder import LSTMAutoencoder
from models.ensemble import AlertCombiner
from training.normalizer import FeatureNormalizer
from training.collector import LiveCollector
from training.trainer import TrainingPipeline, CHEAT_CLASSES
from metrics import prometheus_metrics as pm

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

config = Config.from_env()
collector = LiveCollector()
consumer_instance: InferenceConsumer | None = None


def _try_load_models() -> tuple:
    xgboost_path = os.path.join(config.model_dir, "xgboost_classifier.json")
    autoencoder_path = os.path.join(config.model_dir, "lstm_autoencoder.pt")
    scaler_path = os.path.join(config.model_dir, "scaler.joblib")
    threshold_path = os.path.join(config.model_dir, "anomaly_threshold.json")

    if not all(os.path.exists(p) for p in [xgboost_path, autoencoder_path, scaler_path, threshold_path]):
        return None, None, None, None

    xgboost_classifier = XGBoostClassifier.load(xgboost_path, CHEAT_CLASSES)
    autoencoder = LSTMAutoencoder.load(autoencoder_path)
    normalizer = FeatureNormalizer.load(scaler_path, FEATURE_NAMES)

    with open(threshold_path) as threshold_file:
        threshold_stats = json.load(threshold_file)

    alert_combiner = AlertCombiner(
        confidence_threshold=config.xgboost_confidence_threshold,
        anomaly_std_multiplier=config.anomaly_std_multiplier,
        cooldown_seconds=config.alert_cooldown_seconds,
        anomaly_mean=threshold_stats["mean"],
        anomaly_std=threshold_stats["std"],
    )

    pm.model_loaded.labels(model="xgboost").set(1)
    pm.model_loaded.labels(model="autoencoder").set(1)
    logger.info("Loaded pre-trained models from %s", config.model_dir)
    return xgboost_classifier, autoencoder, alert_combiner, normalizer


@asynccontextmanager
async def lifespan(app: FastAPI):
    global consumer_instance
    xgboost_classifier, autoencoder, alert_combiner, normalizer = _try_load_models()

    consumer_instance = InferenceConsumer(
        config=config,
        xgboost_classifier=xgboost_classifier,
        autoencoder=autoencoder,
        alert_combiner=alert_combiner,
        normalizer=normalizer,
        collector=collector,
    )
    consumer_instance.start()

    mode = "inference" if consumer_instance.inference_ready else "collection"
    logger.info("ML service started in %s mode", mode)

    yield

    consumer_instance.stop()
    logger.info("ML service stopped")


app = FastAPI(title="ML Cheat Detection Service", lifespan=lifespan)
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)


@app.get("/health")
def health():
    mode = "inference" if consumer_instance and consumer_instance.inference_ready else "collection"
    return {"status": "ok", "mode": mode}


@app.get("/status")
def status():
    models_loaded = consumer_instance.inference_ready if consumer_instance else False
    metadata_path = os.path.join(config.model_dir, "metadata.json")
    training_metadata = None
    if os.path.exists(metadata_path):
        with open(metadata_path) as metadata_file:
            training_metadata = json.load(metadata_file)
    return {
        "mode": "inference" if models_loaded else "collection",
        "models_loaded": models_loaded,
        "samples_collected": collector.count(),
        "training_metadata": training_metadata,
    }


@app.post("/train")
def train(min_samples: int = 5000):
    sample_count = collector.count()
    if sample_count < min_samples:
        raise HTTPException(
            status_code=400,
            detail=f"Not enough samples: {sample_count}/{min_samples}",
        )

    samples = collector.get_all()
    pipeline = TrainingPipeline(config.model_dir, config.anomaly_std_multiplier)
    metadata = pipeline.train_from_samples(samples)

    xgboost_classifier, autoencoder, alert_combiner, normalizer = _try_load_models()
    if consumer_instance and xgboost_classifier:
        consumer_instance.load_models(xgboost_classifier, autoencoder, alert_combiner, normalizer)

    return {"status": "trained", "metadata": metadata}


@app.post("/train/cs2cd")
def train_cs2cd(min_cs2cd_samples: int = 1000):
    if not config.cs2cd_dataset_path:
        raise HTTPException(status_code=400, detail="CS2CD_DATASET_PATH not configured")

    if not os.path.isdir(config.cs2cd_dataset_path):
        raise HTTPException(
            status_code=400,
            detail=f"CS2CD dataset not found at {config.cs2cd_dataset_path}",
        )

    live_samples = collector.get_all() if collector.count() > 0 else None
    pipeline = TrainingPipeline(config.model_dir, config.anomaly_std_multiplier)
    metadata = pipeline.train_from_cs2cd(
        config.cs2cd_dataset_path,
        live_samples=live_samples,
        min_cs2cd_samples=min_cs2cd_samples,
    )

    xgboost_classifier, autoencoder, alert_combiner, normalizer = _try_load_models()
    if consumer_instance and xgboost_classifier:
        consumer_instance.load_models(xgboost_classifier, autoencoder, alert_combiner, normalizer)

    return {"status": "trained", "metadata": metadata}
