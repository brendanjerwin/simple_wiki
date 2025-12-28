package eager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
)

// FileShadowingMigrationScanJob scans the data directory for PascalCase files
// and enqueues migration jobs for each one
type FileShadowingMigrationScanJob struct {
	dataDir       string
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
	opener        wikipage.PageOpener
}

// NewFileShadowingMigrationScanJob creates a new scan job
func NewFileShadowingMigrationScanJob(
	dataDir string,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator wikipage.PageReaderMutator,
	opener wikipage.PageOpener,
) *FileShadowingMigrationScanJob {
	return &FileShadowingMigrationScanJob{
		dataDir:       dataDir,
		coordinator:   coordinator,
		readerMutator: readerMutator,
		opener:        opener,
	}
}

// Execute scans for PascalCase identifiers and enqueues migration jobs
func (j *FileShadowingMigrationScanJob) Execute() error {
	// Check if directory exists
	if _, err := os.Stat(j.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s: no such file or directory", j.dataDir)
	}

	// Find all PascalCase identifiers that need migration
	pascalIdentifiers, err := j.FindPascalCaseIdentifiers()
	if err != nil {
		return fmt.Errorf("failed to find PascalCase identifiers: %w", err)
	}

	// Enqueue a migration job for each PascalCase identifier
	for _, identifier := range pascalIdentifiers {
		migrationJob := NewFileShadowingMigrationJob(j.readerMutator, j.opener, j.dataDir, identifier)
		j.coordinator.EnqueueJob(migrationJob)
	}

	return nil
}

// GetName returns the job name
func (*FileShadowingMigrationScanJob) GetName() string {
	return "FileShadowingMigrationScanJob"
}

// FindPascalCaseIdentifiers returns all identifiers that need migration
// by reading MD files and checking their stored identifier field.
// Returns an error if the data directory cannot be read.
func (j *FileShadowingMigrationScanJob) FindPascalCaseIdentifiers() ([]string, error) {
	files, err := os.ReadDir(j.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var pascalIdentifiers []string
	identifiersFound := make(map[string]bool)

	for _, file := range files {
		// Only look at MD files
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		identifier, err := j.extractIdentifierFromMD(file.Name())
		if err != nil {
			// Log but continue - individual file errors shouldn't stop the scan
			continue
		}
		if identifier == "" {
			continue
		}

		// Skip if we've already processed this identifier
		if identifiersFound[identifier] {
			continue
		}
		identifiersFound[identifier] = true

		// Check if this identifier needs munging by comparing with its munged version
		mungedVersion, err := wikiidentifiers.MungeIdentifier(identifier)
		if err != nil || mungedVersion == "" {
			continue // Skip invalid identifiers
		}
		if identifier != mungedVersion {
			// Additional check: ensure that migration wouldn't cause file conflicts
			// by checking if the base32 encodings would be different
			originalBase32 := base32tools.EncodeToBase32(strings.ToLower(identifier))
			mungedBase32 := base32tools.EncodeToBase32(strings.ToLower(mungedVersion))

			if originalBase32 != mungedBase32 {
				// This identifier needs migration
				pascalIdentifiers = append(pascalIdentifiers, identifier)
			}
			// If base32 encodings are the same, skip this identifier to avoid file conflicts
		}
	}

	return pascalIdentifiers, nil
}

// tomlFrontmatterParts is the expected number of parts when splitting TOML frontmatter by "+++".
// Format: [before]+++[frontmatter]+++[body] = 3 parts
const tomlFrontmatterParts = 3

// extractIdentifierFromMD reads an MD file and extracts the identifier from TOML frontmatter.
// Returns the identifier and any error encountered.
// An empty identifier with nil error means the file has no identifier (derived from filename).
func (j *FileShadowingMigrationScanJob) extractIdentifierFromMD(filename string) (string, error) {
	filePath := filepath.Join(j.dataDir, filename)
	mdData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	content := string(mdData)

	// Check for TOML frontmatter
	if !strings.HasPrefix(content, "+++") {
		// No frontmatter - derive identifier from filename
		encodedName := strings.TrimSuffix(filename, ".md")
		logicalID, err := base32tools.DecodeFromBase32(encodedName)
		if err != nil {
			return "", fmt.Errorf("failed to decode base32 filename %s: %w", encodedName, err)
		}
		return logicalID, nil
	}

	// Parse TOML frontmatter
	parts := strings.SplitN(content, "+++", tomlFrontmatterParts)
	if len(parts) < tomlFrontmatterParts {
		return "", fmt.Errorf("malformed TOML frontmatter in %s: expected %d parts", filename, tomlFrontmatterParts)
	}

	frontmatter := strings.TrimSpace(parts[1])

	// Use proper TOML parsing for robustness
	var data struct {
		Identifier string `toml:"identifier"`
	}
	if err := toml.Unmarshal([]byte(frontmatter), &data); err != nil {
		return "", fmt.Errorf("failed to parse TOML frontmatter in %s: %w", filename, err)
	}
	if data.Identifier != "" {
		return data.Identifier, nil
	}

	// No identifier in frontmatter - derive from filename
	encodedName := strings.TrimSuffix(filename, ".md")
	logicalID, err := base32tools.DecodeFromBase32(encodedName)
	if err != nil {
		return "", fmt.Errorf("failed to decode base32 filename %s: %w", encodedName, err)
	}
	return logicalID, nil
}

// FileShadowingMigrationJob handles migrating a specific PascalCase page to munged_name
type FileShadowingMigrationJob struct {
	readerMutator wikipage.PageReaderMutator
	opener        wikipage.PageOpener
	dataDir       string
	logicalPageID string
}

// NewFileShadowingMigrationJob creates a new migration job
func NewFileShadowingMigrationJob(
	readerMutator wikipage.PageReaderMutator,
	opener wikipage.PageOpener,
	dataDir string,
	logicalPageID string,
) *FileShadowingMigrationJob {
	return &FileShadowingMigrationJob{
		readerMutator: readerMutator,
		opener:        opener,
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
	mungedID, err := wikiidentifiers.MungeIdentifier(j.logicalPageID)
	if err != nil {
		return fmt.Errorf("invalid page identifier %q: %w", j.logicalPageID, err)
	}

	// Check for shadowing conflicts using interface methods
	// We can use ReadPage() for the munged version since we want to read it normally
	mungedPage, err := j.opener.ReadPage(mungedID)
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
	if err := j.readerMutator.DeletePage(wikipage.PageIdentifier(j.logicalPageID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to soft delete original PascalCase page %s: %w", j.logicalPageID, err)
	}

	// Now save the page using WriteFrontMatter and WriteMarkdown
	fm, err := finalPage.GetFrontMatter()
	if err != nil {
		return fmt.Errorf("failed to get frontmatter: %w", err)
	}
	md, err := finalPage.GetMarkdown()
	if err != nil {
		return fmt.Errorf("failed to get markdown: %w", err)
	}

	// First write the frontmatter
	if err := j.readerMutator.WriteFrontMatter(wikipage.PageIdentifier(finalPage.Identifier), fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}
	// Then write the markdown
	if err := j.readerMutator.WriteMarkdown(wikipage.PageIdentifier(finalPage.Identifier), md); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	return nil
}

// readPascalPageDirectly reads a page by directly accessing the base32-encoded MD file
// for the identifier (not the munged version)
func (j *FileShadowingMigrationJob) readPascalPageDirectly(pascalID string) *wikipage.Page {
	page := &wikipage.Page{
		Identifier: pascalID,
	}

	// Calculate the base32-encoded filename for the identifier
	// Note: we use the lowercase identifier, not the munged version
	mdPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(pascalID))+".md")

	// Read MD file
	if mdData, err := os.ReadFile(mdPath); err == nil {
		page.Text = string(mdData)
		return page
	}

	// Return empty page if file could not be read
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
	mungedID, err := wikiidentifiers.MungeIdentifier(logicalPageID)
	if err != nil {
		// Invalid identifier cannot have shadowing conflicts
		return false, nil
	}

	// Check if base32-encoded MD file exists on disk (for the munged identifier)
	var mungedFiles []string

	mdPath := filepath.Join(j.dataDir, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".md")
	if _, err := os.Stat(mdPath); err == nil {
		mungedFiles = append(mungedFiles, mdPath)
	}

	return len(mungedFiles) > 0, mungedFiles
}
