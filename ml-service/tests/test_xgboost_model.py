import numpy as np
import os
import tempfile
import pytest
from models.xgboost_model import XGBoostClassifier

CLASSES = ["none", "aimbot", "speedhack", "wallhack", "triggerbot"]


def _make_training_data(num_samples=500):
    np.random.seed(42)
    features = np.random.randn(num_samples, 18)
    labels = np.random.choice(CLASSES, size=num_samples)
    return features, labels


def test_train_and_predict():
    classifier = XGBoostClassifier(CLASSES)
    features, labels = _make_training_data()
    metrics = classifier.train(features, labels)
    assert "accuracy" in metrics
    predictions, probabilities = classifier.predict(features[:5])
    assert len(predictions) == 5
    assert probabilities.shape == (5, 5)
    for prediction in predictions:
        assert prediction in CLASSES


def test_predict_single():
    classifier = XGBoostClassifier(CLASSES)
    features, labels = _make_training_data()
    classifier.train(features, labels)
    single_feature = features[0:1]
    prediction, probabilities = classifier.predict(single_feature)
    assert len(prediction) == 1
    assert probabilities.shape == (1, 5)


def test_feature_importances():
    classifier = XGBoostClassifier(CLASSES)
    features, labels = _make_training_data()
    classifier.train(features, labels)
    importances = classifier.get_feature_importances()
    assert len(importances) == 18


def test_save_load_roundtrip():
    classifier = XGBoostClassifier(CLASSES)
    features, labels = _make_training_data()
    classifier.train(features, labels)
    predictions_before, _ = classifier.predict(features[:10])
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "model.json")
        classifier.save(path)
        loaded = XGBoostClassifier.load(path, CLASSES)
        predictions_after, _ = loaded.predict(features[:10])
        assert predictions_before == predictions_after
