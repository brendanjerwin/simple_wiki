package server

import (
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
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
	logger     lumber.Logger
}

// NewPageInventoryNormalizationJob creates a new per-page inventory normalization job.
func NewPageInventoryNormalizationJob(
	pageID wikipage.PageIdentifier,
	deps InventoryNormalizationDependencies,
	logger lumber.Logger,
) *PageInventoryNormalizationJob {
	return &PageInventoryNormalizationJob{
		pageID:     pageID,
		normalizer: NewInventoryNormalizer(deps, logger),
		logger:     logger,
	}
}

// Execute runs the per-page inventory normalization.
func (j *PageInventoryNormalizationJob) Execute() error {
	createdPages, err := j.normalizer.NormalizePage(j.pageID)
	if err != nil {
		return err
	}

	if len(createdPages) > 0 {
		j.logger.Info("Per-page normalization for %s created %d item pages", j.pageID, len(createdPages))
	}

	return nil
}

// GetName returns the job name.
func (*PageInventoryNormalizationJob) GetName() string {
	return PageInventoryNormalizationJobName
}
