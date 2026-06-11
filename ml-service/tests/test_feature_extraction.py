import math
import pytest
from training.feature_extraction import FeatureExtractor

FEATURE_NAMES = [
    "aim_delta_mean_1s", "aim_delta_mean_5s", "aim_delta_max_1s",
    "aim_snap_count_5s", "aim_to_enemy_offset_mean_5s",
    "hit_rate_5s", "hit_rate_30s", "shots_fired_5s",
    "kills_per_30s", "time_to_kill_mean_30s",
    "speed_mean_1s", "speed_mean_5s", "speed_max_1s",
    "direction_change_count_5s",
    "aim_lock_ratio_5s", "prefire_ratio_5s",
    "reaction_time_mean_5s", "enemy_tracking_score_5s",
]


def make_tick(
    vel_x=0.0, vel_y=0.0, aim_angle=0.0, aim_delta=0.0,
    is_shooting=False, hit_target=False,
    nearest_enemy_dist=10.0, nearest_enemy_angle=0.0,
    nearest_enemy_visible=False, aim_to_enemy_offset=1.0,
):
    return {
        "x": 0.0, "y": 0.0,
        "vx": vel_x, "vy": vel_y,
        "aim": aim_angle, "aim_delta": aim_delta,
        "shooting": is_shooting, "hit": hit_target,
        "hp": 100, "alive": True,
        "nearest_enemy_dist": nearest_enemy_dist,
        "nearest_enemy_angle": nearest_enemy_angle,
        "nearest_enemy_visible": nearest_enemy_visible,
        "aim_enemy_offset": aim_to_enemy_offset,
        "time_since_visible": 0.0, "enemies_visible": 1 if nearest_enemy_visible else 0,
        "cheat_label": "none", "ts": 0, "pid": "p1", "tick": 0,
    }


def test_output_has_all_18_features():
    extractor = FeatureExtractor("p1")
    for _ in range(60):
        extractor.push(make_tick())
    features = extractor.compute()
    for name in FEATURE_NAMES:
        assert name in features, f"Missing feature: {name}"


def test_speed_computation():
    extractor = FeatureExtractor("p1")
    for _ in range(60):
        extractor.push(make_tick(vel_x=3.0, vel_y=4.0))
    features = extractor.compute()
    assert abs(features["speed_mean_1s"] - 5.0) < 0.01
    assert abs(features["speed_max_1s"] - 5.0) < 0.01


def test_aim_delta_mean():
    extractor = FeatureExtractor("p1")
    for _ in range(60):
        extractor.push(make_tick(aim_delta=0.5))
    features = extractor.compute()
    assert abs(features["aim_delta_mean_1s"] - 0.5) < 0.01


def test_hit_rate():
    extractor = FeatureExtractor("p1")
    for i in range(300):
        extractor.push(make_tick(
            is_shooting=True,
            hit_target=(i % 2 == 0),
        ))
    features = extractor.compute()
    assert abs(features["hit_rate_5s"] - 0.5) < 0.01


def test_aim_snap_count():
    extractor = FeatureExtractor("p1")
    for i in range(300):
        delta = 1.0 if i % 10 == 0 else 0.1
        extractor.push(make_tick(aim_delta=delta))
    features = extractor.compute()
    assert features["aim_snap_count_5s"] == 30


def test_empty_buffer_returns_zeros():
    extractor = FeatureExtractor("p1")
    extractor.push(make_tick())
    features = extractor.compute()
    assert features["speed_mean_1s"] == 0.0
    assert features["hit_rate_5s"] == 0.0
