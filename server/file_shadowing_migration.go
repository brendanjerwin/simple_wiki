package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/schollz/versionedtext"
)

// FileShadowingMigrationScanJob scans the data directory for PascalCase files
// and enqueues migration jobs for each one
type FileShadowingMigrationScanJob struct {
	dataDir     string
	coordinator *jobs.JobQueueCoordinator
	queueName   string
	site        *Site
}

// NewFileShadowingMigrationScanJob creates a new scan job
func NewFileShadowingMigrationScanJob(dataDir string, coordinator *jobs.JobQueueCoordinator, queueName string, site *Site) *FileShadowingMigrationScanJob {
	return &FileShadowingMigrationScanJob{
		dataDir:     dataDir,
		coordinator: coordinator,
		queueName:   queueName,
		site:        site,
	}
}

// Execute scans for PascalCase files and enqueues migration jobs
func (j *FileShadowingMigrationScanJob) Execute() error {
	// Check if directory exists
	if _, err := os.Stat(j.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s: no such file or directory", j.dataDir)
	}
	
	// Find all PascalCase files
	pascalFiles := j.FindPascalCaseFiles()
	
	// Group by logical page
	groups := j.GroupFilesByLogicalPage(pascalFiles)
	
	// Enqueue a migration job for each logical page
	for logicalPage := range groups {
		migrationJob := NewFileShadowingMigrationJob(j.site, logicalPage)
		j.coordinator.EnqueueJob(j.queueName, migrationJob)
	}
	
	return nil
}

// GetName returns the job name
func (*FileShadowingMigrationScanJob) GetName() string {
	return "FileShadowingMigrationScanJob"
}

// FindPascalCaseFiles returns all PascalCase page files in the data directory
func (j *FileShadowingMigrationScanJob) FindPascalCaseFiles() []string {
	files, err := os.ReadDir(j.dataDir)
	if err != nil {
		return []string{}
	}
	
	var pascalFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		name := file.Name()
		
		// Skip non-page files (uploads, etc.)
		if strings.HasPrefix(name, "sha256") || 
		   (!strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".md")) {
			continue
		}
		
		// Skip base32-encoded files (these are already munged)
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".md")
		if _, err := base32tools.DecodeFromBase32(base); err == nil {
			continue // This is a base32-encoded file, skip it
		}
		
		// Check if this literal filename represents a PascalCase identifier
		mungedVersion := wikiidentifiers.MungeIdentifier(base)
		if base != mungedVersion {
			// This is a PascalCase identifier (munging would change it)
			pascalFiles = append(pascalFiles, name)
		}
	}
	
	return pascalFiles
}


// GroupFilesByLogicalPage groups files by their logical page identifier
func (*FileShadowingMigrationScanJob) GroupFilesByLogicalPage(files []string) map[string][]string {
	groups := make(map[string][]string)
	
	for _, file := range files {
		// Extract the identifier from the literal filename (these are PascalCase files)
		identifier := strings.TrimSuffix(strings.TrimSuffix(file, ".json"), ".md")
		groups[identifier] = append(groups[identifier], file)
	}
	
	return groups
}

// FileShadowingMigrationJob handles migrating a specific PascalCase page to munged_name
type FileShadowingMigrationJob struct {
	site          *Site
	logicalPageID string
}

// NewFileShadowingMigrationJob creates a new migration job
func NewFileShadowingMigrationJob(site *Site, logicalPageID string) *FileShadowingMigrationJob {
	return &FileShadowingMigrationJob{
		site:          site,
		logicalPageID: logicalPageID,
	}
}

// Execute migrates a PascalCase page to munged_name format
func (j *FileShadowingMigrationJob) Execute() error {
	// Find literal PascalCase files for this identifier
	pascalJSONPath := filepath.Join(j.site.PathToData, j.logicalPageID+".json")
	pascalMdPath := filepath.Join(j.site.PathToData, j.logicalPageID+".md")
	
	// Check if any PascalCase files exist
	var pascalFiles []string
	if _, err := os.Stat(pascalJSONPath); err == nil {
		pascalFiles = append(pascalFiles, pascalJSONPath)
	}
	if _, err := os.Stat(pascalMdPath); err == nil {
		pascalFiles = append(pascalFiles, pascalMdPath)
	}
	
	if len(pascalFiles) == 0 {
		return fmt.Errorf("no PascalCase files found for identifier: %s", j.logicalPageID)
	}
	
	// Get munged identifier
	mungedID := wikiidentifiers.MungeIdentifier(j.logicalPageID)
	
	// Check for shadowing conflicts using Site methods
	hasShadowing := j.checkForShadowingUsingSite(mungedID)
	
	var finalPage *Page
	
	if hasShadowing {
		// Compare content richness and choose the richer version
		// Read both versions directly to compare content
		pascalPage := j.readPascalPageUsingSite(j.logicalPageID)
		mungedPage := j.readMungedPageDirectly(mungedID)
		
		// Choose richer content (simple heuristic: longer content)
		pascalLength := len(pascalPage.Text.GetCurrent())
		mungedLength := len(mungedPage.Text.GetCurrent())
		
		if pascalLength > mungedLength {
			finalPage = pascalPage
			finalPage.Identifier = mungedID // Change identifier to munged version
		} else {
			finalPage = mungedPage // Keep existing munged page
		}
	} else {
		// No shadowing - use PascalCase content with munged identifier
		finalPage = j.readPascalPageUsingSite(j.logicalPageID)
		finalPage.Identifier = mungedID // Change identifier to munged version
	}
	
	// Save the page with munged identifier (this will use proper base32 encoding)
	if err := finalPage.Save(); err != nil {
		return fmt.Errorf("failed to save munged page: %v", err)
	}
	
	// Remove literal PascalCase files
	for _, pascalFile := range pascalFiles {
		if err := os.Remove(pascalFile); err != nil {
			return fmt.Errorf("failed to remove PascalCase file %s: %v", pascalFile, err)
		}
	}
	
	return nil
}

// GetName returns the job name
func (j *FileShadowingMigrationJob) GetName() string {
	return fmt.Sprintf("FileShadowingMigrationJob-%s", j.logicalPageID)
}

// CheckForShadowing checks if munged versions already exist for this logical page
func (j *FileShadowingMigrationJob) CheckForShadowing(logicalPageID string) (bool, []string) {
	// Get the munged version of the identifier
	mungedID := wikiidentifiers.MungeIdentifier(logicalPageID)
	
	// Check if munged versions exist on disk
	var mungedFiles []string
	
	// Check for .json file
	jsonPath := filepath.Join(j.site.PathToData, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".json")
	if _, err := os.Stat(jsonPath); err == nil {
		mungedFiles = append(mungedFiles, jsonPath)
	}
	
	// Check for .md file  
	mdPath := filepath.Join(j.site.PathToData, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".md")
	if _, err := os.Stat(mdPath); err == nil {
		mungedFiles = append(mungedFiles, mdPath)
	}
	
	return len(mungedFiles) > 0, mungedFiles
}

// checkForShadowingUsingSite checks if munged page already exists using Site methods
func (j *FileShadowingMigrationJob) checkForShadowingUsingSite(mungedID string) bool {
	mungedPage := j.site.Open(mungedID)
	return !mungedPage.IsNew()
}

// readPascalPageUsingSite reads a PascalCase page by directly opening the literal files
func (j *FileShadowingMigrationJob) readPascalPageUsingSite(pascalID string) *Page {
	// Create a temporary site that reads from literal filenames (not base32)
	// We need to read the literal files and construct a Page object
	page := &Page{
		Identifier: pascalID,
		Site:       j.site,
	}
	
	// Read JSON file if it exists
	jsonPath := filepath.Join(j.site.PathToData, pascalID+".json")
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
					page.Text = versionedtext.NewVersionedText(currentText)
					return page
				}
			}
			
			// If that fails, try to parse as simple {current: "text"} format
			var simpleText struct {
				Current string `json:"current"`
			}
			if json.Unmarshal(pageData.Text, &simpleText) == nil && simpleText.Current != "" {
				page.Text = versionedtext.NewVersionedText(simpleText.Current)
				return page
			}
		}
	}
	
	// Read MD file if JSON didn't work or doesn't exist
	mdPath := filepath.Join(j.site.PathToData, pascalID+".md")
	if mdData, err := os.ReadFile(mdPath); err == nil {
		page.Text = versionedtext.NewVersionedText(string(mdData))
		return page
	}
	
	// Return empty page if neither file could be read
	page.Text = versionedtext.NewVersionedText("")
	return page
}

// readMungedPageDirectly reads a munged page by directly accessing the base32-encoded files
func (j *FileShadowingMigrationJob) readMungedPageDirectly(mungedID string) *Page {
	page := &Page{
		Identifier: mungedID,
		Site:       j.site,
	}
	
	// Calculate the base32-encoded filenames for the munged version
	jsonPath := filepath.Join(j.site.PathToData, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".json")
	mdPath := filepath.Join(j.site.PathToData, base32tools.EncodeToBase32(strings.ToLower(mungedID))+".md")
	
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
					page.Text = versionedtext.NewVersionedText(currentText)
					return page
				}
			}
			
			// If that fails, try to parse as simple {current: "text"} format
			var simpleText struct {
				Current string `json:"current"`
			}
			if json.Unmarshal(pageData.Text, &simpleText) == nil && simpleText.Current != "" {
				page.Text = versionedtext.NewVersionedText(simpleText.Current)
				return page
			}
		}
	}
	
	// Read MD file if JSON didn't work or doesn't exist
	if mdData, err := os.ReadFile(mdPath); err == nil {
		page.Text = versionedtext.NewVersionedText(string(mdData))
		return page
	}
	
	// Return empty page if neither file could be read
	page.Text = versionedtext.NewVersionedText("")
	return page
}

