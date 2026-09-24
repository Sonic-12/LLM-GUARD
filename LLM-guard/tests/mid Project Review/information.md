# Mid-Project Review

**Project:** LLM-Guard Generative AI Prompt Firewall

**Scope Reviewed:** Week 1 (Proxy Core + DLP) and Week 2 (Firewall Rules + Jailbreak Detection)

## Executive Summary
This review evaluates Week 1 and Week 2 of LLM-Guard against the two criteria defined for the Mid-Project Review stage:  Request latency and Jailbreak detection accuracy.

## Methodology

### 1. Latency Audit
- **Requirement:** Minimal added latency from the proxy and DLP layer.
- **Method:** Measured end-to-end request time across multiple sequential requests to the live proxy, with per-hook timing isolated to separate the DLP/rules layer's overhead from LLM generation time.
- **Timing (15 runs):** Average 39,051.8 ms | Min 35,917.1 ms | Max 43,234.3 ms end-to-end per request.
- **Result:** Security checks add only 8–103ms per request. The bulk of response time comes from local LLM generation, which is outside this layer's scope.

### 2. Red Team Test
- **Requirement:** Block at least 95% of known jailbreak attempts.
- **Method:** Ran an automated battery of 24 adversarial (jailbreak) prompts and 9 benign prompts against the live, fully deployed system (proxy, DLP service, and firewall service), measuring block rate and false-positive rate.
- **Result:** Passed 100% (24/24) adversarial prompts blocked, with 0 false positives on benign prompts, verified against the live system.

## Conclusion
Both Mid-Project Review criteria are met and verified against the live system. 

---

## Important Note
RBAC must be **temporarily disabled** to run this Mid-Project Review, since the red-team test does not send an authentication token and RBAC did not exist at the time this review was originally written against.

1. In `config.yaml`, set `rbac.enabled: false`.
2. Restart the proxy.
3. Run the Mid-Project Review tests.
4. Set `rbac.enabled: true` again and restart the proxy once finished.