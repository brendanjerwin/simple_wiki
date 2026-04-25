package server

// AgentScheduleRefreshJobName is the queue name for AgentScheduleRefreshJob;
// the queue runs single-worker so refreshes for a given page execute serially
// in submission order, avoiding races against the cron registrar.
const AgentScheduleRefreshJobName = "AgentScheduleRefresh"

// AgentScheduleRefreshJob reconciles the cron registrations for one page's
// agent.schedules block after the page has been written. It is a Job
// (compatible with JobQueueCoordinator) so refreshes flow through the same
// concurrency machinery as every other background unit of work.
type AgentScheduleRefreshJob struct {
	scheduler *AgentScheduler
	page      string
}

// NewAgentScheduleRefreshJob constructs a job that asks scheduler to Refresh
// the supplied page on Execute.
func NewAgentScheduleRefreshJob(scheduler *AgentScheduler, page string) *AgentScheduleRefreshJob {
	return &AgentScheduleRefreshJob{
		scheduler: scheduler,
		page:      page,
	}
}

// GetName implements jobs.Job.
func (*AgentScheduleRefreshJob) GetName() string {
	return AgentScheduleRefreshJobName
}

// Execute implements jobs.Job by delegating to AgentScheduler.Refresh. Any
// error from Refresh is returned so the queue's error channel surfaces it.
func (j *AgentScheduleRefreshJob) Execute() error {
	return j.scheduler.Refresh(j.page)
}
