package protocol

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

	// ErrRateLimited means Google returned 429. Caller should back off.
	ErrRateLimited = errors.New("keep: rate limited")

	// ErrBoundNoteDeleted means a previously bound note no longer exists
	// in the user's Keep account. UI should offer rebind or remove.
	ErrBoundNoteDeleted = errors.New("keep: bound note deleted")

	// ErrProtocolDrift means a response shape or status didn't match what
	// our typed decoder expects. Almost always indicates Google changed
	// the wire protocol; surfaces to UI as "update simple_wiki".
	ErrProtocolDrift = errors.New("keep: protocol drift")
)
