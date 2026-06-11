import time
import pytest
from models.ensemble import AlertCombiner


def test_xgboost_alert_above_threshold():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="aimbot",
        xgboost_confidence=0.9,
        anomaly_score=0.5,
        feature_importances={"aim_delta_mean_1s": 0.3},
    )
    assert alert is not None
    assert alert["cheat_type"] == "aimbot"
    assert alert["model"] == "xgboost"


def test_xgboost_below_threshold_no_alert():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="aimbot",
        xgboost_confidence=0.5,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert alert is None


def test_autoencoder_alert_above_threshold():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="none",
        xgboost_confidence=0.3,
        anomaly_score=3.0,
        feature_importances={},
    )
    assert alert is not None
    assert alert["cheat_type"] == "unknown"
    assert alert["model"] == "autoencoder"


def test_ensemble_when_both_trigger():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="speedhack",
        xgboost_confidence=0.95,
        anomaly_score=3.0,
        feature_importances={"speed_max_1s": 0.5},
    )
    assert alert is not None
    assert alert["model"] == "ensemble"
    assert alert["cheat_type"] == "speedhack"


def test_cooldown_suppresses_duplicate():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    first_alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="aimbot",
        xgboost_confidence=0.9,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert first_alert is not None
    second_alert = combiner.evaluate(
        player_id="p1",
        timestamp=1002,
        xgboost_label="aimbot",
        xgboost_confidence=0.9,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert second_alert is None


def test_cooldown_expires():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    first_alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="aimbot",
        xgboost_confidence=0.9,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert first_alert is not None
    later_alert = combiner.evaluate(
        player_id="p1",
        timestamp=1006,
        xgboost_label="aimbot",
        xgboost_confidence=0.9,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert later_alert is not None


def test_none_label_no_alert():
    combiner = AlertCombiner(
        confidence_threshold=0.8,
        anomaly_std_multiplier=3.0,
        cooldown_seconds=5,
        anomaly_mean=1.0,
        anomaly_std=0.5,
    )
    alert = combiner.evaluate(
        player_id="p1",
        timestamp=1000,
        xgboost_label="none",
        xgboost_confidence=0.95,
        anomaly_score=0.5,
        feature_importances={},
    )
    assert alert is None
