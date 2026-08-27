"""Builds the Presidio AnalyzerEngine with built-in + custom recognizers."""

from presidio_analyzer import AnalyzerEngine
from presidio_analyzer.nlp_engine import NlpEngineProvider
from recognizers import api_key_recognizer

ENTITIES = ["EMAIL_ADDRESS", "CREDIT_CARD", "US_SSN", "API_KEY"]

NLP_CONFIGURATION = {
    "nlp_engine_name": "spacy",
    "models": [{"lang_code": "en", "model_name": "en_core_web_md"}],
}


def build_analyzer() -> AnalyzerEngine:
    nlp_engine = NlpEngineProvider(nlp_configuration=NLP_CONFIGURATION).create_engine()
    engine = AnalyzerEngine(nlp_engine=nlp_engine)
    engine.registry.add_recognizer(api_key_recognizer)
    return engine


analyzer = build_analyzer()


def analyze(text: str):
    return analyzer.analyze(text=text, entities=ENTITIES, language="en")