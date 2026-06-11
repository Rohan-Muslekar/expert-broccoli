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
