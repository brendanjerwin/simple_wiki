package pagestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// historyDirName is the subdirectory under the data dir that holds all
// page version history. Sibling to __deleted__ (trash).
const historyDirName = "__history__"

// versionFileExt is the extension for version content snapshots.
const versionFileExt = ".md"

// versionMetaExt is the extension for version metadata sidecars.
const versionMetaExt = ".meta.json"

// VersionMetadata is the on-disk metadata for a single version snapshot.
type VersionMetadata struct {
	VersionID      string    `json:"version_id"`
	PageIdentifier string    `json:"page_identifier"`
	CreatedAt      time.Time `json:"created_at"`
	Author         string    `json:"author"`
	IsAgent        bool      `json:"is_agent"`
	Source         string    `json:"source"`
	SHA256         string    `json:"sha256"`
	ByteSize       int64     `json:"byte_size"`
}

// versionMetadataOnDisk is the JSON structure written to .meta.json files.
// Mirrors VersionMetadata but is kept separate so the on-disk format can
// evolve independently from the in-memory type.
type versionMetadataOnDisk struct {
	VersionID      string    `json:"version_id"`
	PageIdentifier string    `json:"page_identifier"`
	CreatedAt      time.Time `json:"created_at"`
	Author         string    `json:"author"`
	IsAgent        bool      `json:"is_agent"`
	Source         string    `json:"source"`
	SHA256         string    `json:"sha256"`
	ByteSize       int64     `json:"byte_size"`
}

// HistoryReader is the read-only surface for page version history.
// Consumers that only need to read/explore history should depend on this
// interface, not on *Store.
type HistoryReader interface {
	// ListVersions returns metadata for every version of a page, newest-first.
	ListVersions(identifier wikipage.PageIdentifier) ([]VersionMetadata, error)

	// ReadVersion returns the full content of a specific version.
	ReadVersion(identifier wikipage.PageIdentifier, versionID string) (string, error)

	// RestoreVersion writes a historical version's content back as the live
	// page. The current live content is captured as a new version first
	// (via the normal write path), so no history is lost.
	RestoreVersion(identifier wikipage.PageIdentifier, versionID string, identity wikipage.Identity) error

	// DiffVersions returns a unified diff between two versions of a page.
	DiffVersions(identifier wikipage.PageIdentifier, oldID, newID string) (string, error)
}

// historyRoot returns the path to the __history__ directory.
func (s *Store) historyRoot() string {
	return filepath.Join(s.pathToData, historyDirName)
}

// historyDir returns the path to a page's history directory, using the
// munged (base32-encoded) identifier so directories are filesystem-safe.
func (s *Store) historyDir(identifier string) string {
	munged := base32tools.EncodeToBase32(strings.ToLower(identifier))
	return filepath.Join(s.historyRoot(), munged)
}

// captureVersionLocked writes a version snapshot (content + metadata) to
// the page's history directory. The caller MUST hold the page lock.
//
// The content is the outgoing page text (the state being replaced).
// The identity provides author and is_agent for the metadata.
// The source describes what triggered the capture (write_frontmatter,
// write_markdown, modify_markdown, modify_fm_md, soft_delete, restore,
// migration).
//
// A capture failure does NOT block the live write — history is best-effort.
// The caller is responsible for logging the error.
func (s *Store) captureVersionLocked(identifier, content string, identity wikipage.Identity) error {
	return s.captureVersionLockedWithSource(identifier, content, identity, "modify")
}

// captureVersionLockedWithSource is the source-aware capture entry point.
// The source string is recorded in the version metadata for audit/search.
func (s *Store) captureVersionLockedWithSource(identifier, content string, identity wikipage.Identity, source string) error {
	now := time.Now().UTC()
	versionID := ulid.NewSystemGenerator().NewULID()

	sha := sha256.Sum256([]byte(content))
	shaHex := hex.EncodeToString(sha[:])

	author := ""
	isAgent := false
	if identity != nil {
		author = identity.LoginName()
		isAgent = identity.IsAgent()
	}

	dir := s.historyDir(identifier)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create history dir for %s: %w", identifier, err)
	}

	// Write content snapshot.
	contentPath := filepath.Join(dir, versionID+versionFileExt)
	if err := os.WriteFile(contentPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write history content for %s/%s: %w", identifier, versionID, err)
	}

	// Write metadata sidecar.
	meta := versionMetadataOnDisk{
		VersionID:      versionID,
		PageIdentifier: identifier,
		CreatedAt:      now,
		Author:         author,
		IsAgent:        isAgent,
		Source:         source,
		SHA256:         shaHex,
		ByteSize:       int64(len(content)),
	}
	metaPath := filepath.Join(dir, versionID+versionMetaExt)
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal history metadata for %s/%s: %w", identifier, versionID, err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write history metadata for %s/%s: %w", identifier, versionID, err)
	}

	return nil
}

// ListVersions returns metadata for every version of a page, newest-first.
// ULIDs are monotonically sortable, so descending filename order = newest-first.
func (s *Store) ListVersions(identifier wikipage.PageIdentifier) ([]VersionMetadata, error) {
	dir := s.historyDir(string(identifier))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read history dir for %s: %w", identifier, err)
	}

	var versions []VersionMetadata
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, versionMetaExt) {
			continue
		}

		versionID := strings.TrimSuffix(name, versionMetaExt)
		meta, err := s.readVersionMetadata(dir, versionID)
		if err != nil {
			return nil, fmt.Errorf("failed to read history metadata for %s/%s: %w", identifier, versionID, err)
		}

		versions = append(versions, VersionMetadata{
			VersionID:      meta.VersionID,
			PageIdentifier: meta.PageIdentifier,
			CreatedAt:      meta.CreatedAt,
			Author:         meta.Author,
			IsAgent:        meta.IsAgent,
			Source:         meta.Source,
			SHA256:         meta.SHA256,
			ByteSize:       meta.ByteSize,
		})
	}

	// Sort newest-first (ULID descending = newest first).
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionID > versions[j].VersionID
	})

	return versions, nil
}

// readVersionMetadata reads and parses a .meta.json file for a version.
func (s *Store) readVersionMetadata(dir, versionID string) (versionMetadataOnDisk, error) {
	metaPath := filepath.Join(dir, versionID+versionMetaExt)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return versionMetadataOnDisk{}, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta versionMetadataOnDisk
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return versionMetadataOnDisk{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return meta, nil
}

// ReadVersion returns the full content of a specific version of a page.
func (s *Store) ReadVersion(identifier wikipage.PageIdentifier, versionID string) (string, error) {
	dir := s.historyDir(string(identifier))
	contentPath := filepath.Join(dir, versionID+versionFileExt)

	content, err := os.ReadFile(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("version %s not found for page %s: %w", versionID, identifier, os.ErrNotExist)
		}
		return "", fmt.Errorf("failed to read version %s for page %s: %w", versionID, identifier, err)
	}

	return string(content), nil
}

// RestoreVersion writes a historical version's content back as the live page.
// The current live content is captured as a new version first (via the normal
// write path), so no history is lost.
func (s *Store) RestoreVersion(identifier wikipage.PageIdentifier, versionID string, identity wikipage.Identity) error {
	content, err := s.ReadVersion(identifier, versionID)
	if err != nil {
		return err
	}

	// Write the historical content back as the live page. This goes through
	// ModifyOrCreatePage, which captures the current live content as a new
	// version before overwriting — so no history is lost.
	return s.ModifyOrCreatePage(string(identifier), identity, "restore", func(_ string) (string, error) {
		return content, nil
	})
}

// DiffVersions returns a unified diff between two versions of a page.
func (s *Store) DiffVersions(identifier wikipage.PageIdentifier, oldID, newID string) (string, error) {
	oldContent, err := s.ReadVersion(identifier, oldID)
	if err != nil {
		return "", fmt.Errorf("failed to read old version %s: %w", oldID, err)
	}

	newContent, err := s.ReadVersion(identifier, newID)
	if err != nil {
		return "", fmt.Errorf("failed to read new version %s: %w", newID, err)
	}

	return computeUnifiedDiff(oldContent, newContent), nil
}

// shouldSkipHistoryCapture checks whether the page's frontmatter has
// opted out of version history capture. Pages can set:
//
//	[wiki.history]
//	opt_out = true
//
// to disable history capture for that page. This is useful for
// system/metrics pages that are written frequently and don't benefit
// from version history. The check is on the OUTGOING content (the
// state being replaced) — if the page had opted out, we don't capture.
func shouldSkipHistoryCapture(pageText string) bool {
	p := &wikipage.Page{Text: pageText}
	fm, err := p.GetFrontMatter()
	if err != nil {
		return false // can't parse frontmatter → capture by default
	}
	wiki, ok := fm["wiki"].(map[string]any)
	if !ok {
		return false
	}
	history, ok := wiki["history"].(map[string]any)
	if !ok {
		return false
	}
	optOut, ok := history["opt_out"].(bool)
	return ok && optOut
}
