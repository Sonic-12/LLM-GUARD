import sqlite3
import os
from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime, timezone

DB_PATH = os.getenv("AUDIT_DB_PATH", "audit_logs.db")
DASHBOARD_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dashboard.html")

app = FastAPI(title="LLM-GUARD SIEM & Analytics API", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=False, 
    allow_methods=["*"],
    allow_headers=["*"],
)

def init_db():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS audit_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL,
            event_id TEXT UNIQUE NOT NULL,
            user_id TEXT NOT NULL,
            client_ip TEXT NOT NULL,
            source TEXT,
            prompt_length INTEGER NOT NULL,
            allowed BOOLEAN NOT NULL,
            status_code INTEGER NOT NULL,
            reason TEXT,
            rule_triggered TEXT,
            jailbreak_score REAL,
            prompt_sample TEXT
        )
    """)
    try:
        cursor.execute("ALTER TABLE audit_events ADD COLUMN source TEXT")
        conn.commit()
    except sqlite3.OperationalError:
        pass  # column already exists
    conn.commit()
    conn.close()

init_db()

class AuditEventSchema(BaseModel):
    timestamp: Optional[str] = None
    event_id: str
    user_id: str
    client_ip: str
    source: Optional[str] = None  
    prompt_length: int
    allowed: bool
    status_code: int
    reason: Optional[str] = None
    rule_triggered: Optional[str] = None
    jailbreak_score: Optional[float] = 0.0
    prompt_sample: str

@app.get("/dashboard", include_in_schema=False)
def dashboard():
    """Serve the security console at http://localhost:9200/dashboard"""
    if not os.path.exists(DASHBOARD_PATH):
        raise HTTPException(status_code=404, detail="dashboard.html not found next to app.py")
    return FileResponse(DASHBOARD_PATH, media_type="text/html")


@app.post("/api/v1/telemetry/ingest")
def ingest_event(event: AuditEventSchema):
    ts = event.timestamp or datetime.now(timezone.utc).isoformat()
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        cursor.execute("""
            INSERT INTO audit_events 
            (timestamp, event_id, user_id, client_ip, source, prompt_length, allowed, status_code, reason, rule_triggered, jailbreak_score, prompt_sample)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            ts, event.event_id, event.user_id, event.client_ip, event.source,
            event.prompt_length, event.allowed, event.status_code,
            event.reason, event.rule_triggered, event.jailbreak_score,
            event.prompt_sample
        ))
        conn.commit()
        conn.close()
        return {"status": "success", "event_id": event.event_id}
    except sqlite3.IntegrityError:
        raise HTTPException(status_code=400, detail="Event ID already ingested")
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/v1/telemetry/events")
def query_events(
    limit: int = Query(50, le=500),
    allowed: Optional[bool] = None,
    user_id: Optional[str] = None
):
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()

    query = "SELECT * FROM audit_events WHERE 1=1"
    params = []

    if allowed is not None:
        query += " AND allowed = ?"
        params.append(allowed)
    if user_id is not None:
        query += " AND user_id = ?"
        params.append(user_id)

    query += " ORDER BY id DESC LIMIT ?"
    params.append(limit)

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()

    results = []
    for row in rows:
        r = dict(row)
        r["allowed"] = bool(r["allowed"])
        results.append(r)
    return results

@app.get("/api/v1/telemetry/metrics")
def get_metrics():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    cursor.execute("SELECT COUNT(*) FROM audit_events")
    total_requests = cursor.fetchone()[0] or 0

    cursor.execute("SELECT COUNT(*) FROM audit_events WHERE allowed = 0")
    total_blocked = cursor.fetchone()[0] or 0

    cursor.execute("SELECT COUNT(*) FROM audit_events WHERE allowed = 1")
    total_allowed = cursor.fetchone()[0] or 0

    block_rate = (total_blocked / total_requests * 100) if total_requests > 0 else 0.0

    conn.close()

    return {
        "total_requests": total_requests,
        "total_blocked": total_blocked,
        "total_allowed": total_allowed,
        "block_rate_percentage": round(block_rate, 2)
    }