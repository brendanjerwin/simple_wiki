package pagestore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HistoryDecimationJob walks the __history__ directory and thins old
// versions per a GFS-style retention schedule:
//   - Keep ALL versions from the last 7 days
//   - Keep 1/week (newest in each ISO week) for the last 26 weeks
//   - Keep 1/month (newest in each calendar month) for the last 5 years
//   - Purge everything older than 5 years
//
// The job is cron-triggered daily and run through the JobQueueCoordinator.
// Decimation is best-effort: a failure to delete one version is logged and
// does not abort the run.
type HistoryDecimationJob struct {
	store *Store
	now   time.Time
}

// NewHistoryDecimationJob creates a decimation job that will run against the
// given store's __history__ directory. The now parameter is normally
// time.Now().UTC() but is injectable for testing.
func NewHistoryDecimationJob(store *Store, now time.Time) *HistoryDecimationJob {
	return &HistoryDecimationJob{store: store, now: now}
}

// Execute implements the jobs.Job interface.
func (j *HistoryDecimationJob) Execute() error {
	if j.now.IsZero() {
		j.now = time.Now().UTC()
	} // Refresh for production; tests inject a fixed now.
	historyRoot := j.store.historyRoot()
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no history directory — nothing to decimate
		}
		return fmt.Errorf("failed to read history root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pageDir := filepath.Join(historyRoot, entry.Name())
		if err := j.decimatePageHistory(pageDir); err != nil {
			// Log and continue — one page's failure shouldn't abort the run.
			// TODO: log when Store has a logger.
			_ = err
		}
	}

	return nil
}

// GetName implements the jobs.Job interface.
func (*HistoryDecimationJob) GetName() string {
	return "HistoryDecimation"
}

// decimatePageHistory applies the retention schedule to a single page's
// history directory and deletes versions that fall outside retention.
func (j *HistoryDecimationJob) decimatePageHistory(pageDir string) error {
	versions, err := j.loadVersionTimestamps(pageDir)
	if err != nil {
		return fmt.Errorf("failed to load versions in %s: %w", pageDir, err)
	}

	if len(versions) == 0 {
		return nil
	}

	// Sort newest-first (already sorted by ULID descending, but be explicit).
	sort.Slice(versions, func(i, k int) bool {
		return versions[i].createdAt.After(versions[k].createdAt)
	})

	survivors := j.selectSurvivors(versions)

	// Delete versions not in the survivor set.
	survivorSet := make(map[string]bool, len(survivors))
	for _, v := range survivors {
		survivorSet[v.versionID] = true
	}

	for _, v := range versions {
		if !survivorSet[v.versionID] {
			if err := j.deleteVersion(pageDir, v.versionID); err != nil {
				// Log and continue — best-effort deletion.
				_ = err
			}
		}
	}

	return nil
}

// versionWithTimestamp pairs a version ID with its creation time.
type versionWithTimestamp struct {
	versionID string
	createdAt time.Time
}

// loadVersionTimestamps reads all .meta.json files in a page history directory
// and returns their version IDs and creation timestamps.
func (j *HistoryDecimationJob) loadVersionTimestamps(pageDir string) ([]versionWithTimestamp, error) {
	entries, err := os.ReadDir(pageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read page history dir: %w", err)
	}

	var versions []versionWithTimestamp
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, versionMetaExt) {
			continue
		}

		versionID := strings.TrimSuffix(name, versionMetaExt)
		meta, err := readVersionMetadataFile(pageDir, versionID)
		if err != nil {
			continue // skip unreadable metadata
		}

		versions = append(versions, versionWithTimestamp{
			versionID: versionID,
			createdAt: meta.CreatedAt,
		})
	}

	return versions, nil
}

// selectSurvivors applies the GFS retention schedule and returns the versions
// to keep. The input must be sorted newest-first.
func (j *HistoryDecimationJob) selectSurvivors(versions []versionWithTimestamp) []versionWithTimestamp {
	const (
		retentionRecentDays   = 7
		retentionWeeklyWeeks  = 26
		retentionMonthlyYears = 5
	)

	var survivors []versionWithTimestamp
	seenWeeks := make(map[string]bool)
	seenMonths := make(map[string]string) // month key -> versionID kept

	for _, v := range versions {
		age := j.now.Sub(v.createdAt)

		switch {
		case age <= retentionRecentDays*24*time.Hour:
			// Keep all versions within the recent window.
			survivors = append(survivors, v)

		case age <= retentionWeeklyWeeks*7*24*time.Hour:
			// Keep 1/week: the newest version in each ISO week.
			weekKey := isoWeekKey(v.createdAt)
			if !seenWeeks[weekKey] {
				seenWeeks[weekKey] = true
				survivors = append(survivors, v)
			}

		case age <= retentionMonthlyYears*365*24*time.Hour:
			// Keep 1/month: the newest version in each calendar month.
			monthKey := v.createdAt.Format("2006-01")
			if seenMonths[monthKey] == "" {
				seenMonths[monthKey] = v.versionID
				survivors = append(survivors, v)
			}

		default:
			// Older than 5 years — purge (do not add to survivors).
		}
	}

	return survivors
}

// deleteVersion removes both the .md and .meta.json files for a version.
func (j *HistoryDecimationJob) deleteVersion(pageDir, versionID string) error {
	contentPath := filepath.Join(pageDir, versionID+versionFileExt)
	if err := os.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete version content %s: %w", versionID, err)
	}

	metaPath := filepath.Join(pageDir, versionID+versionMetaExt)
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete version metadata %s: %w", versionID, err)
	}

	return nil
}

// isoWeekKey returns a string key identifying the ISO week of a timestamp
// (e.g. "2026-W30"). Used for the weekly retention bucket.
func isoWeekKey(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

// readVersionMetadataFile reads and parses a .meta.json file for a version.
// This is a package-level helper used by the decimation job to avoid
// depending on the Store receiver (which the job holds but doesn't need
// for this read).
func readVersionMetadataFile(dir, versionID string) (versionMetadataOnDisk, error) {
	metaPath := filepath.Join(dir, versionID+versionMetaExt)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return versionMetadataOnDisk{}, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta versionMetadataOnDisk
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return versionMetadataOnDisk{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return meta, nil
}
