# LLM-Guard Security

> See also: [Architecture](architecture.md) · [Setup Guide](setup.md)

This document describes the security controls implemented in LLM-Guard, the
assumptions behind them, and their known limitations. It reflects the
current implementation only.

## 1. Security Model

LLM-Guard follows two principles throughout:

- **Enforce at one point.** All traffic to the model passes through the
  proxy, so every control sits on a single, mandatory path. There is no
  route from a client to the model that bypasses the hook chain.
- **Fail closed.** If a service a security check depends on cannot be
  reached, the request or response is refused rather than allowed through
  unchecked. The one deliberate exception is noted in §4.

A request that fails any check is stopped at that stage, is never forwarded
further down the chain, and produces an audit event (§7).

## 2. Authentication and Authorization (RBAC)

**Implementation:** `proxy/internal/rbac`

- Tokens are OpenID Connect ID tokens issued by Keycloak, signed with RS256.
- The proxy verifies each token itself: it fetches the identity provider's
  public signing keys (JWKS), checks the signature, issuer, authorized
  party (client), and expiry, and never forwards the client's password or
  calls Keycloak on a per-request basis.
- The token header's `alg` field is required to be exactly `RS256` before
  any further verification happens. No other algorithm is accepted, which
  rules out algorithm-substitution attacks against the verifier.
- Each request's realm roles are mapped to a configured role. A role with
  no mapping in `config.yaml` is rejected (`UNKNOWN_ROLE`).
- Each role has its own allowed model tiers and maximum prompt length,
  enforced per request:

  | Role | Models | Max prompt length |
  |---|---|---|
  | admin | default and premium | 4000 characters |
  | employee | default and premium | 4000 characters |
  | guest | default only | 1000 characters |

- If the identity provider is not configured (`issuer_url` or `jwks_url`
  missing), the hook fails closed with `RBAC_MISCONFIGURED_NO_IDP` rather
  than allowing requests through unauthenticated.
- A missing or malformed `Authorization` header is rejected with
  `MISSING_TOKEN` before any token parsing is attempted.

## 3. Threat Detection

**Implementation:** `proxy/internal/rules`, `proxy/internal/threatdetect`,
`services/firewall`

Threat detection runs in two stages after RBAC:

1. **Deterministic rules** — a maximum prompt length check and a pattern
   blocklist for known attack phrasing (instruction override, role-reversal,
   and similar jailbreak framings).
2. **ML classification** — the prompt is sent to a dedicated firewall
   service running a trained classifier (TF-IDF and character n-gram
   features with logistic regression). The classifier returns a jailbreak
   probability score and an allow/block decision.

Both stages run on a **normalized** form of the prompt
(`proxy/internal/rules/normalize.go`), which strips zero-width characters,
converts fullwidth Unicode variants to their standard form, and maps common
homoglyphs (visually identical characters from other alphabets, such as
Cyrillic а/е/о) back to Latin equivalents before matching. This is intended
to resist obfuscated prompts built to evade plain keyword matching.

The classifier uses two configurable thresholds rather than a single
cutoff:

| Score | Result |
|---|---|
| Below the low threshold | Allowed |
| Between the low and high thresholds | Blocked, flagged for human review |
| At or above the high threshold | Blocked |

If the firewall service is unreachable, the request is **blocked**
(`INSPECTION_SERVICE_UNAVAILABLE_FAIL_CLOSED`) rather than allowed through
unscored.

## 4. Data Protection (DLP)

**Implementation:** `proxy/internal/dlp`, `dlp-service/`

- Before a prompt reaches the model, it is sent to the DLP service, which
  uses Presidio (with the `en_core_web_md` spaCy model) plus custom pattern
  recognizers to detect email addresses, credit card numbers, US Social
  Security Numbers, and API keys.
- Each detected value is replaced with a token (for example,
  `[EMAIL_ADDRESS_3fa9c1d2]`) before the prompt is forwarded to the model.
  The token-to-value mapping is kept in memory for the lifetime of that
  single request only — the DLP service does not persist any sensitive
  value it handles.
- On the way back, the model's response is scanned by Output Validation
  (§5) for any of these entity types, and the original values are restored
  by the DLP service before the response reaches the caller.
- **Fail-closed on redaction:** if the DLP service cannot be reached for
  the inbound `/redact` call, the request is blocked rather than forwarded
  to the model unmasked.
- **Fail-open on restoration only:** if the DLP service cannot be reached
  for the outbound `/unmask` call, the response is still returned to the
  caller with tokens left in place rather than the original values. This is
  a deliberate exception to the fail-closed principle in §1: it is a
  usability trade-off for a non-security-critical path, since no sensitive
  data is exposed either way — the caller simply sees the token instead of
  the original value.

## 5. Output Validation

**Implementation:** `proxy/internal/outputguard`

Every model reply is checked before it is returned to the caller:

- **Toxicity** — pattern-based detection of threatening language and a
  scored check against a list of abusive terms; a reply matching either is
  blocked.
- **Data leakage** — the reply is re-scanned by the DLP service's
  `/redact` endpoint; if it still contains a detectable sensitive entity
  after the model has generated it, the reply is blocked
  (`OUTPUT_PII_LEAK`). This check fails closed: if the DLP service is
  unreachable for this scan, the reply is blocked rather than returned
  unchecked.
- **Hallucination signals** — replies containing unverifiable citation
  claims or bare URLs are flagged and logged, but not blocked, since the
  proxy has no way to verify the underlying claim.

## 6. Prompt Hardening

**Implementation:** `proxy/internal/hardening`

Before a request reaches the model, an instructional defense is added or
appended to the system prompt instructing the model to treat any attempt to
override, ignore, or reveal its instructions as an attack rather than a
legitimate command. The user's message is then isolated with explicit
delimiters and a reminder that content between them is data to respond to,
never a new instruction — a technique intended to reduce the model's
susceptibility to injected instructions carried inside user content.

## 7. Logging and Telemetry

**Implementation:** `proxy/internal/telemetry`, `sidecar/analytics`

- Every request blocked by RBAC, the rules/threat-detection stage, or
  output validation produces a structured audit event: timestamp, event
  ID, user ID (from the verified token), client address, the layer that
  blocked it, prompt length, status code, reason, and a truncated prompt
  sample.
- Events are written locally as JSON Lines and also forwarded to the
  analytics sidecar, which stores them in SQLite and serves them through
  a query API and a dashboard. A failure to reach the sidecar is logged
  without altering the response already sent to the caller — telemetry
  delivery is not on the request's critical path.
- Two additional logs record non-blocking information: every rules-stage
  decision (allowed or blocked) for audit purposes, and every hallucination
  flag raised on a reply that was still delivered.

## 8. Known Limitations

- The jailbreak classifier is a lightweight statistical model trained on a
  fixed dataset. It is effective against known and closely related attack
  patterns, not a guarantee against novel ones.
- The toxicity and hallucination checks are regex and keyword heuristics,
  not trained models — tuned to avoid false positives rather than to catch
  every case.
- DLP entity detection covers a fixed set of types (email addresses, credit
  cards, US SSNs, API keys); it does not attempt general-purpose PII
  detection beyond these.
- RBAC trusts the identity provider's signing keys as configured; key
  rotation is handled by the JWKS cache's refresh, not by any separate
  revocation mechanism.

## 9. Fail-Safe Behavior Summary

| Stage | Behavior if its dependency is unreachable |
|---|---|
| RBAC | Fails closed if the identity provider is unconfigured; rejects missing/invalid tokens |
| Threat Detection (ML) | Fails closed — request blocked |
| DLP — inbound redaction | Fails closed — request blocked |
| DLP — outbound restoration | Fails open — response returned with tokens unrestored |
| Output Validation — leak re-scan | Fails closed — response blocked |
| Telemetry | Fails silently — does not alter the response, logged separately |