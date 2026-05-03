# ADR-0014: Limited Security Stance for the Self-Hosted Tailnet-Perimeter Wiki

## Status

Accepted

## Date

2026-05-02

## Context

`simple_wiki` is a self-hosted, household-scale, non-internet-exposed application. Deployments sit behind a Tailscale-fronted perimeter (`tailscale serve` or equivalent); the wiki binary itself never listens on a public IP. The user base is "people on the operator's tailnet" — typically a household.

The Google Tasks connector (and future OAuth-bearing connectors) introduces refresh tokens that persist on disk in user profile pages. Published OAuth security guidance — RFC 6819 (*OAuth 2.0 Threat Model and Security Considerations*) §5.1.5.3 and RFC 9700 (*Best Current Practice for OAuth 2.0 Security*) — recommends:

- Encryption-at-rest for refresh tokens.
- Key management infrastructure (KMS / HSM, key rotation, recovery procedures).
- Audit logging of credential access.

Following this guidance for `simple_wiki` would be **counter-intuitive vs. the BCP**: we deliberately do not implement these controls. This ADR records the decision so a future maintainer doesn't try to retrofit security infrastructure under the assumption that "we just hadn't gotten to it yet."

## Decision

Adopt **only** the protocol-layer security controls that cost nothing. Deliberately decline the storage-layer and operational controls that require key management.

### What we do (protocol-layer hygiene, zero key-management cost)

- **PKCE S256** on the OAuth authorization-code flow (RFC 7636).
- **Single-use server-side state token** (≥256 bits, `crypto/rand`, deleted on first lookup, 10-minute TTL).
- **`iss` validation** per RFC 9207, pinned to `https://accounts.google.com` for Google connectors. Pinning even with one IdP makes future iCloud / Microsoft additions a config change, not a security retrofit.
- **Atomic refresh-token rotation** per RFC 6749 §10.4. If a refresh exchange returns a new refresh token, write-ahead persist the new value before consuming the new access token.
- **`invalid_grant` retry-once** to handle the rotation-race ambiguity in RFC 6749 §5.2 transparently.
- **Validation order:** validate `iss` and `state` *before* exchanging the authorization code on callback. Pinned in handler comments.

### What we deliberately don't do

- **No encryption-at-rest for refresh tokens.** They persist in plaintext TOML on user profile pages.
- **No KMS / HSM integration.** No key material is managed by the wiki.
- **No key rotation infrastructure.**
- **No audit logging of credential access.**

### Rationale

Encryption-at-rest requires key management — rotation, KMS or equivalent, recovery. Key management is a substantial operational burden. It is **expressly out of scope** for this project's threat model.

The trust perimeter is the Tailnet, not the filesystem. An attacker who has filesystem read access to the wiki's data directory has already bypassed Tailscale, the operator's host security, and the operator's backup hygiene. At that point, key material on the same host buys little — the attacker reads the keys from the same disk they read the encrypted tokens from. Defense-in-depth with key material co-located with ciphertext is theatre.

The realistic threat is "stolen disk" or "leaked backup tape." For those, the mitigation is **revoke at Google**, which is one click in the user's Google account and propagates within minutes — not "encrypt locally and hope the key material wasn't on the same tape."

The household threat model also includes "household member misuse," which is **out of scope for technical controls** in any case — the wiki is a shared family knowledge base by design.

### Trigger events that flip the stance

This ADR is **superseded** (replaced by a successor ADR with a stricter stance) the moment any of the following becomes true:

- The wiki is exposed on the public internet.
- The wiki is opened to non-Tailnet users.
- The wiki is used commercially.

Until then, this ADR is the operative decision.

## Consequences

### Positive

- Zero key-management burden. No KMS to operate, no rotation schedule to maintain, no recovery runbook to test.
- Protocol-layer hygiene (PKCE, state, iss, rotation) is in place and free, so a future stance flip doesn't have to retrofit those.
- Honesty: the deviation from RFC 6819 / RFC 9700 is documented and reasoned, not silently elided.

### Negative

- Refresh tokens are recoverable from a stolen disk or leaked backup until manually revoked at Google. The wiki cannot prevent this; it can only assume the perimeter holds.
- A maintainer reading RFC 6819 / RFC 9700 will see this ADR as a deliberate dissent and may feel the urge to "fix" it. The trigger-events list is the antidote: flip the stance only when the threat model actually changes.

### Neutral

- The protocol-layer controls listed under "What we do" remain in place regardless of stance. They are correct hygiene, not threat-model-dependent.

## Alternatives considered

- **Encrypt-at-rest with KMS / HSM integration.** Rejected: key management burden is disproportionate to the threat reduction, given that key material would co-locate with ciphertext on a single self-hosted host.
- **Deferred encryption with a calendar-date trigger ("revisit in 12 months").** Rejected: a trigger *event* (perimeter change, user-base change, commercial use) is more honest than an arbitrary date. Calendar dates encourage cargo-cult upgrades without a real change in threat model.
- **Reject OAuth-token-bearing connectors entirely** (only support connectors that don't need durable credentials). Rejected: the feature value of Google Tasks / iCloud Reminders integration outweighs the residual risk in this threat model.
- **Implement audit logging without encryption.** Rejected: audit logging without a tamper-evident store and a reviewer to actually read the logs is theatre. Adding it now would suggest a maturity that doesn't exist.

## References

- ADR-0013: Per-deployment GCP project (companion ADR on what *is* in scope for operator setup).
- Plan: `now-that-we-landed-groovy-pizza.md` (OAuth security profile section).
- RFC 6749 — OAuth 2.0 Authorization Framework.
- RFC 6819 — OAuth 2.0 Threat Model and Security Considerations (the BCP we deviate from).
- RFC 7636 — Proof Key for Code Exchange (PKCE).
- RFC 9207 — OAuth 2.0 Authorization Server Issuer Identification.
- RFC 9700 — Best Current Practice for OAuth 2.0 Security (the BCP we deviate from).
