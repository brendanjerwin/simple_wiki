// Package gateway is the wire-protocol client for Google Tasks REST v1
// (PoEAA Gateway). It hand-rolls JSON over net/http rather than depend
// on Google's official client library — the Tasks surface is small
// enough (5 endpoints + OAuth refresh) that pulling in the entire
// googleapi tree is heavyweight, and matching the shape of the Keep
// gateway keeps the per-connector mental model consistent.
//
// Callers branch on the typed sentinels in errors.go (errors.Is) — never
// on string contents (per CLAUDE.md "Never Branch Logic on Error
// Messages").
package gateway

import "errors"

// Sentinel errors covering the failure surface of OAuth refresh + Tasks
// REST v1. Tests assert with errors.Is and the sync layer branches on
// these to decide whether to pause a subscription, retry, or surface to
// the user.
var (
	// ErrInvalidGrant means the refresh token was rejected by Google's
	// token endpoint (revoked, expired, or used in a rotation race that
	// our retry-once didn't recover). Sync layer marks the subscription
	// paused; UX prompts the user to reconnect.
	ErrInvalidGrant = errors.New("tasks: invalid_grant")

	// ErrAuthRevoked means an access token call returned 401. Sync
	// layer attempts a refresh; if the refresh ALSO fails, that comes
	// back as ErrInvalidGrant.
	ErrAuthRevoked = errors.New("tasks: auth revoked")

	// ErrRateLimited means Google returned 429, or a 403 whose body
	// carries a quota-exhaustion reason (status RESOURCE_EXHAUSTED).
	// Caller should back off and retry on a later tick.
	ErrRateLimited = errors.New("tasks: rate limited")

	// ErrServiceDisabled means Google returned 403 with a body
	// indicating the Tasks API is not enabled on the GCP project
	// (errors[].reason == "accessNotConfigured" OR
	// details[].reason == "SERVICE_DISABLED"). This is an operator
	// setup error — not retryable. Surfaces as FailedPrecondition with
	// the Google activation URL embedded in the message when
	// available.
	ErrServiceDisabled = errors.New("tasks: service disabled")

	// ErrPermissionDenied means Google returned 403 for a reason
	// other than rate-limiting or service-disabled (e.g. the OAuth
	// scope doesn't permit the operation, the resource is owned by a
	// different account, etc). Distinct from ErrAuthRevoked because
	// the token itself is still valid — it just can't reach this
	// resource.
	ErrPermissionDenied = errors.New("tasks: permission denied")

	// ErrPreconditionFailed means a tasks.patch with If-Match returned
	// 412 — the local etag is stale. Caller should pull-and-retry-once
	// with fresh etag.
	ErrPreconditionFailed = errors.New("tasks: precondition failed")

	// ErrNotFound means the tasklist or task no longer exists. Caller
	// decides whether to treat as "deleted upstream" or surface to UX.
	ErrNotFound = errors.New("tasks: not found")

	// ErrProtocolDrift means a response shape didn't match what the
	// typed decoder expects (missing required field, unparseable
	// timestamp, etc). Almost always indicates Google changed the
	// wire protocol; surfaces to UI as "update simple_wiki."
	ErrProtocolDrift = errors.New("tasks: protocol drift")

	// ErrScopeDowngraded means the token endpoint's scope echo did not
	// include the required tasks scope (RFC 6749 §3.3 hygiene — Google
	// may downgrade scope). Surfaces as a configuration error.
	ErrScopeDowngraded = errors.New("tasks: scope downgraded")

	// ErrIssuerMismatch means the RFC 9207 `iss` callback parameter
	// did not match the configured authorization server's issuer.
	// Used by the OAuth callback handler in Phase 6 — exposed here so
	// the gateway's PKCE/iss helpers and the handler can branch on
	// the same sentinel.
	ErrIssuerMismatch = errors.New("tasks: issuer mismatch")
)
