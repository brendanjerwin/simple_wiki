package server

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
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
	inventoryKey                = "inventory"
	newlineDelim                = "\n"
)

// InventoryNormalizationDependencies defines the interfaces needed for the normalization job.
// Uses only PageReaderMutator since ReadPage is not required for normalization operations.
type InventoryNormalizationDependencies interface {
	wikipage.PageReaderMutator
}

// UnexpectedIsContainerTypeError is returned when is_container has an unexpected type.
type UnexpectedIsContainerTypeError struct {
	ActualType string
	Value      any
}

func (e *UnexpectedIsContainerTypeError) Error() string {
	return fmt.Sprintf("unexpected type for is_container: got %s with value %v", e.ActualType, e.Value)
}

// InventoryNormalizationJob scans for inventory anomalies and creates missing item pages.
type InventoryNormalizationJob struct {
	normalizer *InventoryNormalizer
	deps       InventoryNormalizationDependencies
	fmIndex    frontmatter.IQueryFrontmatterIndex
	logger     lumber.Logger
}

// NewInventoryNormalizationJob creates a new inventory normalization job.
// Returns an error if any required dependency is nil.
func NewInventoryNormalizationJob(
	deps InventoryNormalizationDependencies,
	fmIndex frontmatter.IQueryFrontmatterIndex,
	logger lumber.Logger,
) (*InventoryNormalizationJob, error) {
	if deps == nil {
		return nil, errors.New("deps is required")
	}
	if fmIndex == nil {
		return nil, errors.New("fmIndex is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	return &InventoryNormalizationJob{
		normalizer: NewInventoryNormalizer(deps, logger),
		deps:       deps,
		fmIndex:    fmIndex,
		logger:     logger,
	}, nil
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
		return fmt.Errorf("failed to generate audit report: %w", err)
	}

	j.logger.Info("Inventory normalization complete: %d pages created, %d anomalies detected", len(createdPages), len(anomalies))
	return nil
}

// createMissingItemPages creates pages for items that don't have their own page yet.
func (j *InventoryNormalizationJob) createMissingItemPages(containers []string) ([]string, []InventoryAnomaly) {
	var createdPages []string
	var anomalies []InventoryAnomaly

	for _, containerID := range containers {
		items, err := j.normalizer.GetContainerItems(containerID)
		if err != nil {
			j.logger.Error("Failed to get container items for %s: %v", containerID, err)
			anomalies = append(anomalies, InventoryAnomaly{
				Type:        "invalid_item_identifier",
				ItemID:      containerID,
				Description: fmt.Sprintf("Container '%s' has invalid item identifier: %v", containerID, err),
				Containers:  []string{containerID},
				Severity:    "error",
			})
			continue
		}
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
		containerItems, err := j.normalizer.GetContainerItems(containerID)
		if err != nil {
			j.logger.Error("Failed to get container items for %s: %v", containerID, err)
			continue
		}
		for _, itemID := range containerItems {
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
		items, err := j.normalizer.GetContainerItems(pageID)
		if err != nil {
			j.logger.Error("Failed to get container items for %s: %v", pageID, err)
			continue
		}
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
		alreadySet, err := isContainerAlreadySet(fm)
		if err != nil {
			j.logger.Warn("Container %s has unexpected is_container type: %v", containerID, err)
			// Treat unexpected type as not set - will be overwritten with boolean true
		}
		if alreadySet {
			continue
		}

		// Ensure inventory map exists
		inventory, ok := fm[inventoryKey].(map[string]any)
		if !ok {
			inventory = make(map[string]any)
			fm[inventoryKey] = inventory
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
// Returns an UnexpectedIsContainerTypeError if is_container has an unexpected type.
func isContainerAlreadySet(fm map[string]any) (bool, error) {
	inventory, ok := fm[inventoryKey].(map[string]any)
	if !ok {
		return false, nil
	}

	isContainer := inventory["is_container"]
	if isContainer == nil {
		return false, nil
	}

	// Check for boolean
	if b, ok := isContainer.(bool); ok {
		return b, nil
	}

	// Check for string "true" or "false"
	if s, ok := isContainer.(string); ok {
		return s == "true", nil
	}

	// Unexpected type - return typed error
	return false, &UnexpectedIsContainerTypeError{
		ActualType: fmt.Sprintf("%T", isContainer),
		Value:      isContainer,
	}
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
// Note: Creates explicit copies of slices to avoid shared backing array issues with append.
func (j *InventoryNormalizationJob) findCycle(containerID string, visited map[string]bool, path []string) []string {
	if visited[containerID] {
		// Found a cycle - find where the cycle starts
		for i, id := range path {
			if id == containerID {
				// Create explicit copy to avoid shared backing array
				cycle := make([]string, len(path[i:])+1)
				copy(cycle, path[i:])
				cycle[len(cycle)-1] = containerID
				return cycle
			}
		}
		return nil
	}

	visited[containerID] = true

	// Create explicit copy of path to avoid shared backing array issues
	newPath := make([]string, len(path)+1)
	copy(newPath, path)
	newPath[len(newPath)-1] = containerID

	// Get the container's parent
	parentContainer := j.fmIndex.GetValue(containerID, inventoryContainerKeyPath)
	if parentContainer != "" {
		return j.findCycle(parentContainer, visited, newPath)
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
//
// Errors during individual container processing are logged but don't stop processing of other containers.
func (j *InventoryNormalizationJob) removeItemsFromParentContainers(containers []string) int {
	removedCount := 0

	for _, containerID := range containers {
		removed, err := j.processContainerForItemRemoval(containerID)
		if err != nil {
			j.logger.Error("Failed to process container %s for item removal: %v", containerID, err)
			continue
		}
		removedCount += removed
	}

	return removedCount
}

// processContainerForItemRemoval processes a single container and removes items that have explicit container references.
// Returns the count of removed items and any error from the operation.
func (j *InventoryNormalizationJob) processContainerForItemRemoval(containerID string) (int, error) {
	// Read the container's frontmatter
	_, containerFm, err := j.deps.ReadFrontMatter(containerID)
	if err != nil {
		return 0, nil // Container doesn't exist, not an error for this operation
	}

	inventory, ok := containerFm[inventoryKey].(map[string]any)
	if !ok {
		return 0, nil // No inventory section, nothing to do
	}

	items, ok := j.extractItemsArray(inventory)
	if !ok || len(items) == 0 {
		return 0, nil // No items array or empty, nothing to do
	}

	// Build a set of items that should be removed
	itemsToRemove := j.findItemsWithContainerReference(containerID, items)
	if len(itemsToRemove) == 0 {
		return 0, nil // No items to remove
	}

	// Filter out items and write back
	return j.removeAndWriteItems(containerID, containerFm, inventory, items, itemsToRemove)
}

// extractItemsArray extracts and normalizes the items array from inventory frontmatter.
func (*InventoryNormalizationJob) extractItemsArray(inventory map[string]any) ([]any, bool) {
	itemsRaw, ok := inventory["items"]
	if !ok {
		return nil, false
	}

	// Handle both []string and []any
	switch v := itemsRaw.(type) {
	case []string:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = item
		}
		return items, true
	case []any:
		return v, true
	default:
		return nil, false
	}
}

// findItemsWithContainerReference builds a set of munged item IDs that reference this container.
// Note: This creates an N+1 read pattern, but it's necessary for correctness
// since the index may not yet include newly created items. This tradeoff is
// acceptable because: (1) the job runs periodically, not per-request, and
// (2) containers typically have a manageable number of items.
func (j *InventoryNormalizationJob) findItemsWithContainerReference(containerID string, items []any) map[string]bool {
	itemsWithContainerSet := make(map[string]bool)
	mungedContainerID, err := wikiidentifiers.MungeIdentifier(containerID)
	if err != nil {
		j.logger.Error("Invalid container identifier %q: %v", containerID, err)
		return itemsWithContainerSet
	}

	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			continue
		}

		mungedItemID, err := wikiidentifiers.MungeIdentifier(s)
		if err != nil {
			j.logger.Error("Invalid item identifier %q: %v", s, err)
			continue
		}
		if j.itemReferencesContainer(mungedItemID, mungedContainerID) {
			itemsWithContainerSet[mungedItemID] = true
		}
	}

	return itemsWithContainerSet
}

// itemReferencesContainer checks if an item's frontmatter references the given container.
func (j *InventoryNormalizationJob) itemReferencesContainer(itemID, containerID string) bool {
	_, itemFm, err := j.deps.ReadFrontMatter(itemID)
	if err != nil {
		return false
	}

	itemInventory, ok := itemFm[inventoryKey].(map[string]any)
	if !ok {
		return false
	}

	containerRef, ok := itemInventory["container"].(string)
	if !ok {
		return false
	}

	mungedContainerRef, err := wikiidentifiers.MungeIdentifier(containerRef)
	if err != nil {
		return false
	}
	return mungedContainerRef == containerID
}

// removeAndWriteItems removes items from the list and writes back the updated frontmatter.
// Returns the count of removed items and any error from writing frontmatter.
func (j *InventoryNormalizationJob) removeAndWriteItems(containerID string, containerFm map[string]any, inventory map[string]any, items []any, itemsToRemove map[string]bool) (int, error) {
	removedCount := 0
	var newItems []any

	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			newItems = append(newItems, item)
			continue
		}

		mungedItem, err := wikiidentifiers.MungeIdentifier(s)
		if err != nil {
			// Invalid identifier, keep it as-is
			newItems = append(newItems, item)
			continue
		}
		if itemsToRemove[mungedItem] {
			removedCount++
			j.logger.Info("Removed item '%s' from container '%s' items list", s, containerID)
			continue
		}
		newItems = append(newItems, item)
	}

	if removedCount == 0 {
		return 0, nil
	}

	// Update and write back
	inventory["items"] = newItems
	if err := j.deps.WriteFrontMatter(containerID, containerFm); err != nil {
		return 0, fmt.Errorf("failed to write frontmatter for container %s: %w", containerID, err)
	}

	return removedCount, nil
}

// ScheduleInventoryNormalization schedules the inventory normalization job on the cron scheduler.
func ScheduleInventoryNormalization(
	scheduler *jobs.CronScheduler,
	site *Site,
	schedule string,
) (int, error) {
	job, err := NewInventoryNormalizationJob(
		site,
		site.FrontmatterIndexQueryer,
		site.Logger,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create inventory normalization job: %w", err)
	}

	return scheduler.Schedule(schedule, job)
}
