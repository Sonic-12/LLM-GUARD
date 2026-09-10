import os
import joblib
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline
from services.firewall.data_loader import load_dataset

ARTIFACTS_DIR = os.path.join(os.path.dirname(__file__), "artifacts")
MODEL_PATH = os.path.join(ARTIFACTS_DIR, "jailbreak_model.joblib")

def train_and_export_model(export_path: str = MODEL_PATH) -> str:
    """Trains a lightweight semantic classifier and exports the model pipeline."""
    os.makedirs(os.path.dirname(export_path), exist_ok=True)

    # 1. Ingest datasets
    texts, labels = load_dataset()

    # 2. Build pipeline (TF-IDF N-grams + Logistic Regression)
    pipeline = Pipeline([
        ("tfidf", TfidfVectorizer(ngram_range=(1, 2), lowercase=True)),
        ("clf", LogisticRegression(solver="liblinear", random_state=42))
    ])

    # 3. Fit model
    pipeline.fit(texts, labels)

    # 4. Serialize artifacts
    joblib.dump(pipeline, export_path)
    print(f"[+] Model trained on {len(texts)} samples and exported to: {export_path}")
    return export_path

if __name__ == "__main__":
    train_and_export_model()
