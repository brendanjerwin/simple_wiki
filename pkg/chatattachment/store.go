// Package chatattachment provides transient, in-memory storage for chat
// attachments (images, files) scoped to a session lifetime. Attachments are
// not persisted to disk and are not exported or included in logs/backups.
package chatattachment

import (
	"errors"
	"io"
)

// ErrNotFound is returned when an attachment ID does not exist in the store.
var ErrNotFound = errors.New("attachment not found")

// ErrInvalidID is returned when an attachment ID fails validation.
var ErrInvalidID = errors.New("invalid attachment ID")

// Attachment holds metadata and content for a single uploaded chat attachment.
type Attachment struct {
	ID       string
	Filename string
	MIMEType string
	Data     []byte
}

// AttachmentStore is the interface for transient chat attachment storage.
type AttachmentStore interface {
	// Store saves the attachment content and returns an ID.
	// The ID is a URL-safe opaque token unique to this store instance.
	Store(filename, mimeType string, content io.Reader) (id string, err error)

	// Get retrieves an attachment by ID.
	// Returns ErrNotFound if the ID is unknown or expired.
	// Returns ErrInvalidID if the ID fails validation.
	Get(id string) (*Attachment, error)

	// Delete removes an attachment by ID.
	// Returns ErrNotFound if the ID is unknown or already expired.
	Delete(id string) error
}
