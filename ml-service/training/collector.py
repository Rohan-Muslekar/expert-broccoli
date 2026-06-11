# ml-service/training/collector.py
import threading
from metrics.prometheus_metrics import training_samples_collected


class LiveCollector:
    def __init__(self):
        self.samples: list[dict] = []
        self.lock = threading.Lock()

    def add(self, feature_vector: dict):
        with self.lock:
            self.samples.append(feature_vector)
            training_samples_collected.set(len(self.samples))

    def count(self) -> int:
        with self.lock:
            return len(self.samples)

    def get_all(self) -> list[dict]:
        with self.lock:
            return list(self.samples)

    def clear(self):
        with self.lock:
            self.samples.clear()
            training_samples_collected.set(0)
