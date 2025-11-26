package v1

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	inventoryKey           = "inventory"
	containerKey           = "container"
	itemsKey               = "items"
	titleKey               = "title"
	tomlDelimiterConst     = "+++\n"
	newlineConst           = "\n"
	defaultMaxRecursion    = 10
)

// inventoryItemMarkdownTemplate is the markdown template for inventory item pages.
const inventoryItemMarkdownTemplate = `
### Goes in: {{LinkTo .Inventory.Container }}

{{if IsContainer .Identifier }}
## Contents
{{ ShowInventoryContentsOf .Identifier }}
{{ end }}
`

// CreateInventoryItem implements the CreateInventoryItem RPC.
func (s *Server) CreateInventoryItem(_ context.Context, req *apiv1.CreateInventoryItemRequest) (*apiv1.CreateInventoryItemResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	// Munge the identifier to ensure consistency
	identifier := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)

	// Check if page already exists
	_, existingFm, err := s.PageReaderMutator.ReadFrontMatter(identifier)
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

	// Set up inventory structure
	inventory := make(map[string]any)
	container := ""
	if req.Container != "" {
		container = wikiidentifiers.MungeIdentifier(req.Container)
		inventory[containerKey] = container
	}
	inventory[itemsKey] = []string{}
	fm[inventoryKey] = inventory

	// Write the frontmatter
	if err := s.PageReaderMutator.WriteFrontMatter(identifier, fm); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	// Build and write the markdown content
	markdown := buildInventoryItemMarkdown()
	if err := s.PageReaderMutator.WriteMarkdown(identifier, markdown); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write markdown: %v", err)
	}

	summary := fmt.Sprintf("Created inventory item '%s'", title)
	if container != "" {
		summary += fmt.Sprintf(" in container '%s'", container)
	}
	summary += "."

	return &apiv1.CreateInventoryItemResponse{
		Success:        true,
		ItemIdentifier: identifier,
		Summary:        summary,
	}, nil
}

// MoveInventoryItem implements the MoveInventoryItem RPC.
//
//revive:disable:function-length
func (s *Server) MoveInventoryItem(_ context.Context, req *apiv1.MoveInventoryItemRequest) (*apiv1.MoveInventoryItemResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	identifier := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)
	newContainer := ""
	if req.NewContainer != "" {
		newContainer = wikiidentifiers.MungeIdentifier(req.NewContainer)
	}

	// Read the item's current frontmatter
	_, itemFm, err := s.PageReaderMutator.ReadFrontMatter(identifier)
	if err != nil {
		if os.IsNotExist(err) {
			return &apiv1.MoveInventoryItemResponse{
				Success: false,
				Error:   fmt.Sprintf("item not found: %s", identifier),
				Summary: fmt.Sprintf("Could not find item '%s'.", identifier),
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to read item frontmatter: %v", err)
	}

	// Get the previous container
	previousContainer := ""
	if inv, ok := itemFm[inventoryKey].(map[string]any); ok {
		if cont, ok := inv[containerKey].(string); ok {
			previousContainer = cont
		}
	}

	// If the item is already in the target container, return success
	if previousContainer == newContainer {
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
		}, nil
	}

	// Update the item's inventory.container
	if itemFm[inventoryKey] == nil {
		itemFm[inventoryKey] = make(map[string]any)
	}
	inv, ok := itemFm[inventoryKey].(map[string]any)
	if !ok {
		inv = make(map[string]any)
		itemFm[inventoryKey] = inv
	}
	if newContainer == "" {
		delete(inv, containerKey)
	} else {
		inv[containerKey] = newContainer
	}

	// Write the updated item frontmatter
	if err := s.PageReaderMutator.WriteFrontMatter(identifier, itemFm); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update item frontmatter: %v", err)
	}

	// Update the previous container's inventory.items list (remove the item)
	if previousContainer != "" {
		if err := s.removeItemFromContainerList(previousContainer, identifier); err != nil {
			// Log but don't fail - the item's container reference is the authoritative source
			s.Logger.Warn("failed to remove item from previous container's items list: %v", err)
		}
	}

	// Update the new container's inventory.items list (add the item)
	if newContainer != "" {
		if err := s.addItemToContainerList(newContainer, identifier); err != nil {
			// Log but don't fail - the item's container reference is the authoritative source
			s.Logger.Warn("failed to add item to new container's items list: %v", err)
		}
	}

	// Build summary
	title := identifier
	if t, ok := itemFm[titleKey].(string); ok && t != "" {
		title = t
	}

	var summary string
	if previousContainer == "" && newContainer != "" {
		summary = fmt.Sprintf("Moved '%s' into container '%s'.", title, newContainer)
	} else if previousContainer != "" && newContainer == "" {
		summary = fmt.Sprintf("Removed '%s' from container '%s' (now a root-level item).", title, previousContainer)
	} else {
		summary = fmt.Sprintf("Moved '%s' from '%s' to '%s'.", title, previousContainer, newContainer)
	}

	return &apiv1.MoveInventoryItemResponse{
		Success:           true,
		PreviousContainer: previousContainer,
		NewContainer:      newContainer,
		Summary:           summary,
	}, nil
}

// ListContainerContents implements the ListContainerContents RPC.
func (s *Server) ListContainerContents(_ context.Context, req *apiv1.ListContainerContentsRequest) (*apiv1.ListContainerContentsResponse, error) {
	v := reflect.ValueOf(s.FrontmatterIndexQueryer)
	if s.FrontmatterIndexQueryer == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, "FrontmatterIndexQueryer not available")
	}

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
	// Use a set to avoid duplicate items
	itemIDSet := make(map[string]bool)

	// Source 1: Query for items that have this container as their inventory.container
	itemIDsFromContainer := s.FrontmatterIndexQueryer.QueryExactMatch("inventory.container", containerID)
	for _, itemID := range itemIDsFromContainer {
		itemIDSet[itemID] = true
	}

	// Source 2: Get items from the container's inventory.items array
	if s.PageReaderMutator != nil {
		_, containerFm, err := s.PageReaderMutator.ReadFrontMatter(containerID)
		if err == nil {
			if inv, ok := containerFm[inventoryKey].(map[string]any); ok {
				if itemsRaw, ok := inv[itemsKey]; ok {
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
			}
		}
	}

	var items []*apiv1.InventoryItem
	totalCount := 0

	for itemID := range itemIDSet {
		item := &apiv1.InventoryItem{
			Identifier: itemID,
			Container:  containerID,
		}

		// Get the title
		if title := s.FrontmatterIndexQueryer.GetValue(itemID, titleKey); title != "" {
			item.Title = title
		}

		// Check if this item is itself a container
		isContainer := len(s.FrontmatterIndexQueryer.QueryExactMatch("inventory.container", itemID)) > 0
		item.IsContainer = isContainer

		// Recursively get nested items if requested
		if recursive && isContainer && currentDepth < maxDepth {
			nestedItems, nestedCount := s.listContainerContentsRecursive(itemID, true, maxDepth, currentDepth+1)
			item.NestedItems = nestedItems
			totalCount += nestedCount
		}

		items = append(items, item)
		totalCount++
	}

	return items, totalCount
}

// FindItemLocation implements the FindItemLocation RPC.
func (s *Server) FindItemLocation(_ context.Context, req *apiv1.FindItemLocationRequest) (*apiv1.FindItemLocationResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	if req.ItemIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "item_identifier is required")
	}

	itemID := wikiidentifiers.MungeIdentifier(req.ItemIdentifier)

	// Read the item's frontmatter
	_, itemFm, err := s.PageReaderMutator.ReadFrontMatter(itemID)
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
	container := ""
	if inv, ok := itemFm[inventoryKey].(map[string]any); ok {
		if cont, ok := inv[containerKey].(string); ok {
			container = cont
		}
	}

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
		_, fm, err := s.PageReaderMutator.ReadFrontMatter(current)
		if err != nil {
			break
		}

		parent := ""
		if inv, ok := fm[inventoryKey].(map[string]any); ok {
			if cont, ok := inv[containerKey].(string); ok {
				parent = cont
			}
		}
		current = parent
	}

	return path
}

// buildInventoryItemMarkdown creates the markdown content for an inventory item page.
func buildInventoryItemMarkdown() string {
	var builder bytes.Buffer
	_, _ = builder.WriteString("# {{or .Title .Identifier}}")
	_, _ = builder.WriteString(newlineConst)
	_, _ = builder.WriteString(inventoryItemMarkdownTemplate)
	return builder.String()
}

// removeItemFromContainerList removes an item from a container's inventory.items list.
func (s *Server) removeItemFromContainerList(containerID, itemID string) error {
	_, containerFm, err := s.PageReaderMutator.ReadFrontMatter(containerID)
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
	return s.PageReaderMutator.WriteFrontMatter(containerID, containerFm)
}

// addItemToContainerList adds an item to a container's inventory.items list if not already present.
func (s *Server) addItemToContainerList(containerID, itemID string) error {
	_, containerFm, err := s.PageReaderMutator.ReadFrontMatter(containerID)
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

	return s.PageReaderMutator.WriteFrontMatter(containerID, containerFm)
}
