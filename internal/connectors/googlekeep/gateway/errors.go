package gateway

import "errors"

// Sentinel errors covering the failure surface in REFERENCE.md. Callers
// branch on these via errors.Is, never on string contents (per CLAUDE.md
// "Never Branch Logic on Error Messages").
var (
	// ErrInvalidCredentials means Google rejected the supplied email + ASP
	// (Stage 1 BadAuthentication). User must regenerate their App-Specific
	// Password.
	ErrInvalidCredentials = errors.New("keep: invalid credentials")

	// ErrAuthRevoked means the master token or short-lived bearer is no
	// longer accepted (account password change, ASP revoked, or the
	// account requires browser interaction). User must reconnect.
	ErrAuthRevoked = errors.New("keep: auth revoked")

	// ErrRateLimited means Google returned 429, or a 403 whose body
	// carries a quota-exhaustion reason (status RESOURCE_EXHAUSTED).
	// Caller should back off.
	ErrRateLimited = errors.New("keep: rate limited")

	// ErrServiceDisabled means Google returned 403 with a body
	// indicating the upstream API is not enabled on the GCP project
	// (errors[].reason == "accessNotConfigured" OR
	// details[].reason == "SERVICE_DISABLED"). This is an operator
	// setup error — not retryable. Surfaces as FailedPrecondition with
	// the Google activation URL embedded in the message when
	// available. Mirrors the Tasks gateway sentinel of the same name —
	// distinct sentinels per gateway because callers branch by package.
	ErrServiceDisabled = errors.New("keep: service disabled")

	// ErrPermissionDenied means Google returned 403 for a reason
	// other than rate-limiting or service-disabled (e.g. the OAuth
	// scope doesn't permit the operation, the resource is owned by a
	// different account, etc). Distinct from ErrAuthRevoked because
	// the token itself is still valid — it just can't reach this
	// resource.
	ErrPermissionDenied = errors.New("keep: permission denied")

	// ErrBoundNoteDeleted means a previously bound note no longer exists
	// in the user's Keep account. UI should offer rebind or remove.
	ErrBoundNoteDeleted = errors.New("keep: bound note deleted")

	// ErrProtocolDrift means a response shape or status didn't match what
	// our typed decoder expects. Almost always indicates Google changed
	// the wire protocol; surfaces to UI as "update simple_wiki".
	ErrProtocolDrift = errors.New("keep: protocol drift")
)
