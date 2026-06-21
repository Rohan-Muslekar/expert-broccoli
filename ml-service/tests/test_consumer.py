import pytest
from consumer import PlayerBuffer, SEQUENCE_LENGTH


def _make_feature_dict(player_id="p1", cheat_label="none"):
    return {
        "ts": 1000, "pid": player_id, "tick": 1,
        "x": 0.0, "y": 0.0, "vx": 1.0, "vy": 0.0,
        "aim": 0.0, "aim_delta": 0.1, "shooting": False, "hit": False,
        "hp": 100, "alive": True,
        "nearest_enemy_dist": 10.0, "nearest_enemy_angle": 0.0,
        "nearest_enemy_visible": False, "aim_enemy_offset": 1.0,
        "time_since_visible": 0.0, "enemies_visible": 0,
        "cheat_label": cheat_label,
        "aim_delta_mean_1s": 0.1, "aim_delta_mean_5s": 0.1,
        "aim_delta_max_1s": 0.2, "aim_snap_count_5s": 0,
        "aim_to_enemy_offset_mean_5s": 1.0,
        "hit_rate_5s": 0.0, "hit_rate_30s": 0.0,
        "shots_fired_5s": 0, "kills_per_30s": 0,
        "time_to_kill_mean_30s": 0.0,
        "speed_mean_1s": 1.0, "speed_mean_5s": 1.0,
        "speed_max_1s": 1.0, "direction_change_count_5s": 0,
        "aim_lock_ratio_5s": 0.0, "prefire_ratio_5s": 0.0,
        "reaction_time_mean_5s": 0.0, "enemy_tracking_score_5s": 0.0,
    }


def test_buffer_accumulates():
    buffer = PlayerBuffer("p1")
    for _ in range(10):
        buffer.push(_make_feature_dict())
    assert buffer.tick_count == 10
    assert not buffer.has_full_sequence()


def test_buffer_signals_full_sequence():
    buffer = PlayerBuffer("p1")
    for _ in range(SEQUENCE_LENGTH):
        buffer.push(_make_feature_dict())
    assert buffer.has_full_sequence()


def test_buffer_returns_latest_features():
    buffer = PlayerBuffer("p1")
    buffer.push(_make_feature_dict())
    latest = buffer.latest_features()
    assert latest is not None
    assert len(latest) == 18


from unittest.mock import MagicMock, patch
from consumer import InferenceConsumer


def _make_mock_config(auto_train_threshold=0):
    config = MagicMock()
    config.auto_train_threshold = auto_train_threshold
    config.model_dir = "/tmp/models"
    config.anomaly_std_multiplier = 3.0
    return config


def test_auto_retrain_not_triggered_when_threshold_zero():
    config = _make_mock_config(auto_train_threshold=0)
    collector = MagicMock()
    consumer = InferenceConsumer(config=config, collector=collector)
    consumer._maybe_auto_retrain()


def test_auto_retrain_triggered_at_threshold():
    import time
    config = _make_mock_config(auto_train_threshold=100)
    collector = MagicMock()
    collector.count.return_value = 100
    consumer = InferenceConsumer(
        config=config,
        xgboost_classifier=MagicMock(),
        autoencoder=MagicMock(),
        alert_combiner=MagicMock(),
        normalizer=MagicMock(),
        collector=collector,
    )
    with patch.object(consumer, "_run_retrain") as mock_retrain:
        consumer._maybe_auto_retrain()
        # _run_retrain is invoked on a background thread; give it a moment to start
        time.sleep(0.1)
        mock_retrain.assert_called_once()


def test_auto_retrain_skipped_when_not_at_threshold():
    config = _make_mock_config(auto_train_threshold=100)
    collector = MagicMock()
    collector.count.return_value = 50
    consumer = InferenceConsumer(
        config=config,
        xgboost_classifier=MagicMock(),
        autoencoder=MagicMock(),
        collector=collector,
    )
    consumer._maybe_auto_retrain()
