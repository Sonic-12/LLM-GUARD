import os
import joblib
from typing import Tuple

DEFAULT_MODEL_PATH = os.path.join(os.path.dirname(__file__), "artifacts", "jailbreak_model.joblib")
LOW_THRESHOLD = 0.46
HIGH_THRESHOLD = 0.65

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

        is_jailbreak = score >= LOW_THRESHOLD
        review_required = LOW_THRESHOLD <= score < HIGH_THRESHOLD
        return is_jailbreak, score, review_required

_detector_instance = None

def get_detector() -> JailbreakDetector:
    global _detector_instance
    if _detector_instance is None:
        _detector_instance = JailbreakDetector()
    return _detector_instance

def predict_jailbreak(text: str) -> Tuple[bool, float, bool]:
    """Functional interface for proxy integration."""
    return get_detector().predict(text)