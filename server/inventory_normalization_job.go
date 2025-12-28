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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DESIGN NOTE: Inventory uses a dual-representation model:
//  1. inventory.items array on containers - Used for UX (quick item entry in frontmatter)
//  2. inventory.container reference on items - Canonical child→parent relationship
//
// The normalization job reconciles these into eventual consistency.
// The items array is a UX convenience; the container reference is the source of truth.
// This allows users to quickly add items by editing a container's frontmatter,
// while the system maintains proper parent references on each item.

const (
	// AuditReportPage is the identifier for the inventory audit report page
	AuditReportPage = "inventory_audit_report"

	// InventoryNormalizationJobName is the name of the normalization job
	InventoryNormalizationJobName = "InventoryNormalizationJob"

	// frontmatter key paths
	inventoryContainerKeyPath   = "inventory.container"
	inventoryItemsKeyPath       = "inventory.items"
	inventoryIsContainerKeyPath = "inventory.is_container"
	newlineDelim                = "\n"
)

// InventoryNormalizationDependencies defines the interfaces needed for the normalization job.
type InventoryNormalizationDependencies interface {
	wikipage.PageReaderMutator
	wikipage.PageOpener
}

// InventoryNormalizationJob scans for inventory anomalies and creates missing item pages.
type InventoryNormalizationJob struct {
	normalizer *InventoryNormalizer
	deps       InventoryNormalizationDependencies
	fmIndex    frontmatter.IQueryFrontmatterIndex
	logger     lumber.Logger
}

// NewInventoryNormalizationJob creates a new inventory normalization job.
func NewInventoryNormalizationJob(
	deps InventoryNormalizationDependencies,
	fmIndex frontmatter.IQueryFrontmatterIndex,
	logger lumber.Logger,
) *InventoryNormalizationJob {
	return &InventoryNormalizationJob{
		normalizer: NewInventoryNormalizer(deps, logger),
		deps:       deps,
		fmIndex:    fmIndex,
		logger:     logger,
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

	// Step 1: Migrate containers to use is_container field
	migratedCount := j.migrateContainersToIsContainerField()
	if migratedCount > 0 {
		j.logger.Info("Migrated %d containers to use is_container field", migratedCount)
	}

	// Step 2: Find all containers
	containers := j.findAllContainers()
	j.logger.Info("Found %d containers to scan", len(containers))

	// Step 3: Create missing pages and collect anomalies
	createdPages, creationAnomalies := j.createMissingItemPages(containers)

	// Step 4: Remove items from parent containers' items lists
	// When an item has inventory.container set, remove it from the parent's inventory.items array
	removedCount := j.removeItemsFromParentContainers(containers)
	if removedCount > 0 {
		j.logger.Info("Removed %d items from parent container items lists", removedCount)
	}

	// Step 5: Detect all anomalies
	anomalies := creationAnomalies
	anomalies = append(anomalies, j.detectMultipleContainerAnomalies(containers)...)
	anomalies = append(anomalies, j.detectCircularReferenceAnomalies(containers)...)
	anomalies = append(anomalies, j.detectOrphanedItems()...)

	// Step 6: Generate audit report page
	if err := j.generateAuditReport(anomalies, createdPages); err != nil {
		j.logger.Error("Failed to generate audit report: %v", err)
	}

	j.logger.Info("Inventory normalization complete: %d pages created, %d anomalies detected", len(createdPages), len(anomalies))
	return nil
}

// createMissingItemPages creates pages for items that don't have their own page yet.
func (j *InventoryNormalizationJob) createMissingItemPages(containers []string) ([]string, []InventoryAnomaly) {
	var createdPages []string
	var anomalies []InventoryAnomaly

	for _, containerID := range containers {
		items := j.normalizer.GetContainerItems(containerID)
		for _, itemID := range items {
			_, _, err := j.deps.ReadFrontMatter(itemID)
			if err == nil {
				continue // Page exists
			}

			if createErr := j.normalizer.CreateItemPage(itemID, containerID); createErr != nil {
				j.logger.Error("Failed to create page for item %s: %v", itemID, createErr)
				anomalies = append(anomalies, InventoryAnomaly{
					Type:        "page_creation_failed",
					ItemID:      itemID,
					Description: fmt.Sprintf("Failed to create page for item '%s' in container '%s': %v", itemID, containerID, createErr),
					Containers:  []string{containerID},
					Severity:    "error",
				})
			} else {
				createdPages = append(createdPages, itemID)
				j.logger.Info("Created page for item: %s in container: %s", itemID, containerID)
			}
		}
	}

	return createdPages, anomalies
}

// detectMultipleContainerAnomalies finds items that appear in multiple containers.
func (j *InventoryNormalizationJob) detectMultipleContainerAnomalies(containers []string) []InventoryAnomaly {
	itemContainers := j.buildItemContainerMap(containers)

	var anomalies []InventoryAnomaly
	for itemID, containerSet := range itemContainers {
		if len(containerSet) <= 1 {
			continue
		}
		containerList := mapKeysToSortedSlice(containerSet)
		anomalies = append(anomalies, InventoryAnomaly{
			Type:        "multiple_containers",
			ItemID:      itemID,
			Description: fmt.Sprintf("Item '%s' is referenced in multiple containers: %v", itemID, containerList),
			Containers:  containerList,
			Severity:    "warning",
		})
	}
	return anomalies
}

// buildItemContainerMap builds a map of item IDs to the containers they belong to.
func (j *InventoryNormalizationJob) buildItemContainerMap(containers []string) map[string]map[string]bool {
	itemContainers := make(map[string]map[string]bool)

	for _, containerID := range containers {
		// Source 1: Items with inventory.container set to this container
		for _, itemID := range j.getItemsWithContainerReference(containerID) {
			if itemContainers[itemID] == nil {
				itemContainers[itemID] = make(map[string]bool)
			}
			itemContainers[itemID][containerID] = true
		}

		// Source 2: Items listed in this container's inventory.items array
		for _, itemID := range j.normalizer.GetContainerItems(containerID) {
			if itemContainers[itemID] == nil {
				itemContainers[itemID] = make(map[string]bool)
			}
			itemContainers[itemID][containerID] = true
		}
	}

	return itemContainers
}

// mapKeysToSortedSlice converts map keys to a sorted slice.
func mapKeysToSortedSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// detectCircularReferenceAnomalies wraps detectCircularReferences for the Execute flow.
func (j *InventoryNormalizationJob) detectCircularReferenceAnomalies(containers []string) []InventoryAnomaly {
	circularRefs := j.detectCircularReferences(containers)
	anomalies := make([]InventoryAnomaly, 0, len(circularRefs))
	for _, ref := range circularRefs {
		anomalies = append(anomalies, InventoryAnomaly{
			Type:        "circular_reference",
			ItemID:      ref.ItemID,
			Description: ref.Description,
			Containers:  ref.Containers,
			Severity:    "error",
		})
	}
	return anomalies
}

// detectOrphanedItems finds items that reference non-existent containers.
func (j *InventoryNormalizationJob) detectOrphanedItems() []InventoryAnomaly {
	var anomalies []InventoryAnomaly

	allItemsWithContainer := j.fmIndex.QueryKeyExistence(inventoryContainerKeyPath)
	for _, itemID := range allItemsWithContainer {
		containerRef := j.fmIndex.GetValue(itemID, inventoryContainerKeyPath)
		if containerRef == "" {
			continue
		}
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

	return anomalies
}

// GetName returns the job name.
func (*InventoryNormalizationJob) GetName() string {
	return InventoryNormalizationJobName
}

// GetNormalizer returns the underlying normalizer for testing purposes.
func (j *InventoryNormalizationJob) GetNormalizer() *InventoryNormalizer {
	return j.normalizer
}

// findAllContainers finds all pages that act as containers.
func (j *InventoryNormalizationJob) findAllContainers() []string {
	containerSet := make(map[string]bool)

	// Source 1: Pages with explicit is_container = true
	pagesWithIsContainer := j.fmIndex.QueryKeyExistence(inventoryIsContainerKeyPath)
	for _, pageID := range pagesWithIsContainer {
		if j.fmIndex.GetValue(pageID, inventoryIsContainerKeyPath) == "true" {
			containerSet[pageID] = true
		}
	}

	// Source 2: Pages with inventory.items (legacy containers)
	pagesWithItems := j.fmIndex.QueryKeyExistence(inventoryItemsKeyPath)
	for _, pageID := range pagesWithItems {
		containerSet[pageID] = true
	}

	// Source 3: Pages referenced as inventory.container by other items
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

// migrateContainersToIsContainerField finds containers that don't have is_container set
// and adds it to their frontmatter. This migrates legacy containers that were identified
// by having items reference them or by having an inventory.items array.
func (j *InventoryNormalizationJob) migrateContainersToIsContainerField() int {
	migratedCount := 0

	// Find pages that are referenced as containers by other items
	containerSet := make(map[string]bool)
	pagesWithContainer := j.fmIndex.QueryKeyExistence(inventoryContainerKeyPath)
	for _, pageID := range pagesWithContainer {
		containerRef := j.fmIndex.GetValue(pageID, inventoryContainerKeyPath)
		if containerRef != "" {
			containerSet[containerRef] = true
		}
	}

	// Also include pages that have non-empty inventory.items arrays
	pagesWithItems := j.fmIndex.QueryKeyExistence(inventoryItemsKeyPath)
	for _, pageID := range pagesWithItems {
		// Check if the page has actual items in its inventory.items array
		items := j.normalizer.GetContainerItems(pageID)
		if len(items) > 0 {
			containerSet[pageID] = true
		}
	}

	// For each identified container, check if it needs migration
	for containerID := range containerSet {
		// Read frontmatter to check current state
		_, fm, err := j.deps.ReadFrontMatter(containerID)
		if err != nil {
			j.logger.Error("Failed to read frontmatter for container %s during migration: %v", containerID, err)
			continue
		}

		// Check if is_container is already set to true
		if isContainerAlreadySet(fm) {
			continue
		}

		// Ensure inventory map exists
		inventory, ok := fm["inventory"].(map[string]any)
		if !ok {
			inventory = make(map[string]any)
			fm["inventory"] = inventory
		}

		// Set is_container = true
		inventory["is_container"] = true

		// Write back frontmatter
		if err := j.deps.WriteFrontMatter(containerID, fm); err != nil {
			j.logger.Error("Failed to write frontmatter for container %s during migration: %v", containerID, err)
			continue
		}

		migratedCount++
	}

	return migratedCount
}

// isContainerAlreadySet checks if is_container is already set to true.
// Handles both boolean true and string "true" values.
func isContainerAlreadySet(fm map[string]any) bool {
	inventory, ok := fm["inventory"].(map[string]any)
	if !ok {
		return false
	}

	isContainer := inventory["is_container"]
	if isContainer == nil {
		return false
	}

	// Check for boolean true
	if b, ok := isContainer.(bool); ok {
		return b
	}

	// Check for string "true"
	if s, ok := isContainer.(string); ok {
		return s == "true"
	}

	return false
}

// getItemsWithContainerReference gets items that have inventory.container set to this container.
func (j *InventoryNormalizationJob) getItemsWithContainerReference(containerID string) []string {
	return j.fmIndex.QueryExactMatch(inventoryContainerKeyPath, containerID)
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

// removeItemsFromParentContainers removes items from their parent containers' items lists.
// This is called after items have been created/updated to ensure the items list is consistent
// with the canonical container references on the items.
//
// This function reads frontmatter directly rather than using the index to avoid race conditions
// where newly created items haven't been indexed yet.
func (j *InventoryNormalizationJob) removeItemsFromParentContainers(containers []string) int {
	removedCount := 0

	for _, containerID := range containers {
		// Read the container's frontmatter
		_, containerFm, err := j.deps.ReadFrontMatter(containerID)
		if err != nil {
			// Container doesn't exist, skip
			continue
		}

		inventory, ok := containerFm["inventory"].(map[string]any)
		if !ok {
			// No inventory section, skip
			continue
		}

		itemsRaw, ok := inventory["items"]
		if !ok {
			// No items array, skip
			continue
		}

		// Handle both []string and []any
		var items []any
		switch v := itemsRaw.(type) {
		case []string:
			for _, item := range v {
				items = append(items, item)
			}
		case []any:
			items = v
		default:
			// Unknown type, skip
			continue
		}

		if len(items) == 0 {
			// Empty items array, skip
			continue
		}

		// Build a set of munged item IDs that have this container reference
		// by checking each item's frontmatter directly (avoids index race conditions)
		itemsWithContainerSet := make(map[string]bool)
		for _, item := range items {
			if s, ok := item.(string); ok {
				mungedItemID := wikiidentifiers.MungeIdentifier(s)
				
				// Read the item's frontmatter to check if it has this container reference
				_, itemFm, err := j.deps.ReadFrontMatter(mungedItemID)
				if err != nil {
					// Item doesn't exist yet, skip
					continue
				}

				itemInventory, ok := itemFm["inventory"].(map[string]any)
				if !ok {
					continue
				}

				containerRef, ok := itemInventory["container"].(string)
				if !ok {
					continue
				}

				// Check if this item's container reference matches this container
				mungedContainerRef := wikiidentifiers.MungeIdentifier(containerRef)
				mungedContainerID := wikiidentifiers.MungeIdentifier(containerID)
				if mungedContainerRef == mungedContainerID {
					itemsWithContainerSet[mungedItemID] = true
				}
			}
		}

		if len(itemsWithContainerSet) == 0 {
			// No items to remove
			continue
		}

		// Filter out items that have the container reference
		var newItems []any
		for _, item := range items {
			if s, ok := item.(string); ok {
				mungedItem := wikiidentifiers.MungeIdentifier(s)
				if itemsWithContainerSet[mungedItem] {
					// Skip this item (remove it)
					removedCount++
					j.logger.Info("Removed item '%s' from container '%s' items list", s, containerID)
					continue
				}
			}
			newItems = append(newItems, item)
		}

		// If nothing changed, don't write
		if len(newItems) == len(items) {
			continue
		}

		// Update the items array
		inventory["items"] = newItems

		// Write back the frontmatter
		if err := j.deps.WriteFrontMatter(containerID, containerFm); err != nil {
			j.logger.Error("Failed to write frontmatter for container %s: %v", containerID, err)
		}
	}

	return removedCount
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
	)

	return scheduler.Schedule(schedule, job)
}
