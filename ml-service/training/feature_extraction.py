import math
from collections import deque

WINDOW_1S = 60
WINDOW_5S = 300
WINDOW_30S = 1800


class FeatureExtractor:
    def __init__(self, player_id: str):
        self.player_id = player_id
        self.buffer: deque = deque(maxlen=WINDOW_30S)

    def push(self, tick: dict):
        self.buffer.append(tick)

    def compute(self) -> dict:
        buffer_list = list(self.buffer)
        window_1s = buffer_list[-WINDOW_1S:] if len(buffer_list) >= WINDOW_1S else buffer_list
        window_5s = buffer_list[-WINDOW_5S:] if len(buffer_list) >= WINDOW_5S else buffer_list
        window_30s = buffer_list

        return {
            "aim_delta_mean_1s": _mean(window_1s, "aim_delta"),
            "aim_delta_mean_5s": _mean(window_5s, "aim_delta"),
            "aim_delta_max_1s": _max(window_1s, "aim_delta"),
            "aim_snap_count_5s": _count_where(window_5s, lambda tick: tick["aim_delta"] > 0.5),
            "aim_to_enemy_offset_mean_5s": _mean(window_5s, "aim_enemy_offset"),
            "hit_rate_5s": _hit_rate(window_5s),
            "hit_rate_30s": _hit_rate(window_30s),
            "shots_fired_5s": _count_where(window_5s, lambda tick: tick["shooting"]),
            "kills_per_30s": _count_kills(window_30s),
            "time_to_kill_mean_30s": _mean_kill_sequence_length(window_30s),
            "speed_mean_1s": _mean_func(window_1s, _speed),
            "speed_mean_5s": _mean_func(window_5s, _speed),
            "speed_max_1s": _max_func(window_1s, _speed),
            "direction_change_count_5s": _direction_changes(window_5s),
            "aim_lock_ratio_5s": _aim_lock_ratio(window_5s),
            "prefire_ratio_5s": _prefire_ratio(window_5s),
            "reaction_time_mean_5s": _reaction_time_mean(window_5s),
            "enemy_tracking_score_5s": _enemy_tracking_score(window_5s),
        }


def _speed(tick: dict) -> float:
    return math.sqrt(tick["vx"] ** 2 + tick["vy"] ** 2)


def _mean(data: list, field: str) -> float:
    if not data:
        return 0.0
    return sum(tick[field] for tick in data) / len(data)


def _mean_func(data: list, func) -> float:
    if not data:
        return 0.0
    return sum(func(tick) for tick in data) / len(data)


def _max(data: list, field: str) -> float:
    if not data:
        return 0.0
    return max(tick[field] for tick in data)


def _max_func(data: list, func) -> float:
    if not data:
        return 0.0
    return max(func(tick) for tick in data)


def _count_where(data: list, predicate) -> int:
    return sum(1 for tick in data if predicate(tick))


def _hit_rate(data: list) -> float:
    shot_count = 0
    hit_count = 0
    for tick in data:
        if tick["shooting"]:
            shot_count += 1
            if tick["hit"]:
                hit_count += 1
    if shot_count == 0:
        return 0.0
    return hit_count / shot_count


def _count_kills(data: list) -> int:
    kill_count = 0
    in_hit_sequence = False
    for tick in data:
        if tick["hit"] and not in_hit_sequence:
            in_hit_sequence = True
        elif not tick["hit"] and in_hit_sequence:
            kill_count += 1
            in_hit_sequence = False
    if in_hit_sequence:
        kill_count += 1
    return kill_count


def _mean_kill_sequence_length(data: list) -> float:
    sequence_lengths = []
    current_length = 0
    for tick in data:
        if tick["hit"] and tick["shooting"]:
            current_length += 1
        elif current_length > 0:
            sequence_lengths.append(current_length)
            current_length = 0
    if current_length > 0:
        sequence_lengths.append(current_length)
    if not sequence_lengths:
        return 0.0
    return sum(sequence_lengths) / len(sequence_lengths)


def _direction_changes(data: list) -> int:
    if len(data) < 2:
        return 0
    change_count = 0
    for i in range(1, len(data)):
        prev_angle = math.atan2(data[i - 1]["vy"], data[i - 1]["vx"])
        curr_angle = math.atan2(data[i]["vy"], data[i]["vx"])
        prev_speed = _speed(data[i - 1])
        curr_speed = _speed(data[i])
        if prev_speed < 0.1 or curr_speed < 0.1:
            continue
        angle_diff = abs(curr_angle - prev_angle)
        if angle_diff > math.pi:
            angle_diff = 2 * math.pi - angle_diff
        if angle_diff > math.pi / 2:
            change_count += 1
    return change_count


def _aim_lock_ratio(data: list) -> float:
    visible_ticks = 0
    locked_ticks = 0
    for tick in data:
        if tick["nearest_enemy_visible"]:
            visible_ticks += 1
            if tick["aim_enemy_offset"] < 0.1:
                locked_ticks += 1
    if visible_ticks == 0:
        return 0.0
    return locked_ticks / visible_ticks


def _prefire_ratio(data: list) -> float:
    shooting_ticks = 0
    prefire_ticks = 0
    for tick in data:
        if tick["shooting"]:
            shooting_ticks += 1
            if not tick["nearest_enemy_visible"]:
                prefire_ticks += 1
    if shooting_ticks == 0:
        return 0.0
    return prefire_ticks / shooting_ticks


def _reaction_time_mean(data: list) -> float:
    if len(data) < 2:
        return 0.0
    reaction_times = []
    ticks_since_visible = -1
    for i in range(len(data)):
        if i > 0 and data[i]["nearest_enemy_visible"] and not data[i - 1]["nearest_enemy_visible"]:
            ticks_since_visible = 0
        if ticks_since_visible >= 0:
            ticks_since_visible += 1
            if data[i]["shooting"]:
                reaction_times.append(ticks_since_visible)
                ticks_since_visible = -1
    if not reaction_times:
        return 0.0
    return sum(reaction_times) / len(reaction_times)


def _enemy_tracking_score(data: list) -> float:
    if len(data) < 3:
        return 0.0
    aim_deltas = []
    enemy_angle_deltas = []
    for i in range(1, len(data)):
        if not data[i]["nearest_enemy_visible"] or not data[i - 1]["nearest_enemy_visible"]:
            continue
        aim_deltas.append(data[i]["aim"] - data[i - 1]["aim"])
        enemy_angle_deltas.append(data[i]["nearest_enemy_angle"] - data[i - 1]["nearest_enemy_angle"])
    if len(aim_deltas) < 2:
        return 0.0
    return _pearson_correlation(aim_deltas, enemy_angle_deltas)


def _pearson_correlation(x: list, y: list) -> float:
    count = len(x)
    if count == 0 or count != len(y):
        return 0.0
    mean_x = sum(x) / count
    mean_y = sum(y) / count
    numerator = 0.0
    sum_sq_deviation_x = 0.0
    sum_sq_deviation_y = 0.0
    for i in range(count):
        diff_x = x[i] - mean_x
        diff_y = y[i] - mean_y
        numerator += diff_x * diff_y
        sum_sq_deviation_x += diff_x ** 2
        sum_sq_deviation_y += diff_y ** 2
    denominator = math.sqrt(sum_sq_deviation_x * sum_sq_deviation_y)
    if denominator < 1e-12:
        return 0.0
    return numerator / denominator
