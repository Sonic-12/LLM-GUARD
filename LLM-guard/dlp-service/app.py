"DLP service: POST /redact -> masked text + token-to-original mapping."

import uuid

from fastapi import FastAPI
from pydantic import BaseModel

from analyzer import analyze

app = FastAPI(title="LLM-Guard DLP Service")


class RedactRequest(BaseModel):
    text: str


class RedactResponse(BaseModel):
    redacted_text: str
    mapping: dict[str, str]


@app.post("/redact", response_model=RedactResponse)
def redact(req: RedactRequest) -> RedactResponse:
    results = analyze(req.text)
    results.sort(key=lambda r: r.start, reverse=True)

    text = req.text
    mapping: dict[str, str] = {}

    for r in results:
        original = text[r.start:r.end]
        token = f"[{r.entity_type}_{uuid.uuid4().hex[:8]}]"
        mapping[token] = original
        text = text[:r.start] + token + text[r.end:]

    return RedactResponse(redacted_text=text, mapping=mapping)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}