import os
from fastapi import FastAPI, Header, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import Optional
from services.firewall.classifier import predict_jailbreak, get_thresholds, set_thresholds

app = FastAPI(title="LLM-Guard Semantic Firewall Service")
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])


class ThresholdUpdate(BaseModel):
    low: float
    high: float


@app.get("/v1/firewall/config")
def get_config():
    low, high = get_thresholds()
    return {"low_threshold": low, "high_threshold": high}


@app.post("/v1/firewall/config")
def update_config(body: ThresholdUpdate, x_admin_key: Optional[str] = Header(default=None)):
    # Optional guard: set FIREWALL_ADMIN_KEY to require a key for tuning.
    required = os.getenv("FIREWALL_ADMIN_KEY")
    if required and x_admin_key != required:
        raise HTTPException(status_code=401, detail="invalid admin key")
    try:
        low, high = set_thresholds(body.low, body.high)
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    return {"low_threshold": low, "high_threshold": high}

class InspectionRequest(BaseModel):
    prompt: str

class InspectionResponse(BaseModel):
    allowed: bool
    jailbreak_score: float
    review_required: bool
    reason: Optional[str] = None

@app.post("/v1/firewall/inspect", response_model=InspectionResponse)
def inspect_endpoint(req: InspectionRequest):
    is_jailbreak, score, review_required = predict_jailbreak(req.prompt)

    if is_jailbreak:
        reason = "SEMANTIC_JAILBREAK_DETECTED_REVIEW_REQUIRED" if review_required else "SEMANTIC_JAILBREAK_DETECTED"
        return InspectionResponse(
            allowed=False,
            jailbreak_score=score,
            review_required=review_required,
            reason=reason
        )

    return InspectionResponse(
        allowed=True,
        jailbreak_score=score,
        review_required=False,
        reason=None
    )

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=5001)