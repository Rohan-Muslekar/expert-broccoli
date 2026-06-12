import gzip
import json
import os
import tempfile

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


def _write_csv_gz(directory, filename, header, rows):
    path = os.path.join(directory, filename)
    lines = [",".join(header)] + [",".join(str(v) for v in row) for row in rows]
    content = "\n".join(lines).encode("utf-8")
    with gzip.open(path, "wb") as f:
        f.write(content)
    return path


MATCH_JSON = {
    "match_id": "test_001",
    "cheater_steamids": ["STEAM_1", "STEAM_3"],
    "players": [
        {"steamid": "STEAM_0", "team": "CT"},
        {"steamid": "STEAM_1", "team": "CT"},
        {"steamid": "STEAM_2", "team": "CT"},
        {"steamid": "STEAM_3", "team": "T"},
        {"steamid": "STEAM_4", "team": "T"},
    ],
}


def test_identify_cheaters():
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", MATCH_JSON)
        cheater_ids = _identify_cheaters(json_path)
        assert cheater_ids == {"STEAM_1", "STEAM_3"}


def test_identify_cheaters_no_cheaters():
    data = {
        "match_id": "clean",
        "cheater_steamids": [],
        "players": [{"steamid": "STEAM_0", "team": "CT"}],
    }
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", data)
        cheater_ids = _identify_cheaters(json_path)
        assert cheater_ids == set()


CSV_HEADER = [
    "tick", "steamid", "X", "Y", "velocity_X", "velocity_Y",
    "yaw", "health", "is_alive", "FIRE", "spotted", "shots_fired",
]


def _make_player_row(tick, steamid, x, y, yaw=0.0, health=100, alive=1, fire=0, spotted=0):
    return [tick, steamid, x, y, 0.0, 0.0, yaw, health, alive, fire, spotted, 0]


def test_parse_match_labels_cheaters():
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", MATCH_JSON)
        rows = []
        for tick in range(5):
            for i, steam_id in enumerate(["STEAM_0", "STEAM_1", "STEAM_2", "STEAM_3", "STEAM_4"]):
                x = float(i * 100)
                y = float(tick * 10)
                rows.append(_make_player_row(tick, steam_id, x, y))
        _write_csv_gz(tmpdir, "match.csv.gz", CSV_HEADER, rows)
        ticks = _parse_match(
            os.path.join(tmpdir, "match.csv.gz"),
            json_path,
        )
        assert len(ticks) == 25
        cheater_ticks = [t for t in ticks if t["cheat_label"] == "cheater"]
        clean_ticks = [t for t in ticks if t["cheat_label"] == "none"]
        assert len(cheater_ticks) == 10
        assert len(clean_ticks) == 15


def test_parse_match_computes_nearest_enemy():
    match_json = {
        "match_id": "dist_test",
        "cheater_steamids": [],
        "players": [
            {"steamid": "P0", "team": "CT"},
            {"steamid": "P1", "team": "T"},
        ],
    }
    with tempfile.TemporaryDirectory() as tmpdir:
        json_path = _write_json(tmpdir, "match.json", match_json)
        rows = [
            _make_player_row(0, "P0", 0.0, 0.0),
            _make_player_row(0, "P1", 300.0, 400.0),
        ]
        _write_csv_gz(tmpdir, "match.csv.gz", CSV_HEADER, rows)
        ticks = _parse_match(
            os.path.join(tmpdir, "match.csv.gz"),
            json_path,
        )
        p0_tick = [t for t in ticks if t["pid"] == "P0"][0]
        assert abs(p0_tick["nearest_enemy_dist"] - 500.0) < 1.0


def test_load_cs2cd_dataset_walks_subdirectories():
    match_json = {
        "match_id": "walk_test",
        "cheater_steamids": ["P1"],
        "players": [
            {"steamid": "P0", "team": "CT"},
            {"steamid": "P1", "team": "T"},
        ],
    }
    with tempfile.TemporaryDirectory() as tmpdir:
        cheater_dir = os.path.join(tmpdir, "with_cheater_present")
        clean_dir = os.path.join(tmpdir, "no_cheater_present")
        os.makedirs(cheater_dir)
        os.makedirs(clean_dir)

        _write_json(cheater_dir, "match_001.json", match_json)
        rows = [_make_player_row(0, "P0", 0, 0), _make_player_row(0, "P1", 100, 0)]
        _write_csv_gz(cheater_dir, "match_001.csv.gz", CSV_HEADER, rows)

        clean_json = {**match_json, "cheater_steamids": []}
        _write_json(clean_dir, "match_301.json", clean_json)
        _write_csv_gz(clean_dir, "match_301.csv.gz", CSV_HEADER, rows)

        all_ticks = load_cs2cd_dataset(tmpdir)
        assert len(all_ticks) == 4
        cheater_count = sum(1 for t in all_ticks if t["cheat_label"] == "cheater")
        assert cheater_count == 1


def test_extract_features_from_ticks_produces_vectors():
    ticks = []
    for tick_num in range(120):
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
    features = extract_features_from_ticks(ticks)
    assert len(features) > 0
    assert features[0]["cheat_label"] == "none"
    assert "aim_delta_mean_1s" in features[0]
    assert features[0]["pid"] == "player_0"
