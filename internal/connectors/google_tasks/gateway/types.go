package gateway

import "time"

// TaskStatus is Google Tasks' status enum — exactly two values.
type TaskStatus string

const (
	// TaskStatusNeedsAction is an open (unchecked) task.
	TaskStatusNeedsAction TaskStatus = "needsAction"
	// TaskStatusCompleted is a checked task.
	TaskStatusCompleted TaskStatus = "completed"
)

// Task is the wiki's flat view of a Google Tasks resource. Per the plan
// the gateway hand-rolls the JSON in client.go; this struct is the
// post-decode shape the rest of the connector consumes.
//
// Wire fields the wiki cares about (subset of Google's full schema):
//   - id, etag, title, notes, status, parent, position
//   - due (date-only on Google's side; time-of-day always 00:00)
//   - completed (RFC3339, present when status == completed)
//   - updated (RFC3339, server-assigned; cursor authority)
//   - hidden, deleted (filter flags; surface in tombstone semantics)
//
// Google additionally returns selfLink, kind, links, webViewLink — the
// wiki has no use for those, so the wire decoder drops them.
type Task struct {
	ID        string
	Etag      string
	Title     string
	Notes     string
	Status    TaskStatus
	Parent    string
	Position  string
	Due       time.Time
	Completed time.Time
	Updated   time.Time
	Hidden    bool
	Deleted   bool
}

// TaskList is the wiki's view of a Google Tasks tasklist resource.
// Returned by tasklists.list during the subscribe-picker flow.
type TaskList struct {
	ID      string
	Etag    string
	Title   string
	Updated time.Time
}

// TasksPage is one page of a tasks.list response. NextPageToken is
// empty on the final page; callers must consume all pages before
// advancing the sync cursor (apply-then-advance, never advance during
// pagination — see plan "Cursor" semantics).
type TasksPage struct {
	Tasks         []Task
	NextPageToken string
}

// PatchFields names the fields a single tasks.patch call mutates.
// Empty-string values are the wiki's "leave alone" signal; nil pointers
// are not used because Go's zero-value model conflicts with the wire
// model where "" is a legal (cleared) string. Set the corresponding
// `Set*` flag to true to opt into a write.
//
// This isn't a Config struct — every field has a single owner (the
// caller picks which fields to write) and adding a new field here is
// always a deliberate "Tasks adds a new mutable field" event.
type PatchFields struct {
	SetTitle  bool
	Title     string
	SetNotes  bool
	Notes     string
	SetStatus bool
	Status    TaskStatus
	SetDue    bool
	Due       time.Time // zero clears `due`
	SetParent bool
	Parent    string
}

// TokenResponse is the post-decode shape of the OAuth token endpoint
// success body. Returned by RefreshAccessToken so the caller can
// observe whether Google rotated the refresh token (RFC 6749 §10.4)
// and handle the atomic-replace contract.
type TokenResponse struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    time.Duration
	RefreshToken string // empty when Google did NOT rotate
	Scope        string
}
