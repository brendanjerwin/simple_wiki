package pagestore

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Store is the disk-backed storage primitive for wiki pages. Owns raw I/O,
// per-page locks, and the write-side canonicalization step. Reads stay
// pure (the CanonicalReader decorator wraps Store to add canonicalize-on-
// return for the read path).
//
// The write-side canonicalizer is injected as a dependency so the same
// Store type works with the noop default (Phase 3 wiring) or the real
// frontmatter canonicalizer (Phase 4 wiring) without behavior leaking
// out of pagestore.
type Store struct {
	pathToData    string
	canonicalizer FrontmatterCanonicalizer

	// pageLocks holds one *sync.Mutex per page, keyed by CanonicalLockKey(id).
	pageLocks sync.Map
}

// NewStore constructs a Store rooted at pathToData. The directory must
// already exist; Store does not create it. The store uses NoopCanonicalizer
// by default; call SetCanonicalizer (or use the Phase 4 wiring) to swap
// in the real frontmatter canonicalizer.
func NewStore(pathToData string) *Store {
	return &Store{pathToData: pathToData, canonicalizer: NoopCanonicalizer{}}
}

// SetCanonicalizer swaps the canonicalizer used on the write path. Safe to
// call at Site-construction time before any concurrent writes. If c is nil,
// the canonicalizer reverts to NoopCanonicalizer so the store is always in
// a callable state.
func (s *Store) SetCanonicalizer(c FrontmatterCanonicalizer) {
	if c == nil {
		c = NoopCanonicalizer{}
	}
	s.canonicalizer = c
}

// PathToData returns the data directory the Store is reading from / writing
// to. Exported for consumers (eager-backfill jobs, debug tooling) that need
// to enumerate or sample the on-disk layout.
func (s *Store) PathToData() string {
	return s.pathToData
}

// lockPage acquires the per-page mutex for the given identifier and returns
// the release function. Mirrors checklistmutator/mutator.go:709's pattern.
// Records the lock wait and holder count metrics so a 16-deep queue (the
// production incident this package was extracted to fix) is visible in
// observability without touching the call sites.
func (s *Store) lockPage(id string) func() {
	key := CanonicalLockKey(id)
	v, _ := s.pageLocks.LoadOrStore(key, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		// Defensive: every value in pageLocks is set as *sync.Mutex; a
		// wrong-type entry would be a programming bug. Fall back to a
		// fresh local mutex rather than panic.
		mu = &sync.Mutex{}
	}
	start := time.Now()
	mu.Lock()
	recordLockAcquired(time.Since(start))
	return func() {
		mu.Unlock()
		recordLockReleased()
	}
}

// getFilePaths returns the munged and original file paths for an identifier
// at the given extension. The munged path is the canonical on-disk form;
// the original path is the fallback for legacy pre-munge files.
func (s *Store) getFilePaths(identifier, extension string) (mungedPath, originalPath, mungedIdentifier string) {
	munged, err := wikiidentifiers.MungeIdentifier(identifier)
	if err != nil {
		// Fall back to using the original identifier if munging fails.
		munged = identifier
	}
	mungedPath = path.Join(s.pathToData, base32tools.EncodeToBase32(strings.ToLower(munged))+"."+extension)
	originalPath = path.Join(s.pathToData, base32tools.EncodeToBase32(strings.ToLower(identifier))+"."+extension)
	mungedIdentifier = munged
	return mungedPath, originalPath, mungedIdentifier
}

// readFileByIdentifier reads the .md file for the given identifier under
// the page's lock. Returns the actual identifier the file was found under
// (munged if the canonical file existed; raw if the legacy fallback path
// was used).
func (s *Store) readFileByIdentifier(identifier, extension string) (string, []byte, error) {
	unlock := s.lockPage(identifier)
	defer unlock()

	mungedPath, originalPath, mungedIdentifier := s.getFilePaths(identifier, extension)

	// First try with the munged identifier.
	b, err := os.ReadFile(mungedPath)
	if err == nil {
		return mungedIdentifier, b, nil
	}

	// Then try with the original identifier if that didn't work (older files).
	b, err = os.ReadFile(originalPath)
	if err == nil {
		return identifier, b, nil
	}

	return mungedIdentifier, nil, fmt.Errorf("failed to read file for identifier %s: %w", identifier, err)
}

// readRawTextLocked reads the raw .md file content for the given identifier
// without acquiring the lock. The caller must hold the page lock.
func (s *Store) readRawTextLocked(identifier string) (string, error) {
	mungedPath, originalPath, _ := s.getFilePaths(identifier, "md")
	if b, err := os.ReadFile(mungedPath); err == nil {
		return string(b), nil
	}
	b, err := os.ReadFile(originalPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// writeRawTextLocked writes the .md file content for the given identifier
// without acquiring the lock. The caller must hold the page lock.
//
// The canonicalizer runs over `text` before the bytes hit disk so every
// successful write produces canonical on-disk content. With the noop
// canonicalizer (Phase 3 default), input is unchanged; with the format
// canonicalizer (Phase 4 wiring), YAML→TOML conversion etc. happens here.
// A canonicalizer error fails the write — it's never silently dropped.
func (s *Store) writeRawTextLocked(identifier, text string) error {
	canonical, err := s.canonicalizer.Canonicalize([]byte(text))
	if err != nil {
		return fmt.Errorf("canonicalize page %s before write: %w", identifier, err)
	}
	filePath := path.Join(s.pathToData, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	if err := os.WriteFile(filePath, canonical, 0644); err != nil {
		return fmt.Errorf("failed to save page %s: %w", identifier, err)
	}
	return nil
}

// ModifyOrCreatePage atomically reads, modifies, and writes a page's full
// raw text under the page's lock. If the page does not exist on disk, the
// modifier is called with an empty string (creating the page from scratch).
// Returns any error from read, modify, or write.
//
// Indexing and other post-write side effects are the caller's responsibility
// — Store only owns disk I/O. The lock is released before this function
// returns.
func (s *Store) ModifyOrCreatePage(identifier string, modifier func(currentText string) (string, error)) error {
	unlock := s.lockPage(identifier)
	defer unlock()

	currentText, readErr := s.readRawTextLocked(identifier)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("failed to read page %s for modification: %w", identifier, readErr)
	}
	// If readErr is os.ErrNotExist, currentText is "" — modifier creates from scratch.

	newText, modErr := modifier(currentText)
	if modErr != nil {
		return modErr
	}

	return s.writeRawTextLocked(identifier, newText)
}

// SoftDeletePage moves the .md file for identifier into a timestamped
// __deleted__/ subdirectory. Returns os.ErrNotExist if the file did not
// exist; other errors propagate as-is. Caller is responsible for any
// post-deletion indexing / scheduling cleanup.
func (s *Store) SoftDeletePage(id wikipage.PageIdentifier) error {
	identifier := string(id)
	unlock := s.lockPage(identifier)
	defer unlock()

	timestamp := time.Now().Unix()
	deletedDir := path.Join(s.pathToData, "__deleted__", fmt.Sprintf("%d", timestamp))

	if err := os.MkdirAll(deletedDir, 0755); err != nil {
		return fmt.Errorf("failed to create deleted directory: %w", err)
	}

	mdPath := path.Join(s.pathToData, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	deletedMdPath := path.Join(deletedDir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	mdErr := os.Rename(mdPath, deletedMdPath)
	if mdErr != nil {
		if os.IsNotExist(mdErr) {
			return os.ErrNotExist
		}
		return fmt.Errorf("failed to move Markdown file for page %s: %w", identifier, mdErr)
	}

	return nil
}

// --- Page-lock metrics. Lazy-initialized via sync.Once so the package can
// be imported without an OTel provider configured; tests that don't wire
// observability see nil instruments and the calls below quietly skip.

var (
	lockMetricsInitOnce sync.Once
	lockWaitSeconds     metric.Float64Histogram
	lockHolders         metric.Int64UpDownCounter
)

func initLockMetrics() {
	lockMetricsInitOnce.Do(func() {
		meter := otel.Meter("simple_wiki/pagestore")
		if hist, err := meter.Float64Histogram(
			"wiki_page_lock_wait_seconds",
			metric.WithDescription("How long a caller waited to acquire a page lock"),
			metric.WithUnit("s"),
		); err == nil {
			lockWaitSeconds = hist
		}
		if gauge, err := meter.Int64UpDownCounter(
			"wiki_page_lock_holders",
			metric.WithDescription("Number of page locks currently held"),
			metric.WithUnit("{lock}"),
		); err == nil {
			lockHolders = gauge
		}
	})
}

func recordLockAcquired(waitDuration time.Duration) {
	initLockMetrics()
	ctx := context.Background()
	if lockWaitSeconds != nil {
		lockWaitSeconds.Record(ctx, waitDuration.Seconds())
	}
	if lockHolders != nil {
		lockHolders.Add(ctx, 1)
	}
}

func recordLockReleased() {
	if lockHolders != nil {
		lockHolders.Add(context.Background(), -1)
	}
}
