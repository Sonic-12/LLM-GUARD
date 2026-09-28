import sqlite3
import os
from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime, timedelta

DB_PATH = os.getenv("AUDIT_DB_PATH", "audit_logs.db")

app = FastAPI(title="LLM-GUARD SIEM & Analytics API", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
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
            prompt_length INTEGER NOT NULL,
            allowed BOOLEAN NOT NULL,
            status_code INTEGER NOT NULL,
            reason TEXT,
            rule_triggered TEXT,
            jailbreak_score REAL,
            prompt_sample TEXT
        )
    """)
    conn.commit()
    conn.close()

init_db()

class AuditEventSchema(BaseModel):
    timestamp: Optional[str] = None
    event_id: str
    user_id: str
    client_ip: str
    prompt_length: int
    allowed: bool
    status_code: int
    reason: Optional[str] = None
    rule_triggered: Optional[str] = None
    jailbreak_score: float
    prompt_sample: str

@app.post("/api/v1/telemetry/ingest")
def ingest_event(event: AuditEventSchema):
    ts = event.timestamp or datetime.utcnow().isoformat() + "Z"
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        cursor.execute("""
            INSERT INTO audit_events 
            (timestamp, event_id, user_id, client_ip, prompt_length, allowed, status_code, reason, rule_triggered, jailbreak_score, prompt_sample)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            ts, event.event_id, event.user_id, event.client_ip,
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

    return [dict(row) for row in rows]

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
