import numpy as np
import os
import tempfile
import pytest
from models.autoencoder import LSTMAutoencoder


def _make_clean_sequences(num_sequences=100, sequence_length=60, num_features=18):
    np.random.seed(42)
    return np.random.randn(num_sequences, sequence_length, num_features).astype(np.float32)


def test_train_returns_loss_history():
    autoencoder = LSTMAutoencoder(num_features=18, sequence_length=60)
    sequences = _make_clean_sequences(num_sequences=50)
    history = autoencoder.train(sequences, epochs=5, batch_size=16)
    assert "train_losses" in history
    assert len(history["train_losses"]) == 5
    assert history["train_losses"][-1] < history["train_losses"][0]


def test_anomaly_score_returns_float():
    autoencoder = LSTMAutoencoder(num_features=18, sequence_length=60)
    sequences = _make_clean_sequences(num_sequences=50)
    autoencoder.train(sequences, epochs=5, batch_size=16)
    single_sequence = sequences[0]
    score = autoencoder.anomaly_score(single_sequence)
    assert isinstance(score, float)
    assert score >= 0.0


def test_per_feature_reconstruction_error():
    autoencoder = LSTMAutoencoder(num_features=18, sequence_length=60)
    sequences = _make_clean_sequences(num_sequences=50)
    autoencoder.train(sequences, epochs=5, batch_size=16)
    single_sequence = sequences[0]
    per_feature_error = autoencoder.per_feature_error(single_sequence)
    assert len(per_feature_error) == 18
    for error in per_feature_error:
        assert error >= 0.0


def test_calibrate_threshold():
    autoencoder = LSTMAutoencoder(num_features=18, sequence_length=60)
    sequences = _make_clean_sequences(num_sequences=50)
    autoencoder.train(sequences, epochs=5, batch_size=16)
    threshold_stats = autoencoder.calibrate_threshold(sequences)
    assert "mean" in threshold_stats
    assert "std" in threshold_stats
    assert threshold_stats["std"] > 0


def test_save_load_roundtrip():
    autoencoder = LSTMAutoencoder(num_features=18, sequence_length=60)
    sequences = _make_clean_sequences(num_sequences=50)
    autoencoder.train(sequences, epochs=3, batch_size=16)
    score_before = autoencoder.anomaly_score(sequences[0])
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "model.pt")
        autoencoder.save(path)
        loaded = LSTMAutoencoder.load(path, num_features=18, sequence_length=60)
        score_after = loaded.anomaly_score(sequences[0])
        assert abs(score_before - score_after) < 1e-5
