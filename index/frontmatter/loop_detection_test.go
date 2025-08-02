package frontmatter

import (
	"testing"
	"time"
)

// TestCircularReferenceDetection tests that circular references don't cause infinite loops
func TestCircularReferenceDetection(t *testing.T) {
	pageReader := NewMockPageReader()
	index := NewIndex(pageReader)

	// Create a circular reference in the frontmatter
	circularMap := make(map[string]any)
	innerMap := make(map[string]any)
	circularMap["inner"] = innerMap
	innerMap["outer"] = circularMap // This creates a circular reference

	pageReader.SetPageFrontmatter("circular-page", circularMap)

	// Run the indexing in a goroutine with a timeout
	done := make(chan bool)
	var err error
	
	go func() {
		err = index.AddPageToIndex("circular-page")
		done <- true
	}()

	select {
	case <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		t.Log("Indexing completed without infinite loop")
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out - infinite recursion detected in AddPageToIndex")
	}
}

// TestDeeplyNestedStructure tests that very deep nesting doesn't cause stack overflow
func TestDeeplyNestedStructure(t *testing.T) {
	pageReader := NewMockPageReader()
	index := NewIndex(pageReader)

	// Create a very deep nesting structure (100 levels)
	const deepNestingLevels = 100
	deepMap := make(map[string]any)
	current := deepMap
	
	for i := 0; i < deepNestingLevels; i++ {
		nested := make(map[string]any)
		current["level"] = nested
		current = nested
	}
	current["final"] = "deep_value"

	pageReader.SetPageFrontmatter("deep-page", deepMap)

	// Run the indexing in a goroutine with a timeout
	done := make(chan bool)
	var err error
	
	go func() {
		err = index.AddPageToIndex("deep-page")
		done <- true
	}()

	select {
	case <-done:
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		t.Log("Deep nesting handled successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible stack overflow in deep nesting")
	}
}