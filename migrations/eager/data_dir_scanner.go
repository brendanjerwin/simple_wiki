package eager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataDirScanner provides read access to the data directory for migration purposes.
// This abstraction allows testing without actual filesystem access.
type DataDirScanner interface {
	// DataDirExists checks if the data directory exists
	DataDirExists() bool

	// ListMDFiles returns a list of .md filenames in the data directory
	ListMDFiles() ([]string, error)

	// ReadMDFile reads the contents of an MD file by filename
	ReadMDFile(filename string) ([]byte, error)

	// MDFileExistsByBase32Name checks if a specific MD file exists by its base32-encoded name
	MDFileExistsByBase32Name(base32Name string) bool
}

// FileSystemDataDirScanner implements DataDirScanner using actual filesystem operations
type FileSystemDataDirScanner struct {
	dataDir string
}

// NewFileSystemDataDirScanner creates a new FileSystemDataDirScanner for the given data directory
func NewFileSystemDataDirScanner(dataDir string) *FileSystemDataDirScanner {
	return &FileSystemDataDirScanner{dataDir: dataDir}
}

// DataDirExists checks if the data directory exists
func (s *FileSystemDataDirScanner) DataDirExists() bool {
	_, err := os.Stat(s.dataDir)
	return !os.IsNotExist(err)
}

// ListMDFiles returns a list of .md filenames in the data directory
func (s *FileSystemDataDirScanner) ListMDFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var mdFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, entry.Name())
		}
	}
	return mdFiles, nil
}

// ReadMDFile reads the contents of an MD file by filename
func (s *FileSystemDataDirScanner) ReadMDFile(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dataDir, filename))
}

// MDFileExistsByBase32Name checks if a specific MD file exists by its base32-encoded name
func (s *FileSystemDataDirScanner) MDFileExistsByBase32Name(base32Name string) bool {
	_, err := os.Stat(filepath.Join(s.dataDir, base32Name+".md"))
	return err == nil
}

// DataDir returns the data directory path (useful for constructing paths to pass to other components)
func (s *FileSystemDataDirScanner) DataDir() string {
	return s.dataDir
}
