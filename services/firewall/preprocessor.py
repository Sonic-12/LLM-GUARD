import re
from typing import Dict, Any

ADVERSARIAL_MARKERS = [
    r"\bignore\b", r"\bdisregard\b", r"\boverride\b", r"\bdan\b",
    r"\bunrestricted\b", r"\bdeveloper mode\b", r"\bsystem prompt\b",
    r"\bpretend\b", r"\brogue\b", r"\bevilbot\b"
]

def clean_text(text: str) -> str:
    """Normalizes prompt text for consistent tokenization."""
    text = text.lower().strip()
    text = re.sub(r"\s+", " ", text)
    return text

def extract_features(text: str) -> Dict[str, Any]:
    """
    Extracts structural and semantic indicators of prompt injection attempts.
    """
    normalized = clean_text(text)
    words = normalized.split()
    word_count = len(words)

    marker_hits = sum(
        1 for pattern in ADVERSARIAL_MARKERS if re.search(pattern, normalized)
    )

    # Heuristic ratio of injection markers to total token count
    marker_density = marker_hits / max(word_count, 1)

    return {
        "cleaned_text": normalized,
        "word_count": word_count,
        "marker_hits": marker_hits,
        "marker_density": marker_density,
        "has_adversarial_marker": marker_hits > 0,
    }
