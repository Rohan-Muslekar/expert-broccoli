import json
import os
import tempfile

import pandas as pd

from training.cs2_parser import (
    _identify_cheaters,
    _parse_match,
    load_cs2cd_dataset,
    extract_features_from_ticks,
)


def _write_json(directory, filename, data):
    path = os.path.join(directory, filename)
    with open(path, "w") as f:
        json.dump(data, f)
    return path


def _write_parquet(directory, filename, data_dict):
    path = os.path.join(directory, filename)
    df = pd.DataFrame(data_dict)
    df.to_parquet(path, index=False)
    return path


MATCH_JSON = {
    "cheaters": [
        {"steamid": "STEAM_1"},
        {"steamid": "STEAM_3"},
    ],
}


def test_identify_cheaters():
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", MATCH_JSON)
        cheater_ids = _identify_cheaters(json_path)
        assert cheater_ids == {"STEAM_1", "STEAM_3"}


def test_identify_cheaters_no_cheaters():
    data = {"cheaters": []}
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", data)
        cheater_ids = _identify_cheaters(json_path)
        assert cheater_ids == set()


PARQUET_COLUMNS = [
    "tick", "steamid", "X", "Y", "velocity_X", "velocity_Y",
    "yaw", "health", "is_alive", "FIRE", "spotted", "team_name",
]


def _make_match_data(num_ticks, players):
    data = {col: [] for col in PARQUET_COLUMNS}
    for tick in range(num_ticks):
        for player in players:
            data["tick"].append(tick)
            data["steamid"].append(player["steamid"])
            data["X"].append(player.get("x", 0.0))
            data["Y"].append(player.get("y", 0.0))
            data["velocity_X"].append(0.0)
            data["velocity_Y"].append(0.0)
            data["yaw"].append(player.get("yaw", 0.0))
            data["health"].append(100)
            data["is_alive"].append(True)
            data["FIRE"].append(False)
            data["spotted"].append(False)
            data["team_name"].append(player["team"])
    return data


def test_parse_match_labels_cheaters():
    players = [
        {"steamid": "STEAM_0", "team": "CT", "x": 0.0},
        {"steamid": "STEAM_1", "team": "CT", "x": 100.0},
        {"steamid": "STEAM_2", "team": "CT", "x": 200.0},
        {"steamid": "STEAM_3", "team": "TERRORIST", "x": 300.0},
        {"steamid": "STEAM_4", "team": "TERRORIST", "x": 400.0},
    ]
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", MATCH_JSON)
        parquet_data = _make_match_data(5, players)
        _write_parquet(tmpdir, "match.parquet", parquet_data)
        ticks = _parse_match(
            os.path.join(tmpdir, "match.parquet"),
            json_path,
        )
        assert len(ticks) == 25
        cheater_ticks = [t for t in ticks if t["cheat_label"] == "cheater"]
        clean_ticks = [t for t in ticks if t["cheat_label"] == "none"]
        assert len(cheater_ticks) == 10
        assert len(clean_ticks) == 15


def test_parse_match_computes_nearest_enemy():
    match_json = {"cheaters": []}
    players = [
        {"steamid": "P0", "team": "CT", "x": 0.0, "y": 0.0},
        {"steamid": "P1", "team": "TERRORIST", "x": 300.0, "y": 400.0},
    ]
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", match_json)
        parquet_data = _make_match_data(1, players)
        _write_parquet(tmpdir, "match.parquet", parquet_data)
        ticks = _parse_match(
            os.path.join(tmpdir, "match.parquet"),
            json_path,
        )
        p0_tick = [t for t in ticks if t["pid"] == "P0"][0]
        assert abs(p0_tick["nearest_enemy_dist"] - 500.0) < 1.0


def test_load_cs2cd_dataset_walks_subdirectories():
    cheater_json = {"cheaters": [{"steamid": "P1"}]}
    clean_json = {"cheaters": []}
    players = [
        {"steamid": "P0", "team": "CT", "x": 0.0},
        {"steamid": "P1", "team": "TERRORIST", "x": 100.0},
    ]
    with tempfile.TemporaryDirectory() as tmpdir:
        cheater_dir = os.path.join(tmpdir, "with_cheater_present")
        clean_dir = os.path.join(tmpdir, "no_cheater_present")
        os.makedirs(cheater_dir)
        os.makedirs(clean_dir)

        _write_json(cheater_dir, "0.json", cheater_json)
        _write_parquet(cheater_dir, "0.parquet", _make_match_data(1, players))

        _write_json(clean_dir, "0.json", clean_json)
        _write_parquet(clean_dir, "0.parquet", _make_match_data(1, players))

        all_ticks = load_cs2cd_dataset(tmpdir)
        assert len(all_ticks) == 4
        cheater_count = sum(1 for t in all_ticks if t["cheat_label"] == "cheater")
        assert cheater_count == 1


def test_extract_features_from_ticks_produces_vectors():
    ticks = []
    for tick_num in range(180):
        ticks.append({
            "ts": tick_num, "pid": "player_0", "tick": tick_num,
            "x": 0.0, "y": 0.0, "vx": 1.0, "vy": 0.0,
            "aim": 0.0, "aim_delta": 0.01,
            "shooting": False, "hit": False,
            "hp": 100, "alive": True,
            "nearest_enemy_dist": 500.0, "nearest_enemy_angle": 0.0,
            "nearest_enemy_visible": False, "aim_enemy_offset": 1.0,
            "time_since_visible": 0.0, "enemies_visible": 0,
            "cheat_label": "none",
        })
    features = extract_features_from_ticks(ticks, sample_interval=60)
    assert len(features) > 0
    assert features[0]["cheat_label"] == "none"
    assert "aim_delta_mean_1s" in features[0]
    assert features[0]["pid"] == "player_0"
