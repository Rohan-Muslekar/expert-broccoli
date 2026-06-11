import csv
import logging
import math
import os

from training.feature_extraction import FeatureExtractor

logger = logging.getLogger(__name__)

CS2_FIELD_MAP = {
    "position_x": "x",
    "position_y": "y",
    "velocity_x": "vx",
    "velocity_y": "vy",
    "yaw": "aim",
    "is_firing": "shooting",
    "hit_entity": "hit",
    "health": "hp",
    "is_alive": "alive",
}


def load_cs2cd_csv(dataset_path: str) -> list[dict]:
    """Load CS2CD dataset from a directory of per-match CSV files.

    Expected directory structure:
        dataset_path/
            match_001.csv
            match_002.csv
            ...

    Each CSV has columns for per-tick player state. The parser maps
    CS2 field names to our schema and computes derived fields
    (aim_delta, nearest_enemy_*, aim_enemy_offset).
    """
    all_ticks = []
    csv_files = sorted(f for f in os.listdir(dataset_path) if f.endswith(".csv"))
    logger.info("Found %d match files in %s", len(csv_files), dataset_path)

    for filename in csv_files:
        filepath = os.path.join(dataset_path, filename)
        match_ticks = _parse_match_file(filepath)
        all_ticks.extend(match_ticks)

    logger.info("Loaded %d total ticks from CS2CD", len(all_ticks))
    return all_ticks


def _parse_match_file(filepath: str) -> list[dict]:
    ticks = []
    with open(filepath, "r") as csvfile:
        reader = csv.DictReader(csvfile)
        prev_aim_by_player: dict[str, float] = {}

        for row in reader:
            player_id = row.get("player_id", row.get("steamid", "unknown"))
            tick_dict = _row_to_tick(row, player_id, prev_aim_by_player)
            if tick_dict:
                ticks.append(tick_dict)
                prev_aim_by_player[player_id] = tick_dict["aim"]

    return ticks


def _row_to_tick(row: dict, player_id: str, prev_aim: dict[str, float]) -> dict | None:
    try:
        aim_angle = float(row.get("yaw", 0.0))
        previous_aim = prev_aim.get(player_id, aim_angle)
        aim_delta = abs(aim_angle - previous_aim)
        if aim_delta > math.pi:
            aim_delta = 2 * math.pi - aim_delta

        is_shooting = row.get("is_firing", "false").lower() in ("true", "1", "yes")
        hit_target = row.get("hit_entity", "false").lower() in ("true", "1", "yes")
        is_alive = row.get("is_alive", "true").lower() in ("true", "1", "yes")
        is_cheater = row.get("is_cheater", row.get("label", "false")).lower() in (
            "true",
            "1",
            "yes",
            "cheater",
        )

        nearest_enemy_x = float(row.get("nearest_enemy_x", 0.0))
        nearest_enemy_y = float(row.get("nearest_enemy_y", 0.0))
        player_x = float(row.get("position_x", 0.0))
        player_y = float(row.get("position_y", 0.0))

        dx = nearest_enemy_x - player_x
        dy = nearest_enemy_y - player_y
        nearest_enemy_dist = math.sqrt(dx * dx + dy * dy)
        nearest_enemy_angle = math.atan2(dy, dx)
        nearest_enemy_visible = row.get("enemy_visible", "false").lower() in ("true", "1", "yes")
        aim_to_enemy_offset = abs(aim_angle - nearest_enemy_angle)
        if aim_to_enemy_offset > math.pi:
            aim_to_enemy_offset = 2 * math.pi - aim_to_enemy_offset

        return {
            "ts": int(float(row.get("tick", row.get("timestamp", 0)))),
            "pid": player_id,
            "tick": int(float(row.get("tick", 0))),
            "x": player_x,
            "y": player_y,
            "vx": float(row.get("velocity_x", 0.0)),
            "vy": float(row.get("velocity_y", 0.0)),
            "aim": aim_angle,
            "aim_delta": aim_delta,
            "shooting": is_shooting,
            "hit": hit_target,
            "hp": int(float(row.get("health", 100))),
            "alive": is_alive,
            "nearest_enemy_dist": nearest_enemy_dist,
            "nearest_enemy_angle": nearest_enemy_angle,
            "nearest_enemy_visible": nearest_enemy_visible,
            "aim_enemy_offset": aim_to_enemy_offset,
            "time_since_visible": 0.0,
            "enemies_visible": 1 if nearest_enemy_visible else 0,
            "cheat_label": "cheater" if is_cheater else "none",
        }
    except (ValueError, KeyError):
        logger.debug("Skipping malformed row: %s", row)
        return None


def extract_features_from_ticks(ticks: list[dict]) -> list[dict]:
    """Group ticks by player, run feature extraction, return feature dicts."""
    players: dict[str, list[dict]] = {}
    for tick in ticks:
        players.setdefault(tick["pid"], []).append(tick)

    feature_vectors = []
    for player_id, player_ticks in players.items():
        extractor = FeatureExtractor(player_id)
        for tick in player_ticks:
            extractor.push(tick)
            if len(extractor.buffer) >= 60:
                features = extractor.compute()
                features["pid"] = player_id
                features["cheat_label"] = tick["cheat_label"]
                features["ts"] = tick["ts"]
                feature_vectors.append(features)

    logger.info("Extracted %d feature vectors from %d players", len(feature_vectors), len(players))
    return feature_vectors
