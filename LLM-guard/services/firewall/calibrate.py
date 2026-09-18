import numpy as np
from sklearn.model_selection import StratifiedKFold, cross_val_predict
from services.firewall.data_loader import load_dataset
from services.firewall.train import build_pipeline
def main() -> None:
    texts, labels = load_dataset()
    labels = np.array(labels)

    pipeline = build_pipeline()
    cv = StratifiedKFold(n_splits=5, shuffle=True, random_state=42)
    oof_proba = cross_val_predict(pipeline, texts, labels, cv=cv, method="predict_proba")[:, 1]

    best_f1, best_low = 0.0, 0.5
    for t in np.arange(0.30, 0.71, 0.01):
        preds = (oof_proba >= t).astype(int)
        tp = int(((preds == 1) & (labels == 1)).sum())
        fp = int(((preds == 1) & (labels == 0)).sum())
        fn = int(((preds == 0) & (labels == 1)).sum())
        precision = tp / (tp + fp) if (tp + fp) else 0.0
        recall = tp / (tp + fn) if (tp + fn) else 0.0
        f1 = 2 * precision * recall / (precision + recall) if (precision + recall) else 0.0
        if f1 > best_f1:
            best_f1, best_low = f1, round(float(t), 2)

    best_high = min(0.95, round(best_low + 0.15, 2))

    print(f"5-fold CV out-of-fold accuracy: {((oof_proba >= best_low).astype(int) == labels).mean():.3f}")
    print(f"Best LOW_THRESHOLD (max F1={best_f1:.3f}): {best_low}")
    print(f"Recommended HIGH_THRESHOLD (LOW + 0.15, capped): {best_high}")


if __name__ == "__main__":
    main()