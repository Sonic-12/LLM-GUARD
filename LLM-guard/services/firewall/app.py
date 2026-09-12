from fastapi import FastAPI
from pydantic import BaseModel
from typing import Optional
from services.firewall.classifier import predict_jailbreak

app = FastAPI(title="LLM-Guard Semantic Firewall Service")

class InspectionRequest(BaseModel):
    prompt: str
    threshold: Optional[float] = 0.50

class InspectionResponse(BaseModel):
    allowed: bool
    jailbreak_score: float
    reason: Optional[str] = None

@app.post("/v1/firewall/inspect", response_model=InspectionResponse)
def inspect_endpoint(req: InspectionRequest):
    is_jailbreak, score = predict_jailbreak(req.prompt, threshold=req.threshold)
    
    if is_jailbreak:
        return InspectionResponse(
            allowed=False,
            jailbreak_score=score,
            reason="SEMANTIC_JAILBREAK_DETECTED"
        )
        
    return InspectionResponse(
        allowed=True,
        jailbreak_score=score,
        reason=None
    )

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=5001)