package sync

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// TasksOutboundSyncJobName is both the job's GetName() and the queue
// name registered on the JobQueueCoordinator. The debouncer's
// enqueue path imports this constant rather than hard-coding the
// string in two places.
const TasksOutboundSyncJobName = "GoogleTasksOutboundSync"

// TasksOutboundSyncJob is one wiki↔Tasks reconcile for a single
// (profile, page, listName). Built fresh per enqueue; the
// coordinator runs Execute() on its queue worker.
//
// Like Keep's outbound job, this carries no state besides the lookup
// key — it always reloads the subscription and reads current wiki
// state at run time. Debounce-coalescing N events for the same
// (profile, page, list) into one job picks up every intermediate
// change automatically.
type TasksOutboundSyncJob struct {
	connector *Connector
	profileID wikipage.PageIdentifier
	page      string
	listName  string
}

// NewTasksOutboundSyncJob builds a job.
func NewTasksOutboundSyncJob(connector *Connector, profileID wikipage.PageIdentifier, page, listName string) *TasksOutboundSyncJob {
	return &TasksOutboundSyncJob{
		connector: connector,
		profileID: profileID,
		page:      page,
		listName:  listName,
	}
}

// GetName returns the queue name. The coordinator routes jobs to
// queues by GetName(); every TasksOutboundSyncJob lands on the same
// single-worker queue, which serializes pushes per-worker — Tasks's
// per-user write quota is comfortable but ordering guarantees on
// position deltas matter.
func (*TasksOutboundSyncJob) GetName() string { return TasksOutboundSyncJobName }

// Execute runs the diff-and-push via Connector.Sync. context.Background()
// is used because the JobQueueCoordinator's worker doesn't carry a
// per-enqueue context; network operations have their own timeouts in
// the http.Client.
func (j *TasksOutboundSyncJob) Execute() error {
	return j.connector.Sync(context.Background(), connectors.SubscriptionKey{
		ProfileID: string(j.profileID),
		Page:      j.page,
		ListName:  j.listName,
	})
}
