# Mid Project Review
**Scope:** Week 1 (proxy core and DLP) and Week 2 (firewall rules and jailbreak detection)

## 1. Summary

Both mid project criteria were measured against the running system, with the proxy, DLP service, and firewall service active.

| Criterion | Target | Result | Status |
|---|---|---|---|
| Latency added by proxy and DLP | Minimal | 8 to 103 ms per request | Met |
| Jailbreak prompts blocked | At least 95% | 100% (24 of 24) | Met |
| Benign prompts blocked in error | No target set | 0 of 9 | None observed |

## 2. Latency audit

**Requirement.** The proxy and DLP redaction must add minimal latency to the model response.

**Method.** Fifteen sequential requests were sent to the running proxy. Total request time was recorded, and time spent in the security hooks was recorded separately from model generation time.

| Measure | Value |
|---|---|
| Runs | 15 |
| Average total time | 39,051.8 ms |
| Fastest run | 35,917.1 ms |
| Slowest run | 43,234.3 ms |
| Added by security checks | 8 to 103 ms per request |

**Result.** The security checks added 8 to 103 ms per request, about 0.02% to 0.26% of the average total time. The remaining time is local model generation.

## 3. Red team test

**Requirement.** At least 95% of known jailbreak prompts must be blocked.

**Method.** An automated script sent 24 jailbreak prompts and 9 benign prompts to the running system and compared each outcome with the expected one. The jailbreak set includes classic persona prompts, instruction override and erasure requests, new framings, and obfuscated text.

| Measure | Result |
|---|---|
| Jailbreak prompts blocked | 24 of 24 (100%) |
| Benign prompts blocked in error | 0 of 9 |

**Test condition.** The script sends no authentication token, so access control was switched off in `config.yaml` for the run and switched back on afterwards.

## 4. Conclusion

Both criteria are met. The security layer adds a small fraction of total response time, and the jailbreak test passed its 95% target with no false positives on the benign prompts used.