package server

import (
	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	// PageInventoryNormalizationJobName is the name of the per-page normalization job
	PageInventoryNormalizationJobName = "PageInventoryNormalizationJob"
)

// PageInventoryNormalizationJob is a lightweight job that normalizes inventory for a single page.
// It runs when a page is saved to ensure inventory consistency without waiting for the full job.
type PageInventoryNormalizationJob struct {
	pageID     wikipage.PageIdentifier
	normalizer *InventoryNormalizer
	logger     logging.Logger
}

// NewPageInventoryNormalizationJob creates a new per-page inventory normalization job.
func NewPageInventoryNormalizationJob(
	pageID wikipage.PageIdentifier,
	deps InventoryNormalizationDependencies,
	logger logging.Logger,
) *PageInventoryNormalizationJob {
	return &PageInventoryNormalizationJob{
		pageID:     pageID,
		normalizer: NewInventoryNormalizer(deps, logger),
		logger:     logger,
	}
}

// Execute runs the per-page inventory normalization.
func (j *PageInventoryNormalizationJob) Execute() error {
	result, err := j.normalizer.NormalizePage(j.pageID)
	if err != nil {
		return err
	}

	if len(result.CreatedPages) > 0 {
		j.logger.Info("Per-page normalization for %s created %d item pages", j.pageID, len(result.CreatedPages))
	}

	if len(result.FailedPages) > 0 {
		j.logger.Warn("Per-page normalization for %s failed to create %d item pages", j.pageID, len(result.FailedPages))
		for _, failed := range result.FailedPages {
			j.logger.Warn("  Failed: item=%s container=%s error=%v", failed.ItemID, failed.ContainerID, failed.Error)
		}
	}

	return nil
}

// GetName returns the job name.
func (*PageInventoryNormalizationJob) GetName() string {
	return PageInventoryNormalizationJobName
}
