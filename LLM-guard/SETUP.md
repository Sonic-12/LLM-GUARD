# LLM-Guard — Setup & Run Procedure

## 1. Prerequisites — What to Install

| Tool | Purpose | Where to get it |
|---|---|---|
| **Go** (1.21+) | Runs the reverse proxy | https://go.dev/dl/ |
| **Python** (3.10+) | Runs the DLP, firewall, and analytics sidecar services | https://www.python.org/downloads/ |
| **Ollama** | Serves the local LLM | https://ollama.com/download |
| **Docker Desktop** | Runs Keycloak (the Identity Provider) | https://www.docker.com/products/docker-desktop/ |
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

**B — Python environment for the DLP service** (from the `dlp-service` folder):
```powershell
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
python -m spacy download en_core_web_md
```
The `spacy download` step is required separately — it's the NLP model
the DLP service uses and isn't installed automatically by `pip install`.

**C — Python environment for the firewall/classifier service** (from the repo root):
```powershell
cd D:\project\LLM-GUARD\LLM-guard
python -m venv services\firewall\.venv
.\services\firewall\.venv\Scripts\activate
pip install -r services\firewall\requirements.txt
```

**D — Keycloak (Identity Provider) — one-time container + realm setup**

1. Run the container:
   ```powershell
   docker run -d --name keycloak -p 8081:8080 -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin123 quay.io/keycloak/keycloak:latest start-dev
   ```
2. Open http://localhost:8081 → **Administration Console** → log in `admin` / `admin123`.
3. Top-left dropdown → **Create Realm** → name it `llmguard` → Create.
4. **Realm roles** → **Create role** → create `admin`, `employee`, `guest` (one at a time).
5. **Clients** → **Create client** → Client ID `llmguard-proxy` → Next → turn ON **Client authentication** → Next → turn ON **Direct access grants** → Save.
6. **Clients → llmguard-proxy → Credentials** tab → copy the **Client secret**. You'll need it every time you fetch a token (Section 5).
7. **Users** → **Add user** → repeat for `admin-user`, `employee-user`, `guest-user`:
   - Set username → Create.
   - **Details** tab → fill in Email/First name/Last name (avoids a Keycloak "account not fully set up" error) → Save.
   - **Credentials** tab → Set password (e.g. `Pass123`) → **Temporary: OFF** → Save.
   - **Role mapping** tab → Assign role → pick the matching role (`admin`/`employee`/`guest`).

This is one-time setup — the container and its realm persist across restarts as long as you don't delete the container.

**E — Python environment for the analytics sidecar (Week 4)** (from the repo root):
```powershell
cd D:\project\LLM-GUARD\LLM-guard
python -m venv sidecar\analytics\.venv
.\sidecar\analytics\.venv\Scripts\activate
pip install -r sidecar\analytics\requirements.txt
```

---

## 3. Starting the Project (Every Time)

Six terminals, in this order:

**Terminal 0 — Keycloak** (only if the container isn't already running):
```powershell
docker start keycloak
docker ps
```
Confirm it shows as `Up`, then check http://localhost:8081 loads. If you never stop the container, you can skip this step entirely.

**Terminal 1 — Ollama** (skip if already running as a background service):
```powershell
ollama serve
```

**Terminal 2 — DLP service:**
```powershell
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
.venv\Scripts\activate
uvicorn app:app --reload --port 9100
```
(Binds to `127.0.0.1` by default — matches `config.yaml`'s `dlp.base_url: http://127.0.0.1:9100`.)

**Terminal 3 — Firewall service (jailbreak classifier):**
```powershell
cd D:\project\LLM-GUARD\LLM-guard
.\services\firewall\.venv\Scripts\activate
uvicorn services.firewall.app:app --host 127.0.0.1 --port 5001
```
Expected: `Application startup complete.`

**Terminal 4 — Analytics sidecar (Week 4):**
```powershell
cd D:\project\LLM-GUARD\LLM-guard
.\sidecar\analytics\.venv\Scripts\activate
cd sidecar\analytics
uvicorn app:app --port 9200
```
Expected: `Application startup complete.`

**Terminal 5 — Proxy:**
```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go run ./cmd/proxy
```
Expected: `LLM-Guard proxy listening on :8080 -> upstream[llama] http://localhost:11434`

**Confirm everything's up:**
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:9100/health" -Method Get
Invoke-RestMethod -Uri "http://127.0.0.1:9200/api/v1/telemetry/metrics" -Method Get
```
Both should return without error.

**Dashboard (Week 4):** open `sidecar\analytics\dashboard.html` directly in your browser — no server needed for the page itself, it talks to the sidecar at `127.0.0.1:9200` automatically.

---

## 4. Configuration

Located at `proxy/config.yaml`. Key settings to know:

| Setting | Purpose |
|---|---|
| `dlp.enabled` | `true` = DLP redaction active; `false` = proxy passes requests through untouched |
| `dlp.base_url` | Where the proxy reaches the DLP service — should stay `http://127.0.0.1:9100` |
| `rules.enabled` | `true` = firewall/jailbreak checks active |
| `rules.firewall_url` | Where the proxy reaches the firewall service — should stay `http://127.0.0.1:5001` |
| `rbac.enabled` | `true` = every request requires a valid Keycloak token |
| `rbac.issuer_url` | Your Keycloak realm's issuer, e.g. `http://localhost:8081/realms/llmguard` |
| `rbac.jwks_url` | Keycloak's public-key endpoint for that realm |
| `rbac.client_id` | Must match the client ID created in Section 2D (`llmguard-proxy`) |
| `outputguard.enabled` | `true` = the AI's replies are checked for toxicity/leaks/hallucination signals |
| `outputguard.test_mode` | **Must stay `false`.** Dev-only bypass used during Week 3 testing; leave off. |
| `telemetry.enabled` | `true` = every blocked request is logged (Week 4) |
| `telemetry.local_log_path` | Local JSONL audit log — should stay `logs/telemetry_events.jsonl` |
| `telemetry.sidecar_url` | Where the proxy forwards events for the dashboard — should stay `http://127.0.0.1:9200`. Leave empty to log locally only, with no forwarding. |

**Important:** with `rbac.enabled: true`, every request needs a bearer token (Section 5). If you want to test something without RBAC in the way (e.g. an old script that doesn't send a token), temporarily set `rbac.enabled: false`, restart the proxy, test, then set it back to `true`.

---

## 5. Getting a Token (Required for Every Request Now)

Since RBAC is enabled, every request to the proxy needs a bearer token from Keycloak. Fetch one per role as needed:

```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy

$adminToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=YOUR_CLIENT_SECRET" -d "grant_type=password" -d "username=admin-user" -d "password=Pass123" | ConvertFrom-Json).access_token

$employeeToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=YOUR_CLIENT_SECRET" -d "grant_type=password" -d "username=employee-user" -d "password=Pass123" | ConvertFrom-Json).access_token

$guestToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=YOUR_CLIENT_SECRET" -d "grant_type=password" -d "username=guest-user" -d "password=Pass123" | ConvertFrom-Json).access_token
```
Replace `YOUR_CLIENT_SECRET` with the value from Section 2D, step 6.

**Tokens expire after 5 minutes** — if a request suddenly starts returning `401 TOKEN_EXPIRED`, just re-run the relevant line above.

---

## 6. Quick Test

Send one request through the full stack (using the admin token from Section 5):
```powershell
$body = @{
    model    = "llama3.2:3b"
    messages = @(@{ role = "user"; content = "Say hello in one short sentence." })
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://127.0.0.1:8080/v1/chat/completions" -Method Post -Body $body -ContentType "application/json" -Headers @{ Authorization = "Bearer $adminToken" }
```
A normal AI response back confirms the full pipeline (proxy → RBAC → rules/firewall → DLP → AI model → output validation → back) is working end to end.

---

## 7. Running Automated Tests

**Full Go test suite** (all packages, no external services required):
```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go build ./... ; go vet ./... ; go test ./... -v
```
Should show all packages `ok`, no `FAIL`. As of the last verified run, this is **6 packages**: `dlp`, `hardening`, `outputguard`, `rbac`, `rules`, and `telemetry` (new in Week 4). Fully self-contained — none of the other services (Ollama, DLP, firewall, Keycloak, sidecar) need to be running for this.

**Analytics sidecar test suite** (also self-contained, no proxy/services needed):
```powershell
cd D:\project\LLM-GUARD\LLM-guard\sidecar\analytics
pytest test_analytics.py -v
```
Should show **3/3 passing**.

**Manual live checks** (RBAC + Output Validation, against the real running stack): see the fixture files in `proxy/testdata/manual/` — `default.json`, `premium.json`, `longprompt.json`, `hallucination_test.json`. Example:
```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $guestToken" -d "@testdata\manual\premium.json"
```
Expect `403 MODEL_NOT_ALLOWED_FOR_ROLE` for this one (guest requesting the premium model).

**Telemetry + dashboard check (Week 4):** after triggering any block above, confirm it was logged two ways:
```powershell
Get-Content logs\telemetry_events.jsonl -Tail 3
curl.exe -s "http://127.0.0.1:9200/api/v1/telemetry/events?limit=5"
```
Both should show the event, with a `source` field of `rbac`, `rules`, or `outputguard` depending on which hook blocked it. Then open `dashboard.html` and confirm the same event appears in the table, and the stat cards reflect it.

---

## 8. Shutting Down

`Ctrl+C` in each of the five foreground terminals (Ollama, DLP, firewall, sidecar, proxy), in any order.

Keycloak keeps running in the background as a Docker container even after you close its terminal. To actually stop it:
```powershell
docker stop keycloak
```
(Use `docker start keycloak` next time, per Section 3, Terminal 0 — no need to redo the realm/client/user setup, it persists in the container.)