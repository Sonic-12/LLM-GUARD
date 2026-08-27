# LLM-Guard: Generative AI Prompt Firewall
### Week 1 Progress Report — Proxy Core & DLP Engine

**Module owner:** Proxy Architecture & DLP (Go, Python)
**Reporting period:** Week 1, Day 1 – Day 4
**Status:** Days 1–4 complete; Days 5–7 in progress

---

## 1. Summary

This report covers Week 1 of the LLM-Guard project: the reverse proxy core, the middleware hook architecture, and the first phase of the Data Loss Prevention (DLP) engine. All four planned tasks for Days 1–4 are complete and have been validated. Days 5–7 — wiring the DLP service into the proxy, adding the token cache and `/unmask` endpoint, and establishing a latency baseline — remain open and are the immediate next steps.

---

## 2. Progress Against Plan

| Day | Task | Status |
|---|---|---|
| Day 1 | Architecture & setup — repo structure, Go/Python split decision, pipeline contract defined | Complete |
| Day 2 | Reverse proxy core — HTTP reverse proxy, config-driven target routing | Complete |
| Day 3 | Middleware hook points — pre-request/post-response hook chain, passthrough template | Complete |
| Day 4 | DLP Engine (Part 1) — Presidio-based redaction service, `/redact` endpoint | Complete |
| Day 5 | Wire DLP into proxy — pre/post hooks, token cache, `/unmask` endpoint | Pending |
| Day 6-7 | Testing & latency baseline — end-to-end tests, benchmark | Pending |

---

## 3. Work Completed

### 3.1 Architecture & Repository Setup (Day 1)
- Selected Go for the proxy core (high-throughput, low-latency) with the DLP/ML components as a separate Python sidecar service.
- Established repo structure: `proxy/` (Go), `dlp-service/` (Python), `config/`, `tests/`.
- Defined the end-to-end pipeline contract: Inbound → DLP Redact → Rules Engine → Jailbreak Classifier → LLM API → Output Validation → DLP Unmask → User.

### 3.2 Reverse Proxy Core (Day 2)
- Implemented a base HTTP reverse proxy in Go that intercepts REST calls intended for an LLM API.
- Added config-driven target routing, allowing the proxy to point at different backends without code changes.

### 3.3 Middleware Hook Chain (Day 3)
- Refactored the proxy into a pre-request / post-response middleware chain.
- Added a no-op passthrough hook as the template future modules (rules engine, classifier, output validation) register against.

### 3.4 DLP Engine — Part 1 (Day 4)
- Stood up a standalone FastAPI service (`dlp-service/`) using Microsoft Presidio, backed by spaCy's `en_core_web_md` NLP model.
- Implemented detection for four entity types: email addresses, credit card numbers, U.S. Social Security Numbers, and API keys.
- Added a custom pattern recognizer for API keys (OpenAI-style `sk-…`, AWS `AKIA…` keys, and generic bearer tokens).
- Exposed `POST /redact` (returns redacted text plus a token-to-original mapping) and `GET /health`.

---

## 4. Validation

The DLP service was exercised live against combined test input containing an email address, a Social Security Number, a credit card number, and an API key. All four entity types were correctly detected, tokenized, and mapped back to their original values in the response payload.

> One notable finding during testing: a placeholder SSN (`123-45-6789`) was correctly left unredacted. This is expected, verified behavior — Presidio's SSN recognizer intentionally excludes well-known textbook/placeholder numbers so that real deployments do not flag tutorial or sample documents. Detection was re-confirmed using a non-placeholder SSN, which was redacted correctly.

---

## 5. Next Steps

- **Day 5:** Wire the DLP service into the proxy — pre-request hook calls `/redact`, post-response hook calls a new `/unmask` endpoint, with a short-lived token↔value cache keyed by request ID.
- **Day 6–7:** End-to-end redact → unmask testing, latency baseline measurement, and an integration test suite covering pass-through, redaction accuracy, and unmask correctness.

---
*LLM-Guard — AI Security Posture Management (AI SPM) & Application Security*