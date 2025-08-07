package eager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/schollz/versionedtext"
)

// MigrationDependencies combines interfaces needed for migrations
type MigrationDependencies interface {
	wikipage.PageReaderMutator
	wikipage.PageOpener
}

// FileShadowingMigrationScanJob scans the data directory for PascalCase files
// and enqueues migration jobs for each one
type FileShadowingMigrationScanJob struct {
	dataDir     string
	coordinator *jobs.JobQueueCoordinator
	deps        MigrationDependencies
}

// NewFileShadowingMigrationScanJob creates a new scan job
func NewFileShadowingMigrationScanJob(dataDir string, coordinator *jobs.JobQueueCoordinator, deps MigrationDependencies) *FileShadowingMigrationScanJob {
	return &FileShadowingMigrationScanJob{
		dataDir:     dataDir,
		coordinator: coordinator,
		deps:        deps,
	}
}

// Execute scans for PascalCase identifiers and enqueues migration jobs
func (j *FileShadowingMigrationScanJob) Execute() error {
	// Check if directory exists
	if _, err := os.Stat(j.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s: no such file or directory", j.dataDir)
	}

	// Find all PascalCase identifiers that need migration
	pascalIdentifiers := j.FindPascalCaseIdentifiers()

	// Enqueue a migration job for each PascalCase identifier
	for _, identifier := range pascalIdentifiers {
		migrationJob := NewFileShadowingMigrationJob(j.deps, j.dataDir, identifier)
		j.coordinator.EnqueueJob(migrationJob)
	}

	return nil
}

// GetName returns the job name
func (*FileShadowingMigrationScanJob) GetName() string {
	return "FileShadowingMigrationScanJob"
}

// FindPascalCaseIdentifiers returns all PascalCase identifiers that need migration
// by reading JSON files directly and checking their stored identifier field
func (j *FileShadowingMigrationScanJob) FindPascalCaseIdentifiers() []string {
	files, err := os.ReadDir(j.dataDir)
	if err != nil {
		return []string{}
	}

	var pascalIdentifiers []string
	identifiersFound := make(map[string]bool)

	for _, file := range files {
		// Only look at JSON files (pages)
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Read the JSON file to get the stored identifier
		filePath := filepath.Join(j.dataDir, file.Name())
		jsonData, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Parse the JSON to extract the identifier field
		var pageData struct {
			Identifier string `json:"identifier"`
		}
		if err := json.Unmarshal(jsonData, &pageData); err != nil {
			continue
		}

		identifier := pageData.Identifier
		if identifier == "" {
			continue
		}

		// Skip if we've already processed this identifier
		if identifiersFound[identifier] {
			continue
		}
		identifiersFound[identifier] = true

		// Check if this identifier is PascalCase by comparing with its munged version
		mungedVersion := wikiidentifiers.MungeIdentifier(identifier)
		if identifier != mungedVersion {
			// Additional check: ensure that migration wouldn't cause file conflicts
			// by checking if the base32 encodings would be different
			originalBase32 := base32tools.EncodeToBase32(strings.ToLower(identifier))
			mungedBase32 := base32tools.EncodeToBase32(strings.ToLower(mungedVersion))
			
			if originalBase32 != mungedBase32 {
				// This is a safe PascalCase identifier that needs migration
				pascalIdentifiers = append(pascalIdentifiers, identifier)
			}
			// If base32 encodings are the same, skip this identifier to avoid file conflicts
		}
	}

	return pascalIdentifiers
}

// FileShadowingMigrationJob handles migrating a specific PascalCase page to munged_name
type FileShadowingMigrationJob struct {
	deps          MigrationDependencies
	dataDir       string
	logicalPageID string
}

// NewFileShadowingMigrationJob creates a new migration job
func NewFileShadowingMigrationJob(deps MigrationDependencies, dataDir string, logicalPageID string) *FileShadowingMigrationJob {
	return &FileShadowingMigrationJob{
		deps:          deps,
		dataDir:       dataDir,
		logicalPageID: logicalPageID,
	}
}

// Execute migrates a PascalCase page to munged_name format
func (j *FileShadowingMigrationJob) Execute() error {
	// We can't use ReadPage() to read the PascalCase page because it will prefer
	// munged versions when both exist (this is the shadowing problem we're solving).
	// Instead, we need to read the PascalCase files directly.
	pascalPage := j.readPascalPageDirectly(j.logicalPageID)
	if len(pascalPage.Text) == 0 {
		return fmt.Errorf("no page found for PascalCase identifier: %s", j.logicalPageID)
	}

	// Get munged identifier
	mungedID := wikiidentifiers.MungeIdentifier(j.logicalPageID)

	// Check for shadowing conflicts using interface methods
	// We can use ReadPage() for the munged version since we want to read it normally
	mungedPage, err := j.deps.ReadPage(mungedID)
	if err != nil {
		return fmt.Errorf("failed to open munged page %s: %w", mungedID, err)
	}
	hasShadowing := !mungedPage.IsNew()

	var finalPage *wikipage.Page

	if hasShadowing {
		// Compare content richness and choose the richer version
		// Choose richer content (simple heuristic: longer content)
		pascalLength := len(pascalPage.Text)
		mungedLength := len(mungedPage.Text)

		if pascalLength > mungedLength {
			finalPage = pascalPage
			finalPage.Identifier = mungedID // Change identifier to munged version
		} else {
			finalPage = mungedPage // Keep existing munged page
		}
	} else {
		// No shadowing - use PascalCase content with munged identifier
		finalPage = pascalPage
		finalPage.Identifier = mungedID // Change identifier to munged version
	}

	// IMPORTANT: Delete the original files FIRST, then save the new content
	// This prevents data loss in cases where the original and munged identifiers
	// would result in the same base32-encoded filename

	// Use DeletePage for soft delete (moves to __deleted__ directory)
	if err := j.deps.DeletePage(wikipage.PageIdentifier(j.logicalPageID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to soft delete original PascalCase page %s: %v", j.logicalPageID, err)
	}

	// Now save the page using WriteFrontMatter and WriteMarkdown
	fm, err := finalPage.GetFrontMatter()
	if err != nil {
		return fmt.Errorf("failed to get frontmatter: %v", err)
	}
	md, err := finalPage.GetMarkdown()
	if err != nil {
		return fmt.Errorf("failed to get markdown: %v", err)
	}

	// First write the frontmatter
	if err := j.deps.WriteFrontMatter(wikipage.PageIdentifier(finalPage.Identifier), fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %v", err)
	}
	// Then write the markdown
	if err := j.deps.WriteMarkdown(wikipage.PageIdentifier(finalPage.Identifier), md); err != nil {
		return fmt.Errorf("failed to write markdown: %v", err)
	}

	return nil
}

// readPascalPageDirectly reads a PascalCase page by directly accessing the base32-encoded files
// for the PascalCase identifier (not the munged version)
func (j *FileShadowingMigrationJob) readPascalPageDirectly(pascalID string) *wikipage.Page {
	page := &wikipage.Page{
		Identifier: pascalID,
	}

	// Calculate the base32-encoded filenames for the PascalCase identifier
	// Note: we use the lowercase PascalCase identifier, not the munged version
	jsonPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(pascalID))+".json")
	mdPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(pascalID))+".md")

	// Read JSON file if it exists
	if jsonData, err := os.ReadFile(jsonPath); err == nil {
		// Parse the JSON to get the versioned text
		var pageData struct {
			Text json.RawMessage `json:"text"`
		}
		if json.Unmarshal(jsonData, &pageData) == nil {
			// First try to parse as full versioned text
			var vText versionedtext.VersionedText
			if err := json.Unmarshal(pageData.Text, &vText); err == nil {
				// Use the current text from the parsed versionedtext
				currentText := vText.GetCurrent()
				if currentText != "" {
					page.Text = currentText
					return page
				}
			}

			// If that fails, try to parse as simple {current: "text"} format
			var simpleText struct {
				Current string `json:"current"`
			}
			if json.Unmarshal(pageData.Text, &simpleText) == nil && simpleText.Current != "" {
				page.Text = simpleText.Current
				return page
			}
		}
	}

	// Read MD file if JSON didn't work or doesn't exist
	if mdData, err := os.ReadFile(mdPath); err == nil {
		page.Text = string(mdData)
		return page
	}

	// Return empty page if neither file could be read
	page.Text = ""
	return page
}

// GetName returns the job name
func (j *FileShadowingMigrationJob) GetName() string {
	return fmt.Sprintf("FileShadowingMigrationJob-%s", j.logicalPageID)
}

// CheckForShadowing checks if munged versions already exist for this logical page
func (j *FileShadowingMigrationJob) CheckForShadowing(logicalPageID string) (bool, []string) {
	// Get the munged version of the identifier
	mungedID := wikiidentifiers.MungeIdentifier(logicalPageID)

	// Check if base32-encoded versions exist on disk (for the munged identifier)
	var mungedFiles []string

	// Check for .json file
	jsonPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".json")
	if _, err := os.Stat(jsonPath); err == nil {
		mungedFiles = append(mungedFiles, jsonPath)
	}

	// Check for .md file
	mdPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".md")
	if _, err := os.Stat(mdPath); err == nil {
		mungedFiles = append(mungedFiles, mdPath)
	}

	return len(mungedFiles) > 0, mungedFiles
}