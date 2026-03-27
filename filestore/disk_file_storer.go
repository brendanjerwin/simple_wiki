package filestore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
)

const uploadFileExt = ".upload"

// DiskFileStorer stores uploaded files on disk using SHA256 hashes as filenames.
type DiskFileStorer struct {
	dataDir string
}

// NewDiskFileStorer creates a new DiskFileStorer that stores files in dataDir.
func NewDiskFileStorer(dataDir string) (*DiskFileStorer, error) {
	if dataDir == "" {
		return nil, errors.New("dataDir cannot be empty")
	}
	return &DiskFileStorer{dataDir: dataDir}, nil
}

// Store computes the SHA256 hash of the content, saves it to disk, and returns the FileInfo.
func (s *DiskFileStorer) Store(content io.Reader) (FileInfo, error) {
	buf, err := io.ReadAll(content)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to read content: %w", err)
	}

	h := sha256.New()
	if _, err := h.Write(buf); err != nil {
		return FileInfo{}, fmt.Errorf("failed to hash content: %w", err)
	}

	hash := "sha256-" + base32tools.EncodeBytesToBase32(h.Sum(nil))
	filePath := filepath.Join(s.dataDir, hash+uploadFileExt)

	if err := os.WriteFile(filePath, buf, 0600); err != nil {
		return FileInfo{}, fmt.Errorf("failed to write file: %w", err)
	}

	return FileInfo{Hash: hash, SizeBytes: int64(len(buf))}, nil
}

// GetInfo returns metadata about a file identified by its hash.
func (s *DiskFileStorer) GetInfo(hash string) (FileInfo, error) {
	if err := validateHashPath(hash); err != nil {
		return FileInfo{}, err
	}

	filePath := filepath.Join(s.dataDir, hash+uploadFileExt)
	fi, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfo{}, os.ErrNotExist
		}
		return FileInfo{}, fmt.Errorf("failed to stat file: %w", err)
	}
	return FileInfo{Hash: hash, SizeBytes: fi.Size()}, nil
}

// Delete removes the file identified by its hash.
func (s *DiskFileStorer) Delete(hash string) error {
	if err := validateHashPath(hash); err != nil {
		return err
	}

	filePath := filepath.Join(s.dataDir, hash+uploadFileExt)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// validateHashPath validates that the hash does not contain path traversal characters or null bytes.
func validateHashPath(hash string) error {
	if strings.Contains(hash, "/") || strings.Contains(hash, "..") ||
		strings.Contains(hash, string(filepath.Separator)) || strings.ContainsRune(hash, 0) {
		return fmt.Errorf("%w: contains invalid characters", ErrInvalidHash)
	}
	return nil
}
