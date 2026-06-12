import json
import logging
import threading
import time
from collections import deque

import numpy as np

from models.xgboost_model import FEATURE_NAMES

logger = logging.getLogger(__name__)

SEQUENCE_LENGTH = 60


class PlayerBuffer:
    def __init__(self, player_id: str):
        self.player_id = player_id
        self.sequence_buffer: deque = deque(maxlen=SEQUENCE_LENGTH)
        self.tick_count = 0
        self._latest_raw: dict | None = None

    def push(self, feature_vector: dict):
        normalized_features = [feature_vector.get(name, 0.0) for name in FEATURE_NAMES]
        self.sequence_buffer.append(normalized_features)
        self._latest_raw = feature_vector
        self.tick_count += 1

    def has_full_sequence(self) -> bool:
        return len(self.sequence_buffer) >= SEQUENCE_LENGTH

    def get_sequence(self) -> np.ndarray:
        return np.array(list(self.sequence_buffer), dtype=np.float32)

    def latest_features(self) -> list[float] | None:
        if not self.sequence_buffer:
            return None
        return list(self.sequence_buffer[-1])


class InferenceConsumer:
    def __init__(
        self,
        config,
        xgboost_classifier=None,
        autoencoder=None,
        alert_combiner=None,
        normalizer=None,
        collector=None,
    ):
        self.config = config
        self.xgboost_classifier = xgboost_classifier
        self.autoencoder = autoencoder
        self.alert_combiner = alert_combiner
        self.normalizer = normalizer
        self.collector = collector
        self.player_buffers: dict[str, PlayerBuffer] = {}
        self._stop_event = threading.Event()
        self._thread: threading.Thread | None = None
        self._producer: KafkaProducer | None = None
        self._retrain_lock = threading.Lock()
        self._retrain_in_progress = False

    @property
    def inference_ready(self) -> bool:
        return self.xgboost_classifier is not None and self.autoencoder is not None

    def start(self):
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()
        logger.info("Kafka consumer thread started")

    def stop(self):
        self._stop_event.set()
        if self._thread:
            self._thread.join(timeout=5)
        if self._producer:
            self._producer.close()

    def _run(self):
        from kafka import KafkaConsumer, KafkaProducer

        consumer = KafkaConsumer(
            self.config.consume_topic,
            bootstrap_servers=self.config.kafka_brokers,
            group_id=self.config.consumer_group,
            value_deserializer=lambda raw: json.loads(raw.decode("utf-8")),
            auto_offset_reset="latest",
            consumer_timeout_ms=1000,
        )

        self._producer = KafkaProducer(
            bootstrap_servers=self.config.kafka_brokers,
            value_serializer=lambda value: json.dumps(value).encode("utf-8"),
        )

        logger.info("Connected to Kafka, consuming from %s", self.config.consume_topic)

        while not self._stop_event.is_set():
            try:
                messages = consumer.poll(timeout_ms=500)
                for topic_partition, records in messages.items():
                    for record in records:
                        self._process_message(record.value)
            except Exception:
                logger.exception("Error in consumer loop")
                time.sleep(1)

        consumer.close()

    def _process_message(self, feature_vector: dict):
        from metrics import prometheus_metrics as pm

        player_id = feature_vector.get("pid", "unknown")

        if self.collector:
            self.collector.add(feature_vector)
            self._maybe_auto_retrain()

        if not self.inference_ready:
            return

        if player_id not in self.player_buffers:
            self.player_buffers[player_id] = PlayerBuffer(player_id)

        buffer = self.player_buffers[player_id]

        if self.normalizer:
            normalized = self.normalizer.transform([feature_vector])
            normalized_dict = dict(zip(FEATURE_NAMES, normalized[0].tolist()))
            for name in FEATURE_NAMES:
                feature_vector[name] = normalized_dict[name]

        buffer.push(feature_vector)

        xgboost_start = time.time()
        single_features = np.array([buffer.latest_features()], dtype=np.float32)
        predicted_labels, probabilities = self.xgboost_classifier.predict(single_features)
        xgboost_label = predicted_labels[0]
        xgboost_confidence = float(probabilities[0].max())
        pm.xgboost_inference_duration.observe(time.time() - xgboost_start)
        pm.predictions_total.labels(cheat_type=xgboost_label).inc()

        anomaly_score = 0.0
        if buffer.has_full_sequence() and buffer.tick_count % SEQUENCE_LENGTH == 0:
            autoencoder_start = time.time()
            sequence = buffer.get_sequence()
            anomaly_score = self.autoencoder.anomaly_score(sequence)
            pm.autoencoder_inference_duration.observe(time.time() - autoencoder_start)
            pm.autoencoder_anomaly_score.observe(anomaly_score)

        feature_importances = self.xgboost_classifier.get_top_features(3)
        timestamp = feature_vector.get("ts", 0)

        alert = self.alert_combiner.evaluate(
            player_id=player_id,
            timestamp=timestamp,
            xgboost_label=xgboost_label,
            xgboost_confidence=xgboost_confidence,
            anomaly_score=anomaly_score,
            feature_importances=feature_importances,
        )

        if alert and self._producer:
            self._producer.send(self.config.alerts_topic, value=alert, key=player_id.encode("utf-8"))
            pm.alerts_published_total.labels(model=alert["model"]).inc()
            logger.info("Alert published: %s -> %s (%.2f)", player_id, alert["cheat_type"], alert["confidence"])

    def _maybe_auto_retrain(self):
        if not self.inference_ready:
            return
        if self.config.auto_train_threshold <= 0:
            return
        if not self.collector:
            return
        sample_count = self.collector.count()
        if sample_count == 0 or sample_count % self.config.auto_train_threshold != 0:
            return
        if not self._retrain_lock.acquire(blocking=False):
            return
        try:
            if self._retrain_in_progress:
                return
            self._retrain_in_progress = True
            logger.info("Auto-retrain triggered at %d samples", sample_count)
            retrain_thread = threading.Thread(target=self._run_retrain, daemon=True)
            retrain_thread.start()
        finally:
            self._retrain_lock.release()

    def _run_retrain(self):
        try:
            from training.trainer import TrainingPipeline
            samples = self.collector.get_all()
            pipeline = TrainingPipeline(self.config.model_dir, self.config.anomaly_std_multiplier)
            metadata = pipeline.train_from_samples(samples)
            logger.info("Auto-retrain complete: %s", metadata.get("xgboost_metrics"))

            from main import _try_load_models
            xgboost_classifier, autoencoder, alert_combiner, normalizer = _try_load_models()
            if xgboost_classifier:
                self.load_models(xgboost_classifier, autoencoder, alert_combiner, normalizer)
        except Exception:
            logger.exception("Auto-retrain failed")
        finally:
            self._retrain_in_progress = False

    def load_models(self, xgboost_classifier, autoencoder, alert_combiner, normalizer):
        from metrics import prometheus_metrics as pm

        self.xgboost_classifier = xgboost_classifier
        self.autoencoder = autoencoder
        self.alert_combiner = alert_combiner
        self.normalizer = normalizer
        pm.model_loaded.labels(model="xgboost").set(1)
        pm.model_loaded.labels(model="autoencoder").set(1)
