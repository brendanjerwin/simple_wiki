package server

import (
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

// FileShadowingService manages separate file shadowing queues using the job queue coordinator.
type FileShadowingService struct {
	coordinator *jobs.JobQueueCoordinator
	site        *Site
}

// NewFileShadowingService creates a new FileShadowingService.
func NewFileShadowingService(coordinator *jobs.JobQueueCoordinator, site *Site) *FileShadowingService {
	return &FileShadowingService{
		coordinator: coordinator,
		site:        site,
	}
}


// EnqueueScanJob enqueues a scan job to find PascalCase files.
func (s *FileShadowingService) EnqueueScanJob() {
	scanJob := NewFileShadowingMigrationScanJob(s.site.PathToData, s.coordinator, s.site)
	s.coordinator.EnqueueJob(scanJob)
}

// GetJobQueueCoordinator returns the underlying job queue coordinator for status monitoring.
func (s *FileShadowingService) GetJobQueueCoordinator() *jobs.JobQueueCoordinator {
	return s.coordinator
}