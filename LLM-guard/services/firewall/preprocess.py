"""Shared text preprocessing used by both training and inference, so the
model never sees a distribution mismatch between the two."""
import re

_LETTER_SPACING = re.compile(r"\b(?:\w\s+){2,}\w\b")


def collapse_letter_spacing(text: str) -> str:
    """Collapses 'i g n o r e' -> 'ignore'. Defeats a spacing-based evasion
    that survives both the regex blocklist and char n-gram tokenization
    (single-char 'words' get boundary-padded to near-nothing by char_wb).

    Also lowercases: passing a custom `preprocessor` to TfidfVectorizer
    replaces its default preprocessing entirely, so `lowercase=True` on the
    vectorizer becomes a no-op once a custom preprocessor is set. Doing it
    here keeps train-time and inference-time behavior identical."""
    return _LETTER_SPACING.sub(lambda m: m.group(0).replace(" ", ""), text).lower()