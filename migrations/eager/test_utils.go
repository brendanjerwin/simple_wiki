package eager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
)

const testFileTimestamp = 1609459200 // 2021-01-01 Unix timestamp

// Helper function to create PascalCase pages directly on filesystem
func createPascalCasePage(dir, identifier, content string) {
	// Create JSON file with versioned text structure
	jsonPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".json")
	
	pageData := map[string]any{
		"Identifier": identifier,
		"Text": map[string]any{
			"CurrentText": content,
		},
	}
	
	jsonData, _ := json.Marshal(pageData)
	_ = os.WriteFile(jsonPath, jsonData, 0644)
	
	// Also create MD file for completeness
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	_ = os.WriteFile(mdPath, []byte(content), 0644)
}

// Helper function to create test files
func createTestFile(dir, filename, content string) {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
	// Set a consistent timestamp for testing
	timestamp := time.Unix(testFileTimestamp, 0)
	if err := os.Chtimes(filePath, timestamp, timestamp); err != nil {
		panic(err)
	}
}