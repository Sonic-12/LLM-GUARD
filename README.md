# LLM-Guard

**LLM-Guard** is a Generative AI Prompt Firewall designed to secure applications that interact with Large Language Models (LLMs).

It operates as a security layer between an application and the LLM API, inspecting prompts before they reach the model. The system combines deterministic firewall rules, Data Loss Prevention (DLP), and AI-based threat detection to identify and prevent common LLM security threats.

## Overview

As LLMs become integrated into enterprise applications, they introduce security risks including:

* Prompt Injection
* Jailbreak Attempts
* Sensitive Data Exposure
* Malicious Instructions
* Accidental Data Leakage

LLM-Guard addresses these risks by placing a security gateway between the application and the LLM service.

### High-Level Architecture

```text
    Application
         │
         ▼
┌──────────────────┐
│    LLM-Guard     │
│   Prompt Proxy   │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Firewall Rules  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│    DLP Engine    │
│ PII Detection &  │
│    Redaction     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Threat Detection│
│ Injection /      │
│    Jailbreak     │
└────────┬─────────┘
         │
         ▼
      LLM API
```

## Core Components

### Prompt Proxy

The Prompt Proxy acts as the primary gateway for LLM traffic.

Responsibilities:

* Intercept LLM API requests
* Inspect incoming request payloads
* Apply security controls
* Forward validated requests to the LLM API
* Return the LLM response to the application

The proxy layer is designed using **Go/Python** and communicates through REST APIs.

### Data Loss Prevention

The DLP Engine identifies sensitive information contained within prompt payloads before they are forwarded to an external LLM.

Initial detection focuses on entities such as:

* Email addresses
* Phone numbers
* Credit card information
* Other sensitive identifiable information

Detected entities can be redacted before the request reaches the LLM.

**Technology:** Microsoft Presidio / NER-based detection

### Firewall Rules

The firewall provides a deterministic security layer for incoming prompts.

Current controls include:

* Prompt length restrictions
* Keyword and pattern-based blocking
* Unsafe prompt detection
* Security policy enforcement
* Request allow/block decisions

This layer provides fast protection against known and easily identifiable attack patterns.

### AI Threat Detection

The AI Threat Detection module analyzes prompts for malicious behavior that may bypass basic rule-based filtering.

The detection layer focuses on:

* Prompt Injection
* Jailbreak Attempts
* Malicious instruction patterns
* Suspicious prompt structures
* Semantic characteristics of adversarial prompts

The system is designed to support lightweight NLP/ML-based classification for identifying threats that cannot be reliably detected through static rules alone.

## Request Processing Flow

```text
Incoming Request
       │
       ▼
   Prompt Proxy
       │
       ▼
 Firewall Rules
       │
   ┌───┴────┐
   │        │
 BLOCK    ALLOW
            │
            ▼
        DLP Engine
            │
            ▼
     Threat Detection
            │
       ┌────┴────┐
       │         │
     BLOCK     ALLOW
                 │
                 ▼
              LLM API
```

## Development Progress

### Week 1 — Proxy Architecture & DLP

#### Reverse Proxy

* Designed the initial reverse-proxy architecture.
* Established the request interception flow.
* Implemented the foundation for forwarding LLM API requests.
* Established the security processing pipeline.

#### DLP Pipeline

* Designed the initial DLP processing workflow.
* Integrated entity detection capabilities.
* Added detection for sensitive entities.
* Established the redaction stage before external LLM communication.

### Week 2 — Firewall Rules & Jailbreak Detection

#### Firewall Rules

* Implemented the initial firewall-rule layer.
* Added prompt-length restrictions.
* Added keyword and pattern-based blocking.
* Established security-rule evaluation.
* Implemented request allow/block decisions.

#### Jailbreak Detection

* Started development of the AI threat-detection component.
* Added the foundation for prompt-injection detection.
* Started identifying jailbreak patterns.
* Established the foundation for semantic prompt analysis.
* Prepared the detection layer for lightweight ML-based classification.

## Technology Stack

| Component               | Technology               |
| ----------------------- | ------------------------ |
| Proxy                   | Go / Python              |
| DLP                     | Python                   |
| NLP / ML                | Python                   |
| Entity Detection        | Microsoft Presidio / NER |
| API Communication       | REST                     |
| LLM Integration         | OpenAI-compatible APIs   |
| Development Environment | Windows / Ubuntu         |

## Security Objectives

LLM-Guard is designed around four primary security objectives:

1. **Inspect** — Analyze prompts before they reach the LLM.
2. **Protect** — Detect and redact sensitive information.
3. **Detect** — Identify prompt injection and jailbreak attempts.
4. **Enforce** — Apply security policies and block malicious requests.

## Project Goal

LLM-Guard aims to provide a security gateway for enterprise LLM applications by combining **traffic interception, deterministic firewall rules, Data Loss Prevention, and machine-learning-based threat detection** into a unified protection layer.

