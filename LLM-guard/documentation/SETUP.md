# LLM-Guard Setup
  > See also: [README](../../README.md) · [Architecture](Architecture.md) · [Security Documentation](security.md)
## What you need

- Go 1.22+
- Python 3.10+
- Ollama
- Docker Desktop (for Keycloak)

Pull the models once Ollama is installed:

```powershell
ollama pull llama3.2:3b
ollama pull llama3.1:8b
```

## First time setup

Go dependencies:

```powershell
cd proxy
go mod download
cd ..
```

DLP service:

```powershell
cd dlp-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
python -m spacy download en_core_web_md
cd ..
```

The spacy model has to be downloaded separately, pip install alone won't grab it.

Firewall service:

```powershell
python -m venv services\firewall\.venv
.\services\firewall\.venv\Scripts\activate
pip install -r services\firewall\requirements.txt
New-Item services\__init__.py, services\firewall\__init__.py -ItemType File -Force
```

Analytics sidecar:

```powershell
python -m venv sidecar\analytics\.venv
.\sidecar\analytics\.venv\Scripts\activate
pip install -r sidecar\analytics\requirements.txt
```

### Keycloak

Keycloak handles login and roles. Run it once:

```powershell
docker run -d --name keycloak -p 8081:8080 -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin123 quay.io/keycloak/keycloak:latest start-dev
```

Then open [Keycloak](http://localhost:8081), log into the admin console with Username: admin, Password: admin123, and set it up:

1. Create a realm called llmguard
2. Under Realm roles, create admin, employee and guest
3. Under Clients, create a client with ID llmguard-proxy, turn on Client authentication and Direct access grants
4. Go to that client's Credentials tab and copy the secret, you'll need it to get tokens
5. Create three users: admin-user, employee-user, guest-user. For each one, fill in email/first name/last name (otherwise Keycloak complains the account isn't fully set up), set a password with Temporary turned off, and assign the matching role

This is one time setup. As long as you don't delete the container, it's all still there next time.

## Running it

Keycloak, only if it's not already running:

```powershell
docker start keycloak
```

Ollama:

```powershell
ollama serve
```

**Terminal 1 — DLP**

```powershell
cd dlp-service
.venv\Scripts\activate
uvicorn app:app --reload --port 9100
```

**Terminal 2 — Firewall**

```powershell
.\services\firewall\.venv\Scripts\activate
uvicorn services.firewall.app:app --host 127.0.0.1 --port 5001
```

**Terminal 3 — Analytics sidecar**

```powershell
.\sidecar\analytics\.venv\Scripts\activate
cd sidecar\analytics
uvicorn app:app --port 9200
```

**Terminal 4 — Proxy**

```powershell
cd proxy
go run ./cmd/proxy
```

You should see it say it's listening on port 8080.


## Getting tokens

```powershell
cd proxy

$secret = "YOUR_CLIENT_SECRET"

$adminToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=admin-user" -d "password=Pass123" | ConvertFrom-Json).access_token
$employeeToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=employee-user" -d "password=Pass123" | ConvertFrom-Json).access_token
$guestToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=guest-user" -d "password=Pass123" | ConvertFrom-Json).access_token
Write-Host "All 3 tokens fetched."
cd ..
```

Replace YOUR_CLIENT_SECRET with the value from Keycloak's client credentials tab.

Tokens expire after 5 minutes. If a request suddenly starts returning 401, just run the script again.

## Quick test
Wait for 10-15 seconds after getting the tokens, then run this to make sure the whole chain is working:
```powershell
$body = @{
    model    = "llama3.2:3b"
    messages = @(@{ role = "user"; content = "Say hello in one short sentence." })
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://127.0.0.1:8080/v1/chat/completions" -Method Post -Body $body -ContentType "application/json" -Headers @{ Authorization = "Bearer $adminToken" }
```
>Also: If it returns:  `{"allowed":false,"status_code":503,"reason":"INSPECTION_SERVICE_UNAVAILABLE_FAIL_CLOSED"}`
retry after a few seconds.
    
A normal reply back means the whole chain worked.

## Running the tests

  Historical latency and red-team results from an earlier verification pass
  are recorded in [`Mid Project Review`](Mid-Project-Review.md).

**One script runs everything in order and stops at the first failure.**

**Before you start**

- Firewall, DLP, analytics sidecar and proxy are running (Terminals 1 to 4)
- Keycloak and Ollama are running
- Tokens are fetched in this same window (see "Getting tokens")

**Run (from the project root)**

```powershell
.\tests\run_tests.ps1
cd ..
```

**What it checks**
- Go build, vet, and tests
- Python analytics sidecar tests
- Guest normal-model access
- Guest premium-model restriction
- Prompt length validation
- Admin hallucination test
- Missing-token rejection

```powershell
$env:ADMIN_TOKEN = $adminToken
$env:GUEST_TOKEN = $guestToken
$env:ADMIN_TOKEN.Length
python tests\attack_suite.py
```
**What it checks**
- Runs the attack suite through the proxy
- Tests RBAC, firewall, ML detection, DLP, and output validation
- Verifies telemetry across security layers
- Confirms benign prompts remain allowed
- Reports pass/fail status per layer
- Returns a non-zero exit code on failure

**After it passes**

Open the [Dashboard](http://localhost:9200/dashboard) and confirm the blocked requests were logged.

## Shutting down

Ctrl+C in each terminal. Keycloak keeps running in the background even after you close its window, so if you actually want it stopped:

```powershell
docker stop keycloak
```
