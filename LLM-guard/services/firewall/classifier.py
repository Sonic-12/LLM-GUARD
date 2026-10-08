import os
import joblib
from typing import Tuple

DEFAULT_MODEL_PATH = os.path.join(os.path.dirname(__file__), "artifacts", "jailbreak_model.joblib")
import json
import threading

LOW_THRESHOLD = 0.45
HIGH_THRESHOLD = 0.60
THRESHOLD_PATH = os.path.join(os.path.dirname(__file__), "artifacts", "thresholds.json")
_lock = threading.Lock()


def get_thresholds() -> Tuple[float, float]:
    with _lock:
        return LOW_THRESHOLD, HIGH_THRESHOLD


def set_thresholds(low: float, high: float, persist: bool = True) -> Tuple[float, float]:
    """Runtime sensitivity tuning. Lower `low` = stricter (blocks more)."""
    global LOW_THRESHOLD, HIGH_THRESHOLD
    if not (0.05 <= low <= 0.95) or not (0.05 <= high <= 0.99) or high < low:
        raise ValueError("thresholds must satisfy 0.05 <= low <= high <= 0.99")
    with _lock:
        LOW_THRESHOLD, HIGH_THRESHOLD = float(low), float(high)
        if persist:
            try:
                with open(THRESHOLD_PATH, "w") as f:
                    json.dump({"low": LOW_THRESHOLD, "high": HIGH_THRESHOLD}, f)
            except OSError:
                pass
    return LOW_THRESHOLD, HIGH_THRESHOLD


def _load_saved_thresholds():
    try:
        with open(THRESHOLD_PATH) as f:
            d = json.load(f)
        set_thresholds(d["low"], d["high"], persist=False)
    except (OSError, ValueError, KeyError):
        pass

class JailbreakDetector:
    def __init__(self, model_path: str = DEFAULT_MODEL_PATH):
        self.model_path = model_path

        if not os.path.exists(model_path):
            from services.firewall.train import train_and_export_model
            train_and_export_model(model_path)

        self.model = joblib.load(model_path)

    def predict(self, text: str) -> Tuple[bool, float, bool]:
        """
        Returns (is_jailbreak, confidence_score, review_required).
        review_required is True in the uncertain band - blocked by default
        (fail-safe for a security firewall) but flagged so a human review
        queue (Week 4 dashboard) can re-check it later.
        """
        probabilities = self.model.predict_proba([text])[0]
        score = float(probabilities[1])

        low, high = get_thresholds()
        is_jailbreak = score >= low
        review_required = low <= score < high
        return is_jailbreak, score, review_required

_load_saved_thresholds()

_detector_instance = None

def get_detector() -> JailbreakDetector:
    global _detector_instance
    if _detector_instance is None:
        _detector_instance = JailbreakDetector()
    return _detector_instance

def predict_jailbreak(text: str) -> Tuple[bool, float, bool]:
    """Functional interface for proxy integration."""
    return get_detector().predict(text)