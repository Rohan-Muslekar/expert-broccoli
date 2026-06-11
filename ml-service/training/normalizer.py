import numpy as np
import joblib
from sklearn.preprocessing import StandardScaler


class FeatureNormalizer:
    def __init__(self, feature_names: list[str]):
        self.feature_names = feature_names
        self.scaler = StandardScaler()
        self.is_fitted = False

    def _to_array(self, samples: list[dict]) -> np.ndarray:
        return np.array([[sample[name] for name in self.feature_names] for sample in samples])

    def fit_transform(self, samples: list[dict]) -> np.ndarray:
        array = self._to_array(samples)
        result = self.scaler.fit_transform(array)
        self.is_fitted = True
        return result

    def transform(self, samples: list[dict]) -> np.ndarray:
        array = self._to_array(samples)
        return self.scaler.transform(array)

    def save(self, path: str):
        joblib.dump(self.scaler, path)

    @classmethod
    def load(cls, path: str, feature_names: list[str]) -> "FeatureNormalizer":
        normalizer = cls(feature_names)
        normalizer.scaler = joblib.load(path)
        normalizer.is_fitted = True
        return normalizer
