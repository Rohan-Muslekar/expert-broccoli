class AlertCombiner:
    def __init__(
        self,
        confidence_threshold: float,
        anomaly_std_multiplier: float,
        cooldown_seconds: int,
        anomaly_mean: float = 0.0,
        anomaly_std: float = 1.0,
    ):
        self.confidence_threshold = confidence_threshold
        self.anomaly_std_multiplier = anomaly_std_multiplier
        self.cooldown_seconds = cooldown_seconds
        self.anomaly_mean = anomaly_mean
        self.anomaly_std = anomaly_std
        self.last_alert_time: dict[str, float] = {}

    def evaluate(
        self,
        player_id: str,
        timestamp: float,
        xgboost_label: str,
        xgboost_confidence: float,
        anomaly_score: float,
        feature_importances: dict[str, float],
    ) -> dict | None:
        xgboost_triggered = (
            xgboost_label != "none"
            and xgboost_confidence >= self.confidence_threshold
        )
        anomaly_threshold = self.anomaly_mean + self.anomaly_std_multiplier * self.anomaly_std
        autoencoder_triggered = anomaly_score > anomaly_threshold

        if not xgboost_triggered and not autoencoder_triggered:
            return None

        if xgboost_triggered and autoencoder_triggered:
            model_source = "ensemble"
            cheat_type = xgboost_label
            confidence = max(xgboost_confidence, self._anomaly_to_confidence(anomaly_score))
        elif xgboost_triggered:
            model_source = "xgboost"
            cheat_type = xgboost_label
            confidence = xgboost_confidence
        else:
            model_source = "autoencoder"
            cheat_type = "unknown"
            confidence = self._anomaly_to_confidence(anomaly_score)

        cooldown_key = f"{player_id}:{cheat_type}"
        last_time = self.last_alert_time.get(cooldown_key, 0)
        if timestamp - last_time < self.cooldown_seconds:
            return None

        self.last_alert_time[cooldown_key] = timestamp

        return {
            "ts": int(timestamp),
            "player_id": player_id,
            "source": "ml-model",
            "model": model_source,
            "cheat_type": cheat_type,
            "confidence": round(confidence, 4),
            "anomaly_score": round(anomaly_score, 4),
            "feature_importances": feature_importances,
        }

    def _anomaly_to_confidence(self, score: float) -> float:
        if self.anomaly_std < 1e-12:
            return 0.0
        std_deviations = (score - self.anomaly_mean) / self.anomaly_std
        confidence = min(1.0, max(0.0, std_deviations / 5.0))
        return confidence
