package server

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// AuditReportPage is the identifier for the inventory audit report page
	AuditReportPage = "inventory_audit_report"

	// InventoryNormalizationJobName is the name of the normalization job
	InventoryNormalizationJobName = "InventoryNormalizationJob"

	// frontmatter key paths
	inventoryContainerKeyPath = "inventory.container"
	inventoryItemsKeyPath     = "inventory.items"
	newlineDelim              = "\n"
)

// InventoryNormalizationDependencies defines the interfaces needed for the normalization job.
type InventoryNormalizationDependencies interface {
	wikipage.PageReaderMutator
	wikipage.PageOpener
}

// InventoryNormalizationJob scans for inventory anomalies and creates missing item pages.
type InventoryNormalizationJob struct {
	deps          InventoryNormalizationDependencies
	fmIndex       frontmatter.IQueryFrontmatterIndex
	logger        lumber.Logger
	jobCoordinator *jobs.JobQueueCoordinator
}

// NewInventoryNormalizationJob creates a new inventory normalization job.
func NewInventoryNormalizationJob(
	deps InventoryNormalizationDependencies,
	fmIndex frontmatter.IQueryFrontmatterIndex,
	logger lumber.Logger,
	coordinator *jobs.JobQueueCoordinator,
) *InventoryNormalizationJob {
	return &InventoryNormalizationJob{
		deps:          deps,
		fmIndex:       fmIndex,
		logger:        logger,
		jobCoordinator: coordinator,
	}
}

// InventoryAnomaly represents a detected anomaly in the inventory system.
type InventoryAnomaly struct {
	Type        string   // "orphan", "multiple_containers", "circular_reference", "missing_page"
	ItemID      string   // The item affected
	Description string   // Human-readable description
	Containers  []string // Containers involved (for multiple_containers)
	Severity    string   // "warning", "error"
}

// Execute runs the inventory normalization job.
//
//revive:disable:function-length
func (j *InventoryNormalizationJob) Execute() error {
	j.logger.Info("Starting inventory normalization job")

	var anomalies []InventoryAnomaly
	var createdPages []string

	// Step 1: Find all containers (pages that have inventory.items or are referenced as inventory.container)
	containers := j.findAllContainers()
	j.logger.Info("Found %d containers to scan", len(containers))

	// Step 2: Scan each container for items that need pages created
	for _, containerID := range containers {
		items := j.getContainerItems(containerID)
		for _, itemID := range items {
			// Check if the item has its own page
			_, _, err := j.deps.ReadFrontMatter(itemID)
			if err != nil {
				// Page doesn't exist - create it
				if err := j.createItemPage(itemID, containerID); err != nil {
					j.logger.Error("Failed to create page for item %s: %v", itemID, err)
					anomalies = append(anomalies, InventoryAnomaly{
						Type:        "page_creation_failed",
						ItemID:      itemID,
						Description: fmt.Sprintf("Failed to create page for item '%s' in container '%s': %v", itemID, containerID, err),
						Containers:  []string{containerID},
						Severity:    "error",
					})
				} else {
					createdPages = append(createdPages, itemID)
					j.logger.Info("Created page for item: %s in container: %s", itemID, containerID)
				}
			}
		}
	}

	// Step 3: Detect anomalies
	// 3a: Items in multiple containers
	itemContainers := make(map[string][]string) // item -> containers
	for _, containerID := range containers {
		items := j.getItemsWithContainerReference(containerID)
		for _, itemID := range items {
			itemContainers[itemID] = append(itemContainers[itemID], containerID)
		}
	}

	for itemID, containers := range itemContainers {
		if len(containers) > 1 {
			anomalies = append(anomalies, InventoryAnomaly{
				Type:        "multiple_containers",
				ItemID:      itemID,
				Description: fmt.Sprintf("Item '%s' is referenced in multiple containers: %v", itemID, containers),
				Containers:  containers,
				Severity:    "warning",
			})
		}
	}

	// 3b: Circular references
	circularRefs := j.detectCircularReferences(containers)
	for _, ref := range circularRefs {
		anomalies = append(anomalies, InventoryAnomaly{
			Type:        "circular_reference",
			ItemID:      ref.ItemID,
			Description: ref.Description,
			Containers:  ref.Containers,
			Severity:    "error",
		})
	}

	// 3c: Orphaned items (have inventory.container set but container doesn't exist)
	allItemsWithContainer := j.fmIndex.QueryKeyExistence(inventoryContainerKeyPath)
	for _, itemID := range allItemsWithContainer {
		containerRef := j.fmIndex.GetValue(itemID, inventoryContainerKeyPath)
		if containerRef != "" {
			// Check if container page exists
			_, _, err := j.deps.ReadFrontMatter(containerRef)
			if err != nil {
				anomalies = append(anomalies, InventoryAnomaly{
					Type:        "orphan",
					ItemID:      itemID,
					Description: fmt.Sprintf("Item '%s' references non-existent container '%s'", itemID, containerRef),
					Containers:  []string{containerRef},
					Severity:    "warning",
				})
			}
		}
	}

	// Step 4: Generate audit report page
	if err := j.generateAuditReport(anomalies, createdPages); err != nil {
		j.logger.Error("Failed to generate audit report: %v", err)
	}

	j.logger.Info("Inventory normalization complete: %d pages created, %d anomalies detected", len(createdPages), len(anomalies))
	return nil
}

// GetName returns the job name.
func (*InventoryNormalizationJob) GetName() string {
	return InventoryNormalizationJobName
}

// findAllContainers finds all pages that act as containers.
func (j *InventoryNormalizationJob) findAllContainers() []string {
	containerSet := make(map[string]bool)

	// Pages with inventory.items
	pagesWithItems := j.fmIndex.QueryKeyExistence(inventoryItemsKeyPath)
	for _, pageID := range pagesWithItems {
		containerSet[pageID] = true
	}

	// Pages referenced as inventory.container
	pagesWithContainer := j.fmIndex.QueryKeyExistence(inventoryContainerKeyPath)
	for _, pageID := range pagesWithContainer {
		containerRef := j.fmIndex.GetValue(pageID, inventoryContainerKeyPath)
		if containerRef != "" {
			containerSet[containerRef] = true
		}
	}

	containers := make([]string, 0, len(containerSet))
	for containerID := range containerSet {
		containers = append(containers, containerID)
	}
	sort.Strings(containers)
	return containers
}

// getContainerItems gets items listed in a container's inventory.items array.
func (j *InventoryNormalizationJob) getContainerItems(containerID string) []string {
	_, fm, err := j.deps.ReadFrontMatter(containerID)
	if err != nil {
		return nil
	}

	inventory, ok := fm["inventory"].(map[string]any)
	if !ok {
		return nil
	}

	itemsRaw, ok := inventory["items"]
	if !ok {
		return nil
	}

	// Handle both []string and []any
	var items []string
	switch v := itemsRaw.(type) {
	case []string:
		items = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, wikiidentifiers.MungeIdentifier(s))
			}
		}
	}

	return items
}

// getItemsWithContainerReference gets items that have inventory.container set to this container.
func (j *InventoryNormalizationJob) getItemsWithContainerReference(containerID string) []string {
	return j.fmIndex.QueryExactMatch(inventoryContainerKeyPath, containerID)
}

// createItemPage creates a new inventory item page.
func (j *InventoryNormalizationJob) createItemPage(itemID, containerID string) error {
	identifier := wikiidentifiers.MungeIdentifier(itemID)

	// Build frontmatter
	fm := make(map[string]any)
	fm["identifier"] = identifier

	// Generate title from identifier
	titleCaser := cases.Title(language.AmericanEnglish)
	snaked := strcase.SnakeCase(identifier)
	// Replace underscores with spaces for a nicer title
	titleStr := strings.ReplaceAll(snaked, "_", " ")
	fm["title"] = titleCaser.String(titleStr)

	// Set up inventory structure
	inventory := make(map[string]any)
	if containerID != "" {
		inventory["container"] = wikiidentifiers.MungeIdentifier(containerID)
	}
	inventory["items"] = []string{}
	fm["inventory"] = inventory

	// Write frontmatter
	if err := j.deps.WriteFrontMatter(identifier, fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// Build and write markdown
	markdown := buildNormalizationItemMarkdown()
	if err := j.deps.WriteMarkdown(identifier, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	return nil
}

// buildNormalizationItemMarkdown creates the markdown content for an inventory item page.
func buildNormalizationItemMarkdown() string {
	var builder bytes.Buffer
	_, _ = builder.WriteString("# {{or .Title .Identifier}}" + newlineDelim)
	_, _ = builder.WriteString(inventoryItemMarkdownTemplate)
	return builder.String()
}

// detectCircularReferences detects circular references in the container hierarchy.
func (j *InventoryNormalizationJob) detectCircularReferences(containers []string) []InventoryAnomaly {
	var anomalies []InventoryAnomaly

	for _, containerID := range containers {
		visited := make(map[string]bool)
		path := []string{}
		if cycle := j.findCycle(containerID, visited, path); len(cycle) > 0 {
			// Only report each cycle once (from the first item in the cycle)
			if cycle[0] == containerID {
				anomalies = append(anomalies, InventoryAnomaly{
					Type:        "circular_reference",
					ItemID:      containerID,
					Description: fmt.Sprintf("Circular reference detected: %s", strings.Join(cycle, " -> ")),
					Containers:  cycle,
					Severity:    "error",
				})
			}
		}
	}

	return anomalies
}

// findCycle finds a cycle starting from the given container.
func (j *InventoryNormalizationJob) findCycle(containerID string, visited map[string]bool, path []string) []string {
	if visited[containerID] {
		// Found a cycle - find where the cycle starts
		for i, id := range path {
			if id == containerID {
				cycle := append(path[i:], containerID)
				return cycle
			}
		}
		return nil
	}

	visited[containerID] = true
	path = append(path, containerID)

	// Get the container's parent
	parentContainer := j.fmIndex.GetValue(containerID, inventoryContainerKeyPath)
	if parentContainer != "" {
		return j.findCycle(parentContainer, visited, path)
	}

	return nil
}

// generateAuditReport creates or updates the audit report page.
func (j *InventoryNormalizationJob) generateAuditReport(anomalies []InventoryAnomaly, createdPages []string) error {
	var report bytes.Buffer

	_, _ = report.WriteString("# Inventory Audit Report" + newlineDelim + newlineDelim)
	_, _ = fmt.Fprintf(&report, "*Last updated: %s*"+newlineDelim+newlineDelim, time.Now().Format(time.RFC3339))

	// Summary
	_, _ = report.WriteString("## Summary" + newlineDelim + newlineDelim)
	_, _ = fmt.Fprintf(&report, "- **Pages created this run:** %d"+newlineDelim, len(createdPages))
	_, _ = fmt.Fprintf(&report, "- **Anomalies detected:** %d"+newlineDelim+newlineDelim, len(anomalies))

	// Created Pages
	if len(createdPages) > 0 {
		_, _ = report.WriteString("## Pages Created" + newlineDelim + newlineDelim)
		for _, pageID := range createdPages {
			_, _ = fmt.Fprintf(&report, "- [[%s]]"+newlineDelim, pageID)
		}
		_, _ = report.WriteString(newlineDelim)
	}

	// Anomalies
	if len(anomalies) > 0 {
		_, _ = report.WriteString("## Anomalies" + newlineDelim + newlineDelim)

		// Group by type
		byType := make(map[string][]InventoryAnomaly)
		for _, a := range anomalies {
			byType[a.Type] = append(byType[a.Type], a)
		}

		for anomalyType, items := range byType {
			_, _ = fmt.Fprintf(&report, "### %s"+newlineDelim+newlineDelim, formatAnomalyType(anomalyType))
			for _, a := range items {
				severity := "⚠️"
				if a.Severity == "error" {
					severity = "❌"
				}
				_, _ = fmt.Fprintf(&report, "%s **%s**: %s"+newlineDelim+newlineDelim, severity, a.ItemID, a.Description)
			}
		}
	} else {
		_, _ = report.WriteString("## Anomalies" + newlineDelim + newlineDelim)
		_, _ = report.WriteString("✅ No anomalies detected." + newlineDelim + newlineDelim)
	}

	// Build frontmatter
	fm := map[string]any{
		"identifier": AuditReportPage,
		"title":      "Inventory Audit Report",
	}

	// Build full page content with frontmatter
	fmBytes, err := toml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	var fullPage bytes.Buffer
	_, _ = fullPage.WriteString(tomlDelimiter)
	_, _ = fullPage.Write(fmBytes)
	if !bytes.HasSuffix(fmBytes, []byte(newlineDelim)) {
		_, _ = fullPage.WriteString(newlineDelim)
	}
	_, _ = fullPage.WriteString(tomlDelimiter)
	_, _ = fullPage.WriteString(newlineDelim)
	_, _ = fullPage.Write(report.Bytes())

	// Write frontmatter and markdown
	if err := j.deps.WriteFrontMatter(AuditReportPage, fm); err != nil {
		return fmt.Errorf("failed to write audit report frontmatter: %w", err)
	}
	if err := j.deps.WriteMarkdown(AuditReportPage, report.String()); err != nil {
		return fmt.Errorf("failed to write audit report markdown: %w", err)
	}

	return nil
}

// formatAnomalyType converts anomaly type to human-readable format.
func formatAnomalyType(t string) string {
	switch t {
	case "orphan":
		return "Orphaned Items"
	case "multiple_containers":
		return "Items in Multiple Containers"
	case "circular_reference":
		return "Circular References"
	case "missing_page":
		return "Missing Pages"
	case "page_creation_failed":
		return "Page Creation Failures"
	default:
		titleCaser := cases.Title(language.AmericanEnglish)
		return titleCaser.String(strings.ReplaceAll(t, "_", " "))
	}
}

// ScheduleInventoryNormalization schedules the inventory normalization job on the cron scheduler.
func ScheduleInventoryNormalization(
	scheduler *jobs.CronScheduler,
	site *Site,
	schedule string,
) (int, error) {
	job := NewInventoryNormalizationJob(
		site,
		site.FrontmatterIndexQueryer,
		site.Logger,
		site.JobQueueCoordinator,
	)

	return scheduler.Schedule(schedule, job)
}
