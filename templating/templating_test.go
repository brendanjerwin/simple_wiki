package templating_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	containerA     = "container_a"
	containerB     = "container_b"
	titleKey       = "title"
	identifierKey  = "identifier"
	inventoryKey   = "inventory"
	itemsKey       = "items"
	maxTestLevels  = 12
	levelTemplate  = "level_%d"
)

// Mock implementations for testing
type mockPageReader struct {
	pages map[string]wikipage.FrontMatter
}

func (m *mockPageReader) ReadFrontMatter(identifier string) (string, wikipage.FrontMatter, error) {
	if fm, exists := m.pages[identifier]; exists {
		return identifier, fm, nil
	}
	return "", nil, errors.New("page not found")
}

func (*mockPageReader) ReadMarkdown(identifier string) (string, wikipage.Markdown, error) {
	return identifier, "", nil // Not needed for this test
}

type mockFrontmatterIndex struct {
	index  map[string]map[string][]string
	values map[string]map[string]string
}

func (m *mockFrontmatterIndex) QueryExactMatch(key, value string) []string {
	if keyMap, exists := m.index[key]; exists {
		if results, exists := keyMap[value]; exists {
			return results
		}
	}
	return []string{}
}

func (*mockFrontmatterIndex) QueryPrefixMatch(key, prefix string) []string {
	return []string{}
}

func (*mockFrontmatterIndex) QueryKeyExistence(key string) []string {
	return []string{}
}

func (m *mockFrontmatterIndex) GetValue(identifier, key string) string {
	if idMap, exists := m.values[identifier]; exists {
		if value, exists := idMap[key]; exists {
			return value
		}
	}
	return ""
}

func TestBuildShowInventoryContentsOf_CircularReferences(t *testing.T) {
	// Arrange - Create circular container references
	// Container A contains Container B, Container B contains Container A
	mockSite := &mockPageReader{
		pages: map[string]wikipage.FrontMatter{
			containerA: {
				identifierKey: containerA,
				titleKey:      "Container A",
				inventoryKey: map[string]any{
					itemsKey: []string{containerB},
				},
			},
			containerB: {
				identifierKey: containerB,
				titleKey:      "Container B",
				inventoryKey: map[string]any{
					itemsKey: []string{containerA},
				},
			},
		},
	}

	mockIndex := &mockFrontmatterIndex{
		index: map[string]map[string][]string{},
		values: map[string]map[string]string{
			containerA: {
				"inventory.items": containerB,
				titleKey:          "Container A",
			},
			containerB: {
				"inventory.items": containerA,
				titleKey:          "Container B",
			},
		},
	}

	// Act - Execute the template function that should prevent infinite recursion
	// We use a timeout to ensure the test doesn't hang forever
	done := make(chan string, 1)
	go func() {
		showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
		result := showInventoryFunc(containerA)
		done <- result
	}()

	select {
	case result := <-done:
		// Assert - Function should return without hanging
		if result == "" {
			t.Error("Expected non-empty result, but got empty string")
		}
		if strings.Contains(result, "stack overflow") {
			t.Error("Result contains stack overflow, indicating recursion depth was not limited")
		}
		t.Logf("Result: %s", result)
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - function likely has infinite recursion")
	}
}

func createDeepNestedPages() map[string]wikipage.FrontMatter {
	pages := make(map[string]wikipage.FrontMatter)
	for i := 0; i <= maxTestLevels; i++ {
		levelKey := fmt.Sprintf(levelTemplate, i)
		nextItems := []string{}
		if i < maxTestLevels {
			nextItems = []string{fmt.Sprintf(levelTemplate, i+1)}
		}
		
		pages[levelKey] = wikipage.FrontMatter{
			identifierKey: levelKey,
			titleKey:      fmt.Sprintf("Level %d", i),
			inventoryKey: map[string]any{
				itemsKey: nextItems,
			},
		}
	}
	return pages
}

func createDeepNestedIndex() *mockFrontmatterIndex {
	mockIndex := &mockFrontmatterIndex{
		index:  map[string]map[string][]string{},
		values: make(map[string]map[string]string),
	}
	
	// Set up values for all levels
	for i := 0; i <= maxTestLevels; i++ {
		levelKey := fmt.Sprintf(levelTemplate, i)
		mockIndex.values[levelKey] = map[string]string{
			titleKey: fmt.Sprintf("Level %d", i),
		}
		if i < maxTestLevels {
			nextLevel := fmt.Sprintf(levelTemplate, i+1)
			mockIndex.values[levelKey]["inventory.items"] = nextLevel
		}
	}
	return mockIndex
}

func TestBuildShowInventoryContentsOf_DepthLimit(t *testing.T) {
	// Arrange - Create a deep nested container structure (deeper than max depth of 10)
	mockSite := &mockPageReader{pages: createDeepNestedPages()}
	mockIndex := createDeepNestedIndex()

	// Act
	showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
	result := showInventoryFunc("level_0")

	// Assert - Should contain max depth message
	if !strings.Contains(result, "[Maximum depth reached]") {
		t.Errorf("Expected result to contain '[Maximum depth reached]', but got: %s", result)
	}
	t.Logf("Result with depth limiting: %s", result)
}