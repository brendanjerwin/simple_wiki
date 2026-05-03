package sync

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// KeepOutboundSyncJobName is both the job's GetName() and the queue
// name registered on the JobQueueCoordinator. Mutator-hook subscribers
// import this constant rather than hard-coding the string in two
// places.
const KeepOutboundSyncJobName = "KeepOutboundSync"

// KeepOutboundSyncJob is one wiki→Keep reconcile for a single
// (profile, page, listName). Built fresh per enqueue; the coordinator
// runs Execute() on its queue worker.
//
// The job carries no state besides the lookup key — it always reloads
// the binding and reads current wiki state at run time. This means
// debounce-coalescing N events for the same (profile, page, list) into
// one job picks up every intermediate change automatically.
type KeepOutboundSyncJob struct {
	connector *Connector
	profileID wikipage.PageIdentifier
	page      string
	listName  string
}

// NewKeepOutboundSyncJob builds a job. The connector is the only heavy
// dep; profile/page/list are the lookup key.
func NewKeepOutboundSyncJob(connector *Connector, profileID wikipage.PageIdentifier, page, listName string) *KeepOutboundSyncJob {
	return &KeepOutboundSyncJob{
		connector: connector,
		profileID: profileID,
		page:      page,
		listName:  listName,
	}
}

// GetName returns the queue name. The coordinator routes jobs to
// queues by GetName(); every KeepOutboundSyncJob lands on the same
// single-worker queue, which serializes pushes per-worker — important
// because Keep's targetVersion is global per user and concurrent
// pushes on the same account would race.
func (*KeepOutboundSyncJob) GetName() string { return KeepOutboundSyncJobName }

// Execute runs the diff-and-push. context.Background() is used because
// the JobQueueCoordinator's worker doesn't carry a per-enqueue context
// (Artifex predates context.Context). Network operations have their
// own timeouts in the http.Client.
func (j *KeepOutboundSyncJob) Execute() error {
	return j.connector.SyncToKeep(context.Background(), j.profileID, j.page, j.listName)
}
