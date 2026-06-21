import numpy as np
import xgboost as xgb
from sklearn.metrics import accuracy_score, f1_score, recall_score


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


class XGBoostClassifier:
    def __init__(self, class_names: list[str]):
        self.class_names = class_names
        self.label_to_index = {name: i for i, name in enumerate(class_names)}
        self.index_to_label = {i: name for i, name in enumerate(class_names)}
        self.model: xgb.XGBClassifier | None = None

    def _encode(self, labels: np.ndarray) -> np.ndarray:
        return np.array([self.label_to_index[label] for label in labels])

    def train(
        self,
        features: np.ndarray,
        labels: np.ndarray,
        test_features: np.ndarray | None = None,
        test_labels: np.ndarray | None = None,
    ) -> dict:
        encoded_labels = self._encode(labels)
        self.model = xgb.XGBClassifier(
            max_depth=6,
            learning_rate=0.1,
            n_estimators=200,
            subsample=0.8,
            colsample_bytree=0.8,
        )
        self.model.fit(features, encoded_labels)

        eval_features = test_features if test_features is not None else features
        eval_labels = test_labels if test_labels is not None else labels
        encoded_eval_labels = self._encode(eval_labels)
        predicted = self.model.predict(eval_features)
        return {
            "accuracy": float(accuracy_score(encoded_eval_labels, predicted)),
            "f1_weighted": float(f1_score(encoded_eval_labels, predicted, average="weighted")),
            "recall_weighted": float(recall_score(encoded_eval_labels, predicted, average="weighted")),
        }

    def predict(self, features: np.ndarray) -> tuple[list[str], np.ndarray]:
        probabilities = self.model.predict_proba(features)
        predicted_indices = np.argmax(probabilities, axis=1)
        predicted_labels = [self.index_to_label[i] for i in predicted_indices]
        return predicted_labels, probabilities

    def get_feature_importances(self) -> list[float]:
        return self.model.feature_importances_.tolist()

    def get_top_features(self, num_features: int = 3) -> dict[str, float]:
        importances = self.get_feature_importances()
        paired = list(zip(FEATURE_NAMES, importances))
        paired.sort(key=lambda pair: pair[1], reverse=True)
        return {name: round(importance, 4) for name, importance in paired[:num_features]}

    def save(self, path: str):
        self.model.save_model(path)

    @classmethod
    def load(cls, path: str, class_names: list[str]) -> "XGBoostClassifier":
        classifier = cls(class_names)
        classifier.model = xgb.XGBClassifier()
        classifier.model.load_model(path)
        return classifier
