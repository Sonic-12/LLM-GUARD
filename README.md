<div align="center">

# 🛡️ LLM-Guard

**Generative AI Prompt Firewall**

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-3.10%2B-3776AB?logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-services-009688?logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![Keycloak](https://img.shields.io/badge/Keycloak-OIDC%2FRBAC-000000?logo=keycloak&logoColor=white)](https://www.keycloak.org/)
<br>


<p align="center">
  <img src="LLM-guard/documentation/photos/LLM-GUARD.png" alt="API-Sentinel Dashboard" width="850">
</p>
</div>

---

## Overview

- **LLM-Guard** is a reverse proxy that sits directly in front of a locally-served LLM and enforces
  authentication, threat detection, data loss prevention, and output validation on every request and
  response, with no change required on the model-serving side.
- **The issue:** once an LLM is wired into an application or internal system, it becomes a new attack
  surface. Prompt injection, jailbreaking, and adversarial inputs can manipulate a model into ignoring
  its instructions or disclosing data it was never meant to return, and conventional security tooling
  isn't built to parse natural language, so none of it catches this.
- **Why it matters:** because of this gap, LLM-Guard enforces at a single point in front of the model —
  authenticating the caller, scoring prompts for jailbreak intent, masking sensitive data, and checking
  the model's own reply — and blocks and logs anything that fails a check instead of letting it through.

## Features

- Token-based authentication and per-role access control (RBAC) via Keycloak/OIDC
- Deterministic prompt rules: length limits and pattern blocklist, with Unicode normalization against obfuscated input
- ML-based jailbreak classification with tunable sensitivity thresholds
- Data loss prevention: detects and masks sensitive values before they reach the model, restores them on the way back
- Prompt hardening against instruction-override and injection attempts
- Output validation: toxicity checks, output data-leak re-scan, hallucination-signal flagging
- Structured audit logging with a live security console

## Architecture

```mermaid
flowchart TB
    %% Node Definitions
    Client["Client"]
    RBAC["RBAC"]
    Firewall["Firewall"]
    ThreatDetect["Threat Detection"]
    DLPIn["3. DLP - Mask<br/><i>PII Masking</i>"]
    Hardening["Prompt Hardening"]
    LLM["4. Language Model<br/><i>(LLM)</i>"]
    OutputValidation["Output Validation"]
    DLPOut["DLP - Restore"]
    Response["Final Response"]

    %% Connections
    Client --> RBAC
    RBAC --> Firewall
    Firewall --> ThreatDetect
    ThreatDetect --> DLPIn
    DLPIn --> Hardening
    Hardening --> LLM
    LLM --> OutputValidation
    OutputValidation --> DLPOut
    DLPOut --> Response

    %% Color Styles (Matching Image Palette)
    style Client fill:#eef2ff,stroke:#2563eb,stroke-width:2px,color:#1e3a8a
    style RBAC fill:#eef2ff,stroke:#2563eb,stroke-width:2px,color:#1e3a8a
    style Firewall fill:#eef2ff,stroke:#2563eb,stroke-width:2px,color:#1e3a8a
    style ThreatDetect fill:#eef2ff,stroke:#2563eb,stroke-width:2px,color:#1e3a8a
    style DLPIn fill:#f0f7ff,stroke:#2563eb,stroke-width:2px,color:#0f172a
    style Hardening fill:#eef2ff,stroke:#2563eb,stroke-width:2px,color:#1e3a8a
    style LLM fill:#fffbeb,stroke:#d97706,stroke-width:3px,color:#451a03
    style OutputValidation fill:#fdf4ff,stroke:#c026d3,stroke-width:2px,color:#701a75
    style DLPOut fill:#fdf4ff,stroke:#c026d3,stroke-width:2px,color:#701a75
    style Response fill:#f0fdf4,stroke:#16a34a,stroke-width:2px,color:#14532d
```

RBAC gates every other stage — an unauthenticated or unauthorized request never reaches the
firewall, the model, or any downstream check. Telemetry is not part of the sequential pipeline;
every enforcement point reports its decision to it independently. Each stage is documented fully
in [`Architecture`](LLM-guard/documentation/Architecture.md).

## Dashboard
<table>
<tr>
<td width="50%"><img src="LLM-guard/documentation/photos/Threat.png" alt="Threat"></td>
<td width="50%"><img src="LLM-guard/documentation/photos/Firewall.png" alt="Firewall"></td>
</tr>
<tr>
<td width="50%"><img src="LLM-guard/documentation/photos/Access.png" alt="Access"></td>
<td width="50%"><img src="LLM-guard/documentation/photos/Overview.png" alt="Overview"></td>
</tr>
</table>
- Blocked requests over time, severity mix, and breakdowns by guardrail layer and by rule
- Users with the most blocks, and the latest events as they happen
- Jailbreak score distribution and the firewall rules most frequently triggered
- Access-control (RBAC) denials and output-guard rule triggers

## Project Structure

Full setup, including prerequisites, dependency installation, and the run sequence across all
services, is documented in [`Setup`](LLM-guard/documentation/SETUP.md).

```
LLM-guard/
├── proxy/                    # Go reverse proxy
│   ├── cmd/proxy/             # entry point
│   └── internal/
│       ├── proxy/             # HTTP server, model routing
│       ├── middleware/        # hook chain definition
│       ├── rbac/              # token verification, role and model gating
│       ├── rules/              # length check, blocklist, normalization
│       ├── threatdetect/      # client for the jailbreak classifier
│       ├── dlp/                # client for the DLP service
│       ├── hardening/         # system-prompt defense
│       ├── outputguard/       # output toxicity, leak, and hallucination checks
│       ├── telemetry/         # audit log writer and sidecar forwarder
│       └── config/             # configuration loading
├── dlp-service/               # Presidio-based PII masking service
├── services/firewall/         # jailbreak classifier service
├── sidecar/analytics/         # audit store and dashboard
├── tests/                     # test runner and verification scripts
└── documentation/             # design docs
```

## Documentation

- [`Architecture`](LLM-guard/documentation/Architecture.md): system design, components, and trust boundaries
- [`Setup`](LLM-guard/documentation/SETUP.md): installation, configuration, running, and testing
- [`Security`](LLM-guard/documentation/security.md): security controls, authentication, and threat handling

## License
![License](https://img.shields.io/badge/License-Proprietary-red)
 
This project is **proprietary** and not open source. It was developed as part of the
**Axlero Solutions Internship Program**. All rights are reserved.