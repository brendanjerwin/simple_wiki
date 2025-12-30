package eager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

// JSONArchiveMigrationScanJob scans the data directory for JSON files
// and enqueues migration jobs to archive them to __deleted__
type JSONArchiveMigrationScanJob struct {
	dataDir     string
	coordinator *jobs.JobQueueCoordinator
}

// NewJSONArchiveMigrationScanJob creates a new scan job
func NewJSONArchiveMigrationScanJob(dataDir string, coordinator *jobs.JobQueueCoordinator) *JSONArchiveMigrationScanJob {
	return &JSONArchiveMigrationScanJob{
		dataDir:     dataDir,
		coordinator: coordinator,
	}
}

// Execute scans for JSON files and enqueues archive jobs
func (j *JSONArchiveMigrationScanJob) Execute() error {
	// Check if directory exists
	if _, err := os.Stat(j.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s: no such file or directory", j.dataDir)
	}

	// Read all files in the data directory
	entries, err := os.ReadDir(j.dataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	// Enqueue migration job for each JSON file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".json") {
			continue
		}

		// Enqueue migration job for this JSON file
		job := NewJSONArchiveMigrationJob(j.dataDir, filename)
		if err := j.coordinator.EnqueueJob(job); err != nil {
			return fmt.Errorf("failed to enqueue archive job for %s: %w", filename, err)
		}
	}

	return nil
}

// GetName returns the job name
func (*JSONArchiveMigrationScanJob) GetName() string {
	return "JSON Archive Migration Scan"
}

// JSONArchiveMigrationJob archives a single JSON file to __deleted__
type JSONArchiveMigrationJob struct {
	dataDir      string
	jsonFilename string
}

// NewJSONArchiveMigrationJob creates a new archive job for a specific JSON file
func NewJSONArchiveMigrationJob(dataDir string, jsonFilename string) *JSONArchiveMigrationJob {
	return &JSONArchiveMigrationJob{
		dataDir:      dataDir,
		jsonFilename: jsonFilename,
	}
}

// Execute archives the JSON file to __deleted__/<timestamp>/
func (j *JSONArchiveMigrationJob) Execute() error {
	sourcePath := filepath.Join(j.dataDir, j.jsonFilename)

	// Check if file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		// File doesn't exist, nothing to do
		return nil
	}

	// Create timestamp for the archive directory
	timestamp := time.Now().Format("20060102_150405")
	deletedDir := filepath.Join(j.dataDir, "__deleted__", timestamp)

	// Create the timestamped deleted directory
	if err := os.MkdirAll(deletedDir, 0755); err != nil {
		return fmt.Errorf("failed to create deleted directory: %w", err)
	}

	// Find available filename with incrementing number if needed
	destPath := filepath.Join(deletedDir, j.jsonFilename)
	counter := 0
	
	// Keep incrementing until we find an available filename
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			// File doesn't exist, we can use this path
			break
		}
		
		// File exists, try with incremented counter
		counter++
		
		// Extract base name and extension
		ext := filepath.Ext(j.jsonFilename)
		baseName := strings.TrimSuffix(j.jsonFilename, ext)
		
		// Create new filename with counter
		newFilename := fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		destPath = filepath.Join(deletedDir, newFilename)
	}

	// Move the JSON file to the deleted directory
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move JSON file %s: %w", j.jsonFilename, err)
	}

	return nil
}

// GetName returns the job name
func (j *JSONArchiveMigrationJob) GetName() string {
	return fmt.Sprintf("JSON Archive Migration: %s", j.jsonFilename)
}