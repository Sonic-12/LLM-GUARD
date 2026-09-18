import os
import joblib
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline, FeatureUnion
from services.firewall.data_loader import load_dataset
from services.firewall.preprocess import collapse_letter_spacing

ARTIFACTS_DIR = os.path.join(os.path.dirname(__file__), "artifacts")
MODEL_PATH = os.path.join(ARTIFACTS_DIR, "jailbreak_model.joblib")


def build_pipeline() -> Pipeline:
    """Word n-grams catch known phrasing; char n-grams catch spacing/
    punctuation obfuscation that survives normalization and word-level
    tokenization ('i.g.n.o.r.e', 'ign0re')."""
    features = FeatureUnion([
        ("word", TfidfVectorizer(ngram_range=(1, 2), lowercase=True, preprocessor=collapse_letter_spacing)),
        ("char", TfidfVectorizer(analyzer="char_wb", ngram_range=(3, 5), lowercase=True, preprocessor=collapse_letter_spacing)),
    ])
    return Pipeline([
        ("features", features),
        ("clf", LogisticRegression(solver="liblinear", random_state=42, class_weight="balanced")),
    ])


def train_and_export_model(export_path: str = MODEL_PATH) -> str:
    """Trains the semantic classifier and exports the model pipeline."""
    os.makedirs(os.path.dirname(export_path), exist_ok=True)

    texts, labels = load_dataset()
    pipeline = build_pipeline()
    pipeline.fit(texts, labels)

    joblib.dump(pipeline, export_path)
    print(f"[+] Model trained on {len(texts)} samples and exported to: {export_path}")
    return export_path


if __name__ == "__main__":
    train_and_export_model()