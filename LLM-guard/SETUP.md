# LLM-Guard — Setup & Run Procedure
## 1. Prerequisites — What to Install

| Tool | Purpose | Where to get it |
|---|---|---|
| **Go** (1.21+) | Runs the reverse proxy | https://go.dev/dl/ |
| **Python** (3.10+) | Runs the DLP service | https://www.python.org/downloads/ |
| **Ollama** | Serves the local LLM | https://ollama.com/download |
| **Git** (if not already installed) | To pull/push the repo | https://git-scm.com/downloads |

After installing Ollama, pull the models this project uses:
```powershell
ollama pull llama3.2:3b
ollama pull llama3.1:8b
```

---

## 2. One-Time Project Setup

**A — Go dependencies** (from the `proxy` folder):
```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go mod download
```

**B — Python environment** (from the `dlp-service` folder):
```powershell
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
python -m spacy download en_core_web_md
```
The `spacy download` step is required separately — it's the NLP model
the DLP service uses and isn't installed automatically by `pip install`.

---

## 3. Starting the Project (Every Time)

Three terminals, in this order:

**Terminal 1 — Ollama** (if not already running as a background service):
```powershell
ollama serve
```

**Terminal 2 — DLP service:**
```powershell
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
.venv\Scripts\activate
uvicorn app:app --reload --port 9100
```
(Binds to `127.0.0.1` by default — matches `config.yaml`'s
`dlp.base_url: http://127.0.0.1:9100`, so no extra `--host` flag needed.)

**Terminal 3 — Proxy:**
```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go run ./cmd/proxy
```

**Confirm it's up:**
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:9100/health" -Method Get
```
Should return a healthy status. The proxy terminal should show:
```
LLM-Guard proxy listening on :8080 -> upstream[llama] http://localhost:11434
```

---

## 4. Configuration

Located at `proxy/config.yaml`. Key setting to know:

| Setting | Purpose |
|---|---|
| `dlp.enabled` | `true` = DLP redaction active; `false` = proxy passes requests through untouched |
| `dlp.base_url` | Where the proxy reaches the DLP service — should stay `http://127.0.0.1:9100` |

---

## 5. Quick Test

Send one request through the full stack:
```powershell
$body = @{
    model    = "llama3.2:3b"
    messages = @(@{ role = "user"; content = "Say hello in one short sentence." })
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://127.0.0.1:8080/v1/chat/completions" -Method Post -Body $body -ContentType "application/json"
```
A normal AI response back confirms the full pipeline (proxy → DLP → AI
model → back) is working end to end.

---
## 6. Running Automated Tests

**DLP Redaction Test**
```powershell
# tests/test_redact.ps1 — DLP redaction test
$body = @{
    text = "My email is testuser@gmail.com, SSN is 402-15-3847, card is 4242 4242 4242 4242, key is sk-abcdefghijklmnopqrstuvwx1234"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "http://localhost:9100/redact" -Method Post -Body $body -ContentType "application/json"

Write-Host "Redacted: " $response.redacted_text
```

**Go Integration Tests**
```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go test ./internal/dlp/... -v
```
Should show 5 tests passing. No need to have any of the 3 services above
running for this — it's fully self-contained.

---

## 7. Shutting Down

`Ctrl+C` in each of the three terminals, in any order.