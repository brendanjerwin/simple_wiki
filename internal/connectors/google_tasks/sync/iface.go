package sync

import (
	"context"
	"net/http"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Clock returns the current time. SystemClock for production, fake
// for tests.
type Clock interface {
	Now() time.Time
}

// SystemClock returns time.Now wrapped in a Clock.
type SystemClock struct{}

// Now returns wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// Logger is the lumber-style log surface the orchestrator uses.
// Method shapes match jcelliott/lumber's ConsoleLogger so the wiki's
// existing logger plugs in directly.
type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// TasksClient is the subset of *gateway.TasksClient the connector
// calls. Stated as an interface so tests can substitute a fake
// without spinning up an httptest.Server.
type TasksClient interface {
	ListTaskLists(ctx context.Context) ([]gateway.TaskList, error)
	CreateTaskList(ctx context.Context, title string) (gateway.TaskList, error)
	ListTasks(ctx context.Context, tasklistID string, updatedMin time.Time, pageToken string) (gateway.TasksPage, error)
	InsertTask(ctx context.Context, tasklistID, title, notes string, status gateway.TaskStatus, due time.Time, parent string) (gateway.Task, error)
	PatchTask(ctx context.Context, tasklistID, taskID string, fields gateway.PatchFields, etag string) (gateway.Task, error)
	DeleteTask(ctx context.Context, tasklistID, taskID string) error
}

// TasksClientFactory builds a TasksClient bound to the per-profile
// RefreshClient (the gateway's TokenSource). The bootstrap layer
// supplies the real factory; tests substitute a stub.
type TasksClientFactory func(profileID wikipage.PageIdentifier, refreshToken string) (TasksClient, gateway.TokenSource, error)

// ChecklistReader is the read-side of the wiki's checklist domain.
// The real type lives in server/checklistmutator; pulling it here as
// an interface keeps the sync package from depending on the server
// package — bootstrap injects the concrete reader at startup.
type ChecklistReader interface {
	ListItems(ctx context.Context, page, listName string) (*apiv1.Checklist, error)
}

// ChecklistMutator is the write-side counterpart used by inbound sync
// to apply Tasks-side changes to the wiki. The mutator's notify hook
// MUST be suppressed for the sync window (via SyncSuppressor.Suppress)
// or these calls trigger another sync job and loop forever.
//
// Args mirror the real mutator's: ownerEmail attribution makes
// completed_by readable as the operator who connected Tasks rather
// than an opaque "system" placeholder.
type ChecklistMutator interface {
	// AddItemForSync inserts a fresh wiki item from a Tasks-side
	// arrival. Returns the wiki uid the mutator stamped (so the
	// orchestrator can populate ItemIDMap before persisting).
	AddItemForSync(ctx context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, sortValueHint string) (string, error)
	// UpdateItemForSync mirrors a Tasks-side edit into the wiki.
	UpdateItemForSync(ctx context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error
	// DeleteItemForSync mirrors a Tasks-side deletion (or hidden
	// completed task) into the wiki. Idempotent.
	DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error
	// AppendSyncEvent emits a self-source event into the checklist's
	// op-log without mutating any item. Per ADR-0015: connectors call
	// this after a successful outbound push so their LastSyncedSeq
	// cursor advances past the user-event that triggered the push.
	// Without this, the user-event remains "above the cursor"
	// permanently and any subsequent Tasks-side change for the same
	// uid is silently blocked from inbound-applying. `op` is a
	// diagnostic label (e.g. `outbound_pushed`).
	AppendSyncEvent(ctx context.Context, page, listName, uid, op string) error
}

// SyncSuppressor is the notify-suppression interface the inbound-
// apply pass uses so mutator notifies don't loop back as fresh sync
// triggers. SyncDebouncer satisfies this; tests stub it.
type SyncSuppressor interface {
	Suppress(profileID wikipage.PageIdentifier, page, listName string)
	Unsuppress(profileID wikipage.PageIdentifier, page, listName string)
}

// HTTPClientForRefresh is the http.Client the gateway's RefreshClient
// uses. The connector accepts it as a dependency so bootstrap can
// inject a single shared client (with a sensible Timeout) rather than
// the connector building one with hard-coded values.
type HTTPClientForRefresh = *http.Client
