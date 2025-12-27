package v1

import (
	"context"
	"fmt"
	"os"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/inventory"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ERROR HANDLING CONTRACT:
// - gRPC errors (codes.Internal, codes.InvalidArgument, etc.): Infrastructure/unexpected failures
// - Response with Success=false: Domain-level "can't do that" (item not found, already exists, etc.)
//
// Clients should handle gRPC errors as exceptional, Success=false as normal control flow.

const (
	inventoryKey        = "inventory"
	containerKey        = "container"
	itemsKey            = "items"
	titleKey            = "title"
	descriptionKey      = "description"
	newlineConst        = "\n"
	defaultMaxRecursion = 10
)

// CreateInventoryItem implements the CreateInventoryItem RPC.
func (s *Server) CreateInventoryItem(_ context.Context, req *apiv1.CreateInventoryItemRequest) (*apiv1.CreateInventoryItemResponse, error) {
	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	// Munge the identifier to ensure consistency
	identifier := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)

	// Check if page already exists
	_, existingFm, err := s.pageReaderMutator.ReadFrontMatter(identifier)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	if existingFm != nil {
		return &apiv1.CreateInventoryItemResponse{
			Success:        false,
			ItemIdentifier: identifier,
			Error:          fmt.Sprintf("page already exists: %s", identifier),
			Summary:        fmt.Sprintf("Could not create item '%s' because it already exists.", identifier),
		}, nil
	}

	// Build frontmatter
	fm := make(map[string]any)
	fm[identifierKey] = identifier

	// Set title - auto-generate from identifier if not provided
	title := req.Title
	if title == "" {
		titleCaser := cases.Title(language.AmericanEnglish)
		snaked := strcase.SnakeCase(identifier)
		title = titleCaser.String(snaked)
	}
	fm[titleKey] = title

	// Set description if provided
	if req.Description != "" {
		fm[descriptionKey] = req.Description
	}

	// Set up inventory structure
	inventoryData := make(map[string]any)
	container := ""
	if req.Container != "" {
		container = wikiidentifiers.MungeIdentifier(req.Container)
		inventoryData[containerKey] = container
	}
	inventoryData[itemsKey] = []string{}
	fm[inventoryKey] = inventoryData

	// Write the frontmatter
	if err := s.pageReaderMutator.WriteFrontMatter(identifier, fm); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	// Build and write the markdown content
	markdown := inventory.BuildItemMarkdown()
	if err := s.pageReaderMutator.WriteMarkdown(identifier, markdown); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write markdown: %v", err)
	}

	containerSuffix := ""
	if container != "" {
		containerSuffix = fmt.Sprintf(" in container '%s'", container)
	}

	return &apiv1.CreateInventoryItemResponse{
		Success:        true,
		ItemIdentifier: identifier,
		Summary:        fmt.Sprintf("Created inventory item '%s'%s.", title, containerSuffix),
	}, nil
}

// MoveInventoryItem implements the MoveInventoryItem RPC.
//
//revive:disable:function-length
func (s *Server) MoveInventoryItem(_ context.Context, req *apiv1.MoveInventoryItemRequest) (*apiv1.MoveInventoryItemResponse, error) {
	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	identifier := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)
	newContainer := mungeOptionalContainer(req.NewContainer)

	// Read the item's current frontmatter
	_, itemFm, err := s.pageReaderMutator.ReadFrontMatter(identifier)
	if err != nil {
		return handleMoveItemReadError(err, identifier)
	}

	previousContainer := getContainerFromFrontmatter(itemFm)

	// If the item is already in the target container, return success
	if previousContainer == newContainer {
		return buildAlreadyInContainerResponse(identifier, previousContainer, newContainer), nil
	}

	// Update the item's inventory.container
	updateItemContainer(itemFm, newContainer)

	// Write the updated item frontmatter
	if err := s.pageReaderMutator.WriteFrontMatter(identifier, itemFm); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update item frontmatter: %v", err)
	}

	// Update container item lists
	s.updateContainerItemLists(previousContainer, newContainer, identifier)

	// Build and return response
	title := getItemTitle(itemFm, identifier)
	summary := buildMoveSummary(title, previousContainer, newContainer)

	return &apiv1.MoveInventoryItemResponse{
		Success:           true,
		PreviousContainer: previousContainer,
		NewContainer:      newContainer,
		Summary:           summary,
	}, nil
}

// mungeOptionalContainer munges a container identifier if non-empty.
func mungeOptionalContainer(container string) string {
	if container == "" {
		return ""
	}
	return wikiidentifiers.MungeIdentifier(container)
}

// handleMoveItemReadError handles errors when reading item frontmatter for move.
func handleMoveItemReadError(err error, identifier string) (*apiv1.MoveInventoryItemResponse, error) {
	if os.IsNotExist(err) {
		return &apiv1.MoveInventoryItemResponse{
			Success: false,
			Error:   fmt.Sprintf("item not found: %s", identifier),
			Summary: fmt.Sprintf("Could not find item '%s'.", identifier),
		}, nil
	}
	return nil, status.Errorf(codes.Internal, "failed to read item frontmatter: %v", err)
}

// getContainerFromFrontmatter extracts the container value from frontmatter.
func getContainerFromFrontmatter(fm map[string]any) string {
	inv, ok := fm[inventoryKey].(map[string]any)
	if !ok {
		return ""
	}
	cont, ok := inv[containerKey].(string)
	if !ok {
		return ""
	}
	return cont
}

// buildAlreadyInContainerResponse builds a response when item is already in target container.
func buildAlreadyInContainerResponse(identifier, previousContainer, newContainer string) *apiv1.MoveInventoryItemResponse {
	summary := fmt.Sprintf("Item '%s' is already ", identifier)
	if newContainer == "" {
		summary += "a root-level item."
	} else {
		summary += fmt.Sprintf("in container '%s'.", newContainer)
	}
	return &apiv1.MoveInventoryItemResponse{
		Success:           true,
		PreviousContainer: previousContainer,
		NewContainer:      newContainer,
		Summary:           summary,
	}
}

// updateItemContainer updates the inventory.container in frontmatter.
func updateItemContainer(fm map[string]any, newContainer string) {
	if fm[inventoryKey] == nil {
		fm[inventoryKey] = make(map[string]any)
	}
	inv, ok := fm[inventoryKey].(map[string]any)
	if !ok {
		inv = make(map[string]any)
		fm[inventoryKey] = inv
	}
	if newContainer == "" {
		delete(inv, containerKey)
	} else {
		inv[containerKey] = newContainer
	}
}

// updateContainerItemLists updates the inventory.items lists on containers after a move.
func (s *Server) updateContainerItemLists(previousContainer, newContainer, identifier string) {
	if previousContainer != "" {
		if err := s.removeItemFromContainerList(previousContainer, identifier); err != nil {
			s.logger.Warn("failed to remove item from previous container's items list: %v", err)
		}
	}
	if newContainer != "" {
		if err := s.addItemToContainerList(newContainer, identifier); err != nil {
			s.logger.Warn("failed to add item to new container's items list: %v", err)
		}
	}
}

// getItemTitle extracts the title from frontmatter or returns the identifier as fallback.
func getItemTitle(fm map[string]any, identifier string) string {
	if t, ok := fm[titleKey].(string); ok && t != "" {
		return t
	}
	return identifier
}

// buildMoveSummary builds a human-readable summary of a move operation.
func buildMoveSummary(title, previousContainer, newContainer string) string {
	if previousContainer == "" && newContainer != "" {
		return fmt.Sprintf("Moved '%s' into container '%s'.", title, newContainer)
	}
	if previousContainer != "" && newContainer == "" {
		return fmt.Sprintf("Removed '%s' from container '%s' (now a root-level item).", title, previousContainer)
	}
	return fmt.Sprintf("Moved '%s' from '%s' to '%s'.", title, previousContainer, newContainer)
}

// ListContainerContents implements the ListContainerContents RPC.
func (s *Server) ListContainerContents(_ context.Context, req *apiv1.ListContainerContentsRequest) (*apiv1.ListContainerContentsResponse, error) {
	if req.ContainerIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "container_identifier is required")
	}

	containerID := wikiidentifiers.MungeIdentifier(req.ContainerIdentifier)
	maxDepth := int(req.MaxDepth)
	if maxDepth == 0 {
		maxDepth = defaultMaxRecursion
	}

	items, totalCount := s.listContainerContentsRecursive(containerID, req.Recursive, maxDepth, 0)

	// Build summary
	var summary string
	if len(items) == 0 {
		summary = fmt.Sprintf("Container '%s' is empty.", containerID)
	} else if req.Recursive {
		summary = fmt.Sprintf("Container '%s' contains %d item(s) (including nested).", containerID, totalCount)
	} else {
		summary = fmt.Sprintf("Container '%s' contains %d direct item(s).", containerID, len(items))
	}

	return &apiv1.ListContainerContentsResponse{
		ContainerIdentifier: containerID,
		Items:               items,
		TotalCount:          int32(totalCount),
		Summary:             summary,
	}, nil
}

// listContainerContentsRecursive recursively lists items in a container.
// It combines items from two sources:
// 1. Items that have inventory.container set to this container
// 2. Items listed in this container's inventory.items array
//
//revive:disable:flag-parameter
func (s *Server) listContainerContentsRecursive(containerID string, recursive bool, maxDepth, currentDepth int) ([]*apiv1.InventoryItem, int) {
	itemIDSet := s.collectContainerItemIDs(containerID)

	var items []*apiv1.InventoryItem
	totalCount := 0

	for itemID := range itemIDSet {
		item := s.buildInventoryItem(itemID, containerID)

		// Recursively get nested items if requested
		if recursive && item.IsContainer && currentDepth < maxDepth {
			nestedItems, nestedCount := s.listContainerContentsRecursive(itemID, true, maxDepth, currentDepth+1)
			item.NestedItems = nestedItems
			totalCount += nestedCount
		}

		items = append(items, item)
		totalCount++
	}

	return items, totalCount
}

// collectContainerItemIDs collects all item IDs associated with a container from both sources.
func (s *Server) collectContainerItemIDs(containerID string) map[string]bool {
	itemIDSet := make(map[string]bool)

	// Source 1: Query for items that have this container as their inventory.container
	itemIDsFromContainer := s.frontmatterIndexQueryer.QueryExactMatch("inventory.container", containerID)
	for _, itemID := range itemIDsFromContainer {
		itemIDSet[itemID] = true
	}

	// Source 2: Get items from the container's inventory.items array
	s.addItemsFromContainerArray(containerID, itemIDSet)

	return itemIDSet
}

// addItemsFromContainerArray adds items from the container's inventory.items array to the set.
func (s *Server) addItemsFromContainerArray(containerID string, itemIDSet map[string]bool) {
	_, containerFm, err := s.pageReaderMutator.ReadFrontMatter(containerID)
	if err != nil {
		return
	}

	inv, ok := containerFm[inventoryKey].(map[string]any)
	if !ok {
		return
	}

	itemsRaw, ok := inv[itemsKey]
	if !ok {
		return
	}

	extractItemIDs(itemsRaw, itemIDSet)
}

// extractItemIDs extracts item IDs from a raw items value and adds them to the set.
func extractItemIDs(itemsRaw any, itemIDSet map[string]bool) {
	switch items := itemsRaw.(type) {
	case []string:
		for _, itemID := range items {
			itemIDSet[itemID] = true
		}
	case []any:
		for _, item := range items {
			if itemID, ok := item.(string); ok {
				itemIDSet[itemID] = true
			}
		}
	}
}

// buildInventoryItem creates an InventoryItem with title and container status.
func (s *Server) buildInventoryItem(itemID, containerID string) *apiv1.InventoryItem {
	item := &apiv1.InventoryItem{
		Identifier: itemID,
		Container:  containerID,
	}

	if title := s.frontmatterIndexQueryer.GetValue(itemID, titleKey); title != "" {
		item.Title = title
	}

	// Primary: Check explicit is_container field
	if s.frontmatterIndexQueryer.GetValue(itemID, "inventory.is_container") == "true" {
		item.IsContainer = true
	} else {
		// Fallback for legacy: items reference this as their container
		item.IsContainer = len(s.frontmatterIndexQueryer.QueryExactMatch("inventory.container", itemID)) > 0
	}

	return item
}

// FindItemLocation implements the FindItemLocation RPC.
func (s *Server) FindItemLocation(_ context.Context, req *apiv1.FindItemLocationRequest) (*apiv1.FindItemLocationResponse, error) {
	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	itemID := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)

	// Read the item's frontmatter
	_, itemFm, err := s.pageReaderMutator.ReadFrontMatter(itemID)
	if err != nil {
		if os.IsNotExist(err) {
			return &apiv1.FindItemLocationResponse{
				ItemIdentifier: itemID,
				Found:          false,
				Summary:        fmt.Sprintf("Item '%s' not found.", itemID),
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to read item frontmatter: %v", err)
	}

	// Get the container from inventory.container
	var locations []*apiv1.ContainerPath
	container := getContainerFromFrontmatter(itemFm)

	if container != "" {
		path := &apiv1.ContainerPath{
			Container: container,
		}

		// Build hierarchy path if requested
		if req.IncludeHierarchy {
			path.Path = s.buildContainerHierarchy(container)
		}

		locations = append(locations, path)
	}

	// Build summary
	var summary string
	title := itemID
	if t, ok := itemFm[titleKey].(string); ok && t != "" {
		title = t
	}

	if container == "" {
		summary = fmt.Sprintf("'%s' is a root-level item (not in any container).", title)
	} else {
		summary = fmt.Sprintf("'%s' is in container '%s'.", title, container)
		if req.IncludeHierarchy && len(locations) > 0 && len(locations[0].Path) > 1 {
			summary += fmt.Sprintf(" Full path: %v", locations[0].Path)
		}
	}

	return &apiv1.FindItemLocationResponse{
		ItemIdentifier: itemID,
		Locations:      locations,
		Found:          true,
		Summary:        summary,
	}, nil
}

// buildContainerHierarchy builds the full path from root to the given container.
func (s *Server) buildContainerHierarchy(containerID string) []string {
	var path []string
	visited := make(map[string]bool)
	current := containerID

	for current != "" && !visited[current] {
		visited[current] = true
		path = append([]string{current}, path...) // Prepend

		// Get the container's parent
		_, fm, err := s.pageReaderMutator.ReadFrontMatter(current)
		if err != nil {
			break
		}

		current = getContainerFromFrontmatter(fm)
	}

	return path
}

// removeItemFromContainerList removes an item from a container's inventory.items list.
func (s *Server) removeItemFromContainerList(containerID, itemID string) error {
	_, containerFm, err := s.pageReaderMutator.ReadFrontMatter(containerID)
	if err != nil {
		if os.IsNotExist(err) {
			// Container doesn't exist, nothing to update
			return nil
		}
		return fmt.Errorf("failed to read container frontmatter: %w", err)
	}

	inv, ok := containerFm[inventoryKey].(map[string]any)
	if !ok {
		// No inventory section, nothing to update
		return nil
	}

	itemsRaw, ok := inv[itemsKey]
	if !ok {
		// No items list, nothing to update
		return nil
	}

	// Handle both []string and []any types
	var newItems []string
	switch items := itemsRaw.(type) {
	case []string:
		for _, item := range items {
			if item != itemID {
				newItems = append(newItems, item)
			}
		}
	case []any:
		for _, item := range items {
			if itemStr, ok := item.(string); ok && itemStr != itemID {
				newItems = append(newItems, itemStr)
			}
		}
	default:
		return nil // Unknown type, skip
	}

	inv[itemsKey] = newItems
	return s.pageReaderMutator.WriteFrontMatter(containerID, containerFm)
}

// addItemToContainerList adds an item to a container's inventory.items list if not already present.
func (s *Server) addItemToContainerList(containerID, itemID string) error {
	_, containerFm, err := s.pageReaderMutator.ReadFrontMatter(containerID)
	if err != nil {
		if os.IsNotExist(err) {
			// Container doesn't exist, can't update
			return nil
		}
		return fmt.Errorf("failed to read container frontmatter: %w", err)
	}

	// Ensure inventory section exists
	inv, ok := containerFm[inventoryKey].(map[string]any)
	if !ok {
		inv = make(map[string]any)
		containerFm[inventoryKey] = inv
	}

	// Get or initialize items list
	var items []string
	switch itemsRaw := inv[itemsKey].(type) {
	case []string:
		items = itemsRaw
	case []any:
		for _, item := range itemsRaw {
			if itemStr, ok := item.(string); ok {
				items = append(items, itemStr)
			}
		}
	default:
		items = []string{}
	}

	// Check if item already exists
	for _, item := range items {
		if item == itemID {
			return nil // Already in the list
		}
	}

	// Add the item
	items = append(items, itemID)
	inv[itemsKey] = items

	return s.pageReaderMutator.WriteFrontMatter(containerID, containerFm)
}
