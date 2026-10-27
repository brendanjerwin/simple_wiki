package chatattachment

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultTTL is how long an attachment lives before being reaped.
	// Matches chatbuffer.BufferInactivityTimeout (24 h).
	DefaultTTL = 24 * time.Hour

	// MaxAttachmentBytes is the maximum allowed byte size for a single attachment.
	MaxAttachmentBytes = 20 * 1024 * 1024 // 20 MiB

	// reapEveryN triggers a full sweep of expired entries after this many Store calls.
	// Keeps memory bounded without a background goroutine.
	reapEveryN = 50
)

// entry is an in-memory attachment entry with an expiry timestamp.
type entry struct {
	attachment *Attachment
	expiresAt  time.Time
}

// MemoryAttachmentStore is a concurrency-safe, in-memory attachment store with TTL.
// All stored data is transient: it is not written to disk and is lost on process exit.
// Expired entries are lazily reaped on Store calls (every reapEveryN stores).
type MemoryAttachmentStore struct {
	mu         sync.RWMutex
	entries    map[string]*entry
	ttl        time.Duration
	storeCount atomic.Int64
}

// NewMemoryAttachmentStore returns a new in-memory attachment store with the given TTL.
func NewMemoryAttachmentStore(ttl time.Duration) *MemoryAttachmentStore {
	return &MemoryAttachmentStore{
		entries: make(map[string]*entry),
		ttl:     ttl,
	}
}

// Store saves the content from the reader under a fresh UUID and returns that ID.
func (s *MemoryAttachmentStore) Store(filename, mimeType string, content io.Reader) (string, error) {
	if filename == "" {
		return "", errors.New("filename is required")
	}
	if mimeType == "" {
		return "", errors.New("mimeType is required")
	}

	data, err := io.ReadAll(io.LimitReader(content, MaxAttachmentBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read attachment content: %w", err)
	}
	if len(data) > MaxAttachmentBytes {
		return "", fmt.Errorf("attachment exceeds maximum size of %d bytes", MaxAttachmentBytes)
	}

	id := uuid.New().String()

	s.mu.Lock()
	s.entries[id] = &entry{
		attachment: &Attachment{
			ID:       id,
			Filename: filename,
			MIMEType: mimeType,
			Data:     data,
		},
		expiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()

	// Lazy reap: sweep expired entries every reapEveryN stores to bound memory growth.
	if s.storeCount.Add(1)%reapEveryN == 0 {
		s.reapExpired()
	}

	return id, nil
}

// Get retrieves a non-expired attachment by ID.
func (s *MemoryAttachmentStore) Get(id string) (*Attachment, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}

	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(e.expiresAt) {
		return nil, ErrNotFound
	}
	return e.attachment, nil
}

// Delete removes an attachment from the store.
func (s *MemoryAttachmentStore) Delete(id string) error {
	if err := validateID(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[id]; !ok {
		return ErrNotFound
	}
	delete(s.entries, id)
	return nil
}

// reapExpired removes all expired entries.
func (s *MemoryAttachmentStore) reapExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, id)
		}
	}
}

// validateID rejects IDs that contain path-traversal characters or are empty.
func validateID(id string) error {
	if id == "" {
		return ErrInvalidID
	}
	if strings.Contains(id, "/") || strings.Contains(id, "..") || strings.ContainsRune(id, 0) {
		return fmt.Errorf("%w: contains invalid characters", ErrInvalidID)
	}
	return nil
}
