// Package filestore provides file storage abstraction for wiki uploads.
package filestore

import (
	"errors"
	"io"
)

// ErrInvalidHash is returned when a hash value fails validation (e.g. contains path traversal).
var ErrInvalidHash = errors.New("invalid hash")

// FileInfo holds metadata about an uploaded file.
type FileInfo struct {
	Hash      string
	SizeBytes int64
}

// FileStorer is an interface for storing, retrieving info about, and deleting uploaded files.
type FileStorer interface {
	// Store saves the content from the reader and returns its FileInfo.
	// The hash is computed from content (SHA256, base32-encoded).
	Store(content io.Reader) (FileInfo, error)
	// GetInfo returns metadata about a file identified by its hash.
	// Returns os.ErrNotExist if the file is not found.
	// Returns ErrInvalidHash if the hash fails validation.
	GetInfo(hash string) (FileInfo, error)
	// Delete removes a file identified by its hash.
	// Returns os.ErrNotExist if the file is not found.
	// Returns ErrInvalidHash if the hash fails validation.
	Delete(hash string) error
}
