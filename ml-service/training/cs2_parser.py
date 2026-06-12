import json
import logging
import math
import os

import pandas as pd

from training.feature_extraction import FeatureExtractor

logger = logging.getLogger(__name__)


def load_cs2cd_dataset(dataset_path: str) -> list[dict]:
    all_ticks = []
    for subdir_name in ("with_cheater_present", "no_cheater_present"):
        subdir = os.path.join(dataset_path, subdir_name)
        if not os.path.isdir(subdir):
            logger.warning("Subdirectory not found: %s", subdir)
            continue
        parquet_files = sorted(f for f in os.listdir(subdir) if f.endswith(".parquet"))
        logger.info("Found %d match files in %s", len(parquet_files), subdir)
        for parquet_filename in parquet_files:
            json_filename = parquet_filename.replace(".parquet", ".json")
            parquet_path = os.path.join(subdir, parquet_filename)
            json_path = os.path.join(subdir, json_filename)
            if not os.path.exists(json_path):
                logger.warning("No companion JSON for %s, skipping", parquet_filename)
                continue
            match_ticks = _parse_match(parquet_path, json_path)
            all_ticks.extend(match_ticks)
    logger.info("Loaded %d total ticks from CS2CD dataset", len(all_ticks))
    return all_ticks


def _identify_cheaters(json_path: str) -> set[str]:
    with open(json_path, "r") as f:
        metadata = json.load(f)
    cheaters_list = metadata.get("cheaters", [])
    return {entry["steamid"] for entry in cheaters_list if "steamid" in entry}


def _parse_match(parquet_path: str, json_path: str) -> list[dict]:
    cheater_steam_ids = _identify_cheaters(json_path)

    ticks = []
    prev_aim_by_player: dict[str, float] = {}

    try:
        dataframe = pd.read_parquet(parquet_path)
    except Exception:
        logger.exception("Failed to read %s", parquet_path)
        return []

    required_columns = {"X", "Y", "yaw", "health", "is_alive", "tick", "steamid"}
    if not required_columns.issubset(set(dataframe.columns)):
        missing = required_columns - set(dataframe.columns)
        logger.warning("Missing columns in %s: %s", parquet_path, missing)
        return []

    has_velocity = "velocity_X" in dataframe.columns and "velocity_Y" in dataframe.columns
    has_team = "team_name" in dataframe.columns

    for tick_number, tick_group in dataframe.groupby("tick"):
        tick_rows = tick_group.to_dict("records")
        for row_index, row in enumerate(tick_rows):
            steam_id = str(row.get("steamid", f"player_{row_index}"))
            player_x = float(row.get("X", 0.0))
            player_y = float(row.get("Y", 0.0))

            velocity_x = 0.0
            velocity_y = 0.0
            if has_velocity:
                raw_vx = row.get("velocity_X")
                raw_vy = row.get("velocity_Y")
                if pd.notna(raw_vx):
                    velocity_x = float(raw_vx)
                if pd.notna(raw_vy):
                    velocity_y = float(raw_vy)

            yaw = float(row.get("yaw", 0.0))
            health = int(float(row.get("health", 100)))
            is_alive = bool(row.get("is_alive", True))
            is_firing = bool(row.get("FIRE", False))
            is_spotted = bool(row.get("spotted", False))

            previous_aim = prev_aim_by_player.get(steam_id, yaw)
            aim_delta = abs(yaw - previous_aim)
            if aim_delta > math.pi:
                aim_delta = 2 * math.pi - aim_delta
            prev_aim_by_player[steam_id] = yaw

            player_team = str(row.get("team_name", "CT")) if has_team else "CT"
            nearest_dist, nearest_angle = _compute_nearest_enemy(
                tick_rows, row_index, player_x, player_y,
                player_team, has_team,
            )

            aim_to_enemy_offset = abs(yaw - nearest_angle)
            if aim_to_enemy_offset > math.pi:
                aim_to_enemy_offset = 2 * math.pi - aim_to_enemy_offset

            is_cheater = steam_id in cheater_steam_ids
            cheat_label = "cheater" if is_cheater else "none"

            ticks.append({
                "ts": int(tick_number),
                "pid": steam_id,
                "tick": int(tick_number),
                "x": player_x,
                "y": player_y,
                "vx": velocity_x,
                "vy": velocity_y,
                "aim": yaw,
                "aim_delta": aim_delta,
                "shooting": is_firing,
                "hit": False,
                "hp": health,
                "alive": is_alive,
                "nearest_enemy_dist": nearest_dist,
                "nearest_enemy_angle": nearest_angle,
                "nearest_enemy_visible": is_spotted,
                "aim_enemy_offset": aim_to_enemy_offset,
                "time_since_visible": 0.0,
                "enemies_visible": 1 if is_spotted else 0,
                "cheat_label": cheat_label,
            })

    return ticks


def _compute_nearest_enemy(
    tick_rows: list[dict],
    player_index: int,
    player_x: float,
    player_y: float,
    player_team: str,
    has_team: bool,
) -> tuple[float, float]:
    nearest_dist = 99999.0
    nearest_angle = 0.0
    for other_index, other_row in enumerate(tick_rows):
        if other_index == player_index:
            continue
        if not bool(other_row.get("is_alive", True)):
            continue
        if has_team:
            other_team = str(other_row.get("team_name", ""))
            if other_team == player_team:
                continue
        other_x = float(other_row.get("X", 0.0))
        other_y = float(other_row.get("Y", 0.0))
        dx = other_x - player_x
        dy = other_y - player_y
        dist = math.sqrt(dx * dx + dy * dy)
        if dist < nearest_dist:
            nearest_dist = dist
            nearest_angle = math.atan2(dy, dx)
    return nearest_dist, nearest_angle


def extract_features_from_ticks(ticks: list[dict]) -> list[dict]:
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
