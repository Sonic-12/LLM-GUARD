"""Custom Presidio recognizers not covered by the built-in analyzer."""

from presidio_analyzer import Pattern, PatternRecognizer

API_KEY_PATTERNS = [
    Pattern(name="openai_key", regex=r"\bsk-[A-Za-z0-9]{20,}\b", score=0.9),
    Pattern(name="aws_access_key", regex=r"\bAKIA[0-9A-Z]{16}\b", score=0.9),
    Pattern(name="generic_bearer_token", regex=r"\b[A-Za-z0-9_\-]{32,}\b", score=0.4),
]

api_key_recognizer = PatternRecognizer(
    supported_entity="API_KEY",
    patterns=API_KEY_PATTERNS,
    context=["key", "token", "secret", "api", "credential"],
)