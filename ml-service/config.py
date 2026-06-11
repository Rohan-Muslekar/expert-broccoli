import os
from dataclasses import dataclass


@dataclass
class Config:
    kafka_brokers: str = "kafka:9092"
    consume_topic: str = "features.computed"
    alerts_topic: str = "alerts.detections"
    consumer_group: str = "ml-service"
    model_dir: str = "./saved_models"
    auto_train_threshold: int = 0
    anomaly_std_multiplier: float = 3.0
    alert_cooldown_seconds: int = 5
    xgboost_confidence_threshold: float = 0.8
    host: str = "0.0.0.0"
    port: int = 8000

    @classmethod
    def from_env(cls) -> "Config":
        return cls(
            kafka_brokers=os.getenv("KAFKA_BROKERS", cls.kafka_brokers),
            consume_topic=os.getenv("CONSUME_TOPIC", cls.consume_topic),
            alerts_topic=os.getenv("ALERTS_TOPIC", cls.alerts_topic),
            consumer_group=os.getenv("CONSUMER_GROUP", cls.consumer_group),
            model_dir=os.getenv("MODEL_DIR", cls.model_dir),
            auto_train_threshold=int(os.getenv("AUTO_TRAIN_THRESHOLD", str(cls.auto_train_threshold))),
            anomaly_std_multiplier=float(os.getenv("ANOMALY_STD_MULTIPLIER", str(cls.anomaly_std_multiplier))),
            alert_cooldown_seconds=int(os.getenv("ALERT_COOLDOWN_SECONDS", str(cls.alert_cooldown_seconds))),
            xgboost_confidence_threshold=float(os.getenv("XGBOOST_CONFIDENCE_THRESHOLD", str(cls.xgboost_confidence_threshold))),
        )
