import pytest
from training.trainer import CHEAT_CLASSES, TrainingPipeline


def test_cheat_classes_includes_cheater():
    assert "cheater" in CHEAT_CLASSES
    assert "none" in CHEAT_CLASSES
    assert "aimbot" in CHEAT_CLASSES
    assert len(CHEAT_CLASSES) == 6


def test_train_from_cs2cd_raises_on_empty_dataset(tmp_path):
    pipeline = TrainingPipeline(str(tmp_path / "models"))
    with pytest.raises(ValueError, match="No ticks loaded"):
        pipeline.train_from_cs2cd(str(tmp_path / "nonexistent"))
