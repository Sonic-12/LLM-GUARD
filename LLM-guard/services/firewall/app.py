from fastapi import FastAPI
from pydantic import BaseModel
from typing import Optional
from services.firewall.classifier import predict_jailbreak

app = FastAPI(title="LLM-Guard Semantic Firewall Service")

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