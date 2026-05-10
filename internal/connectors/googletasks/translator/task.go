// Package translator is the Anti-Corruption Layer (DDD sense) between
// the wiki's ChecklistItem domain model and Google Tasks's wire-protocol
// task shape. The functions here are pure: no I/O, no clock, no
// orchestrator state — fed a wiki item or a Tasks task, they return the
// other shape.
//
// The sibling sync package owns orchestration (Connector, Subscription
// store, debounce, cron, jobs); the sibling gateway package owns the
// wire protocol. Translation never reaches across either of those
// boundaries and never holds connector state, which is why it sits in
// its own package and not on a struct method.
package translator

import "time"

// Task is the local placeholder wire-shape for a Google Tasks `Task`
// resource. It defines the fields the schema mapping cares about.
//
// PLACEHOLDER: the real wire type lives (or will live) in the sibling
// gateway package — `internal/connectors/google_tasks/gateway`. Phase 5
// (sync orchestrator) is where the gateway's concrete type and this
// translator's transformations are wired together. Until then the
// translator package owns its own wire-shape definition so it can be
// developed and tested in isolation. When the gateway type lands, this
// definition can either be replaced by an alias (`type Task =
// gateway.Task`) or deleted in favor of the gateway type, depending on
// whether mock construction in tests benefits from a translator-private
// shape.
//
// Field semantics mirror Google's REST API
// (https://developers.google.com/tasks/reference/rest/v1/tasks):
//   - ID, ETag, Title, Notes, Status, Position are strings on the wire.
//   - Updated, Due, Completed are RFC3339 strings on the wire; we model
//     them as time.Time (zero-Time when absent).
//   - Status is "needsAction" | "completed".
//   - Parent is the parent task's id when this task is a subtask
//     (empty for top-level tasks).
//   - Deleted/Hidden are tombstone-style flags Google sets on completed
//     or deleted tasks; the wiki currently surfaces deletion via the
//     normal mapping path.
type Task struct {
	ID        string
	ETag      string
	Title     string
	Notes     string
	Status    string
	Position  string
	Parent    string
	Updated   time.Time
	Due       time.Time
	Completed time.Time
	Deleted   bool
	Hidden    bool
}

// TaskFields is the subset of Task fields the wiki produces when pushing
// a wiki ChecklistItem to Google Tasks (insert or patch). Identifiers
// (ID, ETag) and server-stamped timestamps (Updated) are intentionally
// omitted — those are owned by Google or by the gateway/sync layers.
//
// This shape exists separately from Task because the inbound and
// outbound directions have different field-presence semantics: a Task
// fetched from Google has every field populated by the server; a
// payload sent to Google must only include the fields the client wants
// to mutate (and patch semantics require explicit zero-values for
// fields the client wants to clear, which the gateway layer is
// responsible for distinguishing — the translator just produces the
// "what should this task look like after the push" shape).
type TaskFields struct {
	Title     string
	Notes     string
	Status    string
	Due       time.Time
	Completed time.Time
}

// TaskStatusNeedsAction and TaskStatusCompleted are the two values
// Google Tasks's `status` field takes. Centralized here so the schema
// mapping code reads as a small enum rather than scattered string
// literals.
const (
	TaskStatusNeedsAction = "needsAction"
	TaskStatusCompleted   = "completed"
)
