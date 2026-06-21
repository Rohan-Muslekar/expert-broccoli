import numpy as np
import pytest
import os
import tempfile
from training.normalizer import FeatureNormalizer

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


def test_fit_transform_produces_zero_mean_unit_var():
    normalizer = FeatureNormalizer(FEATURE_NAMES)
    samples = [{name: float(i + j) for j, name in enumerate(FEATURE_NAMES)} for i in range(100)]
    result = normalizer.fit_transform(samples)
    assert result.shape == (100, 18)
    means = result.mean(axis=0)
    stds = result.std(axis=0)
    for i in range(18):
        assert abs(means[i]) < 0.01
        assert abs(stds[i] - 1.0) < 0.01


def test_transform_uses_fitted_params():
    normalizer = FeatureNormalizer(FEATURE_NAMES)
    train_samples = [{name: float(i) for name in FEATURE_NAMES} for i in range(100)]
    normalizer.fit_transform(train_samples)
    test_sample = {name: 50.0 for name in FEATURE_NAMES}
    result = normalizer.transform([test_sample])
    assert result.shape == (1, 18)


def test_save_load_roundtrip():
    normalizer = FeatureNormalizer(FEATURE_NAMES)
    samples = [{name: float(i) for name in FEATURE_NAMES} for i in range(50)]
    normalizer.fit_transform(samples)
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, "scaler.joblib")
        normalizer.save(path)
        loaded = FeatureNormalizer.load(path, FEATURE_NAMES)
        original_result = normalizer.transform(samples)
        loaded_result = loaded.transform(samples)
        np.testing.assert_array_almost_equal(original_result, loaded_result)
