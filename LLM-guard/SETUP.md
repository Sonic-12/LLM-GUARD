# LLM-Guard Setup

## What you need

- Go 1.21+
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
cd D:\project\LLM-GUARD\LLM-guard\proxy
go mod download
```

DLP service:

```powershell
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
python -m spacy download en_core_web_md
```

The spacy model has to be downloaded separately, pip install alone won't grab it.

Firewall service:

```powershell
cd D:\project\LLM-GUARD\LLM-guard
python -m venv services\firewall\.venv
.\services\firewall\.venv\Scripts\activate
pip install -r services\firewall\requirements.txt
```

Analytics sidecar:

```powershell
cd D:\project\LLM-GUARD\LLM-guard
python -m venv sidecar\analytics\.venv
.\sidecar\analytics\.venv\Scripts\activate
pip install -r sidecar\analytics\requirements.txt
```

### Keycloak

Keycloak handles login and roles. Run it once:

```powershell
docker run -d --name keycloak -p 8081:8080 -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin123 quay.io/keycloak/keycloak:latest start-dev
```

Then open [localhost:8081](http://localhost:8081), log into the admin console with admin/admin123, and set it up:

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
cd D:\project\LLM-GUARD\LLM-guard\dlp-service
.venv\Scripts\activate
uvicorn app:app --reload --port 9100
```

**Terminal 2 — Firewall**

```powershell
cd D:\project\LLM-GUARD\LLM-guard
.\services\firewall\.venv\Scripts\activate
uvicorn services.firewall.app:app --host 127.0.0.1 --port 5001
```

**Terminal 3 — Analytics sidecar**

```powershell
cd D:\project\LLM-GUARD\LLM-guard
.\sidecar\analytics\.venv\Scripts\activate
cd sidecar\analytics
uvicorn app:app --port 9200
```

**Terminal 4 — Proxy**

```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go run ./cmd/proxy
```

You should see it say it's listening on port 8080.


## Getting tokens

```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy

$secret = "YOUR_CLIENT_SECRET"

$adminToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=admin-user" -d "password=Pass123" | ConvertFrom-Json).access_token
$employeeToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=employee-user" -d "password=Pass123" | ConvertFrom-Json).access_token
$guestToken = (curl.exe -s -X POST "http://localhost:8081/realms/llmguard/protocol/openid-connect/token" -H "Content-Type: application/x-www-form-urlencoded" -d "client_id=llmguard-proxy" -d "client_secret=$secret" -d "grant_type=password" -d "username=guest-user" -d "password=Pass123" | ConvertFrom-Json).access_token
Write-Host "All 3 tokens fetched."
```

Replace YOUR_CLIENT_SECRET with the value from Keycloak's client credentials tab.

Tokens expire after 5 minutes. If a request suddenly starts returning 401, just run the script again.

## Quick test

```powershell
$body = @{
    model    = "llama3.2:3b"
    messages = @(@{ role = "user"; content = "Say hello in one short sentence." })
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://127.0.0.1:8080/v1/chat/completions" -Method Post -Body $body -ContentType "application/json" -Headers @{ Authorization = "Bearer $adminToken" }
```

A normal reply back means the whole chain worked.

## Running the tests

Go side, no other services need to be running for this:

```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
go build ./... ; go vet ./... ; go test ./... -v
```

Python side:

```powershell
cd D:\project\LLM-GUARD\LLM-guard
.\sidecar\analytics\.venv\Scripts\activate
cd sidecar\analytics
pytest test_analytics.py -v
```

## Checking the live stack

There are sample request files in proxy\testdata\manual. Run through all of them with the tokens from earlier.

```powershell
cd D:\project\LLM-GUARD\LLM-guard\proxy
```

Guest asking for the normal model, should go through fine:

```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $guestToken" -d "@testdata\manual\default.json"
```

Guest asking for the premium model, should get blocked:

```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $guestToken" -d "@testdata\manual\premium.json"
```

Guest sending a prompt that's too long, should also get blocked:

```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $guestToken" -d "@testdata\manual\longprompt.json"
```

Admin asking something that could trigger the hallucination check, should go through, and may or may not get flagged in the log depending on how the model answers:

```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -H "Authorization: Bearer $adminToken" -d "@testdata\manual\hallucination_test.json"
```

No token at all, should get rejected before any of this even runs:

```powershell
curl.exe -i -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -d "@testdata\manual\default.json"
```

After running these, check that the blocked ones actually got logged in dashboard

## Shutting down

Ctrl+C in each terminal. Keycloak keeps running in the background even after you close its window, so if you actually want it stopped:

```powershell
docker stop keycloak
```