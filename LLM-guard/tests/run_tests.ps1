$guestToken = if ($env:GUEST_TOKEN) { $env:GUEST_TOKEN } else { "YOUR_GUEST_TOKEN_HERE" }
$adminToken = if ($env:ADMIN_TOKEN) { $env:ADMIN_TOKEN } else { "YOUR_ADMIN_TOKEN_HERE" }
$ErrorActionPreference = "Stop"
Write-Host "`n[1/3] Running Go Proxy Build, Vet, and Unit Tests..." -ForegroundColor Cyan
Set-Location "D:\project\LLM-GUARD\LLM-guard\proxy"

go build ./...
go vet ./...
go test ./... -v

if ($LASTEXITCODE -ne 0) {
    Write-Error "Go tests failed! Aborting."
    exit $LASTEXITCODE
}

Write-Host "`n[2/3] Running Sidecar Analytics Pytest..." -ForegroundColor Cyan
Set-Location "D:\project\LLM-GUARD\LLM-guard\sidecar\analytics"
& ".\.venv\Scripts\pytest.exe" test_analytics.py -v

if ($LASTEXITCODE -ne 0) {
    Write-Error "Pytest failed! Aborting."
    exit $LASTEXITCODE
}

Write-Host "`n[3/3] Executing Proxy API Requests..." -ForegroundColor Cyan
Set-Location "D:\project\LLM-GUARD\LLM-guard\proxy"

Write-Host "`n---> Request 1: Default (Guest)" -ForegroundColor Yellow
curl.exe -i -X POST http://localhost:8080/v1/chat/completions `
    -H "Content-Type: application/json" `
    -H "Authorization: Bearer $guestToken" `
    -d "@testdata\manual\default.json"

Write-Host "`n---> Request 2: Premium (Guest)" -ForegroundColor Yellow
curl.exe -i -X POST http://localhost:8080/v1/chat/completions `
    -H "Content-Type: application/json" `
    -H "Authorization: Bearer $guestToken" `
    -d "@testdata\manual\premium.json"

Write-Host "`n---> Request 3: Long Prompt (Guest)" -ForegroundColor Yellow
curl.exe -i -X POST http://localhost:8080/v1/chat/completions `
    -H "Content-Type: application/json" `
    -H "Authorization: Bearer $guestToken" `
    -d "@testdata\manual\longprompt.json"

Write-Host "`n---> Request 4: Hallucination Test (Admin)" -ForegroundColor Yellow
curl.exe -i -X POST http://localhost:8080/v1/chat/completions `
    -H "Content-Type: application/json" `
    -H "Authorization: Bearer $adminToken" `
    -d "@testdata\manual\hallucination_test.json"

Write-Host "`n---> Request 5: Default (Unauthenticated)" -ForegroundColor Yellow
curl.exe -i -X POST http://localhost:8080/v1/chat/completions `
    -H "Content-Type: application/json" `
    -d "@testdata\manual\default.json"

Write-Host "`n==========================================" -ForegroundColor Green
Write-Host " All steps executed successfully!" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green