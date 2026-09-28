import os
import joblib
from typing import Tuple

DEFAULT_MODEL_PATH = os.path.join(os.path.dirname(__file__), "artifacts", "jailbreak_model.joblib")

class JailbreakDetector:
    def __init__(self, model_path: str = DEFAULT_MODEL_PATH, threshold: float = 0.50):
        self.threshold = threshold
        self.model_path = model_path
        
        if not os.path.exists(model_path):
            from services.firewall.train import train_and_export_model
            train_and_export_model(model_path)
            
        self.model = joblib.load(model_path)

    def predict(self, text: str) -> Tuple[bool, float]:
        """
        Evaluates input text.
        Returns:
            (is_jailbreak: bool, confidence_score: float)
        """
        # Predict probability for class 1 (Jailbreak / Injection)
        probabilities = self.model.predict_proba([text])[0]
        jailbreak_score = float(probabilities[1])

        is_jailbreak = jailbreak_score >= self.threshold
        return is_jailbreak, jailbreak_score

# Singleton instance for high-performance reuse
_detector_instance = None

def get_detector() -> JailbreakDetector:
    global _detector_instance
    if _detector_instance is None:
        _detector_instance = JailbreakDetector()
    return _detector_instance

def predict_jailbreak(text: str, threshold: float = 0.50) -> Tuple[bool, float]:
    """Functional interface for proxy integration."""
    detector = get_detector()
    detector.threshold = threshold
    return detector.predict(text)
