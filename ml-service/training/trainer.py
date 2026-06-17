import gc
import json
import logging
import os

import numpy as np

from models.xgboost_model import XGBoostClassifier, FEATURE_NAMES
from models.autoencoder import LSTMAutoencoder
from training.normalizer import FeatureNormalizer
from training.cs2_parser import iter_match_files, _parse_match, extract_features_from_ticks

logger = logging.getLogger(__name__)

CHEAT_CLASSES = ["none", "cheater"]
SEQUENCE_LENGTH = 60


class TrainingPipeline:
    def __init__(self, model_dir: str, anomaly_std_multiplier: float = 3.0):
        self.model_dir = model_dir
        self.anomaly_std_multiplier = anomaly_std_multiplier

    def train_from_samples(self, samples: list[dict]) -> dict:
        os.makedirs(self.model_dir, exist_ok=True)

        player_ids = list({sample["pid"] for sample in samples})
        split_index = int(len(player_ids) * 0.8)
        train_player_ids = set(player_ids[:split_index])
        test_player_ids = set(player_ids[split_index:])

        train_samples = [s for s in samples if s["pid"] in train_player_ids]
        test_samples = [s for s in samples if s["pid"] in test_player_ids]

        if not train_samples or not test_samples:
            raise ValueError(f"Not enough players for split: {len(player_ids)} players found")

        normalizer = FeatureNormalizer(FEATURE_NAMES)
        train_features = normalizer.fit_transform(train_samples)
        test_features = normalizer.transform(test_samples)

        train_labels = np.array(["none" if s.get("cheat_label", "none") == "none" else "cheater" for s in train_samples])
        test_labels = np.array(["none" if s.get("cheat_label", "none") == "none" else "cheater" for s in test_samples])

        logger.info("Training XGBoost on %d samples (%d test)", len(train_samples), len(test_samples))
        xgboost_classifier = XGBoostClassifier(CHEAT_CLASSES)
        xgboost_metrics = xgboost_classifier.train(train_features, train_labels, test_features, test_labels)
        logger.info("XGBoost metrics: %s", xgboost_metrics)

        clean_train_mask = train_labels == "none"
        clean_train_features = train_features[clean_train_mask]
        logger.info("Training LSTM autoencoder on %d clean samples", len(clean_train_features))

        max_autoencoder_samples = 5000
        if len(clean_train_features) > max_autoencoder_samples:
            step = len(clean_train_features) // max_autoencoder_samples
            clean_train_features = clean_train_features[::step][:max_autoencoder_samples]
            logger.info("Downsampled autoencoder training to %d samples", len(clean_train_features))

        clean_sequences = self._make_sequences(clean_train_features, SEQUENCE_LENGTH)
        if len(clean_sequences) < 10:
            raise ValueError(f"Not enough clean sequences for autoencoder: {len(clean_sequences)}")

        autoencoder = LSTMAutoencoder(num_features=len(FEATURE_NAMES), sequence_length=SEQUENCE_LENGTH)
        autoencoder_history = autoencoder.train(clean_sequences, epochs=10, batch_size=64)
        threshold_stats = autoencoder.calibrate_threshold(clean_sequences)
        logger.info("Autoencoder threshold: mean=%.4f, std=%.4f", threshold_stats["mean"], threshold_stats["std"])

        xgboost_classifier.save(os.path.join(self.model_dir, "xgboost_classifier.json"))
        autoencoder.save(os.path.join(self.model_dir, "lstm_autoencoder.pt"))
        normalizer.save(os.path.join(self.model_dir, "scaler.joblib"))

        with open(os.path.join(self.model_dir, "anomaly_threshold.json"), "w") as threshold_file:
            json.dump(threshold_stats, threshold_file)

        metadata = {
            "total_samples": len(samples),
            "train_samples": len(train_samples),
            "test_samples": len(test_samples),
            "clean_sequences": len(clean_sequences),
            "num_players": len(player_ids),
            "xgboost_metrics": xgboost_metrics,
            "autoencoder_final_loss": autoencoder_history["train_losses"][-1],
            "anomaly_threshold": threshold_stats,
        }
        with open(os.path.join(self.model_dir, "metadata.json"), "w") as metadata_file:
            json.dump(metadata, metadata_file, indent=2)

        logger.info("All models saved to %s", self.model_dir)
        return metadata

    def train_from_cs2cd(self, cs2cd_path: str, live_samples: list[dict] | None = None, min_cs2cd_samples: int = 1000, max_matches: int = 5) -> dict:
        match_files = iter_match_files(cs2cd_path)
        if not match_files:
            raise ValueError(f"No match files found in CS2CD dataset at {cs2cd_path}")

        match_files = match_files[:max_matches]
        all_features = []
        for match_index, (parquet_path, json_path) in enumerate(match_files):
            match_name = os.path.basename(parquet_path)
            logger.info("Processing match %d/%d: %s", match_index + 1, len(match_files), match_name)

            ticks = _parse_match(parquet_path, json_path)
            if not ticks:
                logger.warning("No ticks from %s, skipping", match_name)
                continue

            features = extract_features_from_ticks(ticks)
            logger.info("Match %s: %d ticks -> %d features", match_name, len(ticks), len(features))
            all_features.extend(features)

            del ticks, features
            gc.collect()

        if len(all_features) < min_cs2cd_samples:
            raise ValueError(f"Only {len(all_features)} CS2CD features extracted, need {min_cs2cd_samples}")

        logger.info("CS2CD total: %d feature vectors from %d matches", len(all_features), len(match_files))

        if live_samples:
            all_features.extend(live_samples)
            logger.info("Merged %d live samples, total: %d", len(live_samples), len(all_features))

        metadata = self.train_from_samples(all_features)
        metadata["cs2cd_samples"] = len(all_features) - (len(live_samples) if live_samples else 0)
        metadata["live_samples"] = len(live_samples) if live_samples else 0
        return metadata

    def _make_sequences(self, features: np.ndarray, sequence_length: int) -> np.ndarray:
        if len(features) < sequence_length:
            return np.array([])
        sequences = []
        for i in range(len(features) - sequence_length + 1):
            sequences.append(features[i : i + sequence_length])
        return np.array(sequences, dtype=np.float32)
