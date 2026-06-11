package pagestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
)

const (
	deletedDirName       = "__deleted__"
	trashMetadataName    = "metadata.toml"
	trashRetentionPeriod = 30 * 24 * time.Hour
	decimalBase          = 10
	int64BitSize         = 64
)

type trashMetadata struct {
	Identifier string    `toml:"identifier"`
	Title      string    `toml:"title"`
	DeletedAt  time.Time `toml:"deleted_at"`
	DeletedBy  string    `toml:"deleted_by"`
	PurgesAt   time.Time `toml:"purges_at"`
}

func trashIDFromTime(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), decimalBase)
}

func (s *Store) deletedRoot() string {
	return filepath.Join(s.pathToData, deletedDirName)
}

func (s *Store) trashDir(trashID string) (string, error) {
	if trashID == "" || strings.ContainsAny(trashID, `/\`) {
		return "", fmt.Errorf("invalid trash id: %q", trashID)
	}

	root := s.deletedRoot()
	dir := filepath.Join(root, trashID)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", fmt.Errorf("resolve trash path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid trash id: %q", trashID)
	}

	return dir, nil
}

func (s *Store) createTrashDir(now time.Time) (trashID string, deletedDir string, err error) {
	if err := os.MkdirAll(s.deletedRoot(), 0755); err != nil {
		return "", "", fmt.Errorf("create deleted root: %w", err)
	}
	for offset := int64(0); ; offset++ {
		trashID := trashIDFromTime(now.Add(time.Duration(offset)))
		deletedDir, err := s.trashDir(trashID)
		if err != nil {
			return "", "", err
		}
		if err := os.Mkdir(deletedDir, 0755); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", "", fmt.Errorf("failed to create deleted directory: %w", err)
		}
		return trashID, deletedDir, nil
	}
}

func trashTitle(identifier string, rawText []byte) string {
	page := &wikipage.Page{Identifier: identifier, Text: string(rawText), WasLoadedFromDisk: true}
	fm, err := page.GetFrontMatter()
	if err != nil {
		return ""
	}
	title, ok := fm["title"].(string)
	if !ok {
		return ""
	}
	return title
}

func trashEntryFromMetadata(trashID string, metadata trashMetadata) wikipage.TrashEntry {
	return wikipage.TrashEntry{
		TrashID:    trashID,
		Identifier: wikipage.PageIdentifier(metadata.Identifier),
		Title:      metadata.Title,
		DeletedAt:  metadata.DeletedAt,
		DeletedBy:  metadata.DeletedBy,
		PurgesAt:   metadata.PurgesAt,
	}
}

func writeTrashMetadata(trashDir string, metadata trashMetadata) error {
	b, err := toml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal trash metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, trashMetadataName), b, 0644); err != nil {
		return fmt.Errorf("write trash metadata: %w", err)
	}
	return nil
}

func readTrashMetadata(trashDir string) (trashMetadata, error) {
	var metadata trashMetadata
	b, err := os.ReadFile(filepath.Join(trashDir, trashMetadataName))
	if err != nil {
		return trashMetadata{}, err
	}
	if err := toml.Unmarshal(b, &metadata); err != nil {
		return trashMetadata{}, fmt.Errorf("parse trash metadata: %w", err)
	}
	return metadata, nil
}

func firstMarkdownInTrash(trashDir string) (string, error) {
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		return filepath.Join(trashDir, entry.Name()), nil
	}
	return "", os.ErrNotExist
}

func legacyTrashMetadata(trashID, trashDir string) (trashMetadata, error) {
	mdPath, err := firstMarkdownInTrash(trashDir)
	if err != nil {
		return trashMetadata{}, err
	}
	encodedIdentifier := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	identifier, err := base32tools.DecodeFromBase32(encodedIdentifier)
	if err != nil {
		return trashMetadata{}, fmt.Errorf("decode legacy trash identifier: %w", err)
	}
	deletedUnix, err := strconv.ParseInt(trashID, decimalBase, int64BitSize)
	if err != nil {
		deletedUnix = time.Now().Unix()
	}
	deletedAt := time.Unix(deletedUnix, 0).UTC()
	return trashMetadata{
		Identifier: identifier,
		DeletedAt:  deletedAt,
		PurgesAt:   deletedAt.Add(trashRetentionPeriod),
	}, nil
}

func loadTrashMetadata(trashID, trashDir string) (trashMetadata, error) {
	metadata, err := readTrashMetadata(trashDir)
	if err == nil {
		return metadata, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return trashMetadata{}, err
	}
	return legacyTrashMetadata(trashID, trashDir)
}

// SoftDeletePage moves the .md file for identifier into trash using a blank
// deleted-by actor.
func (s *Store) SoftDeletePage(id wikipage.PageIdentifier) error {
	return s.SoftDeletePageBy(id, "")
}

// SoftDeletePageBy moves the .md file for identifier into trash with restore
// metadata. The page is hidden from normal reads because only root .md files
// are considered live pages.
func (s *Store) SoftDeletePageBy(id wikipage.PageIdentifier, deletedBy string) error {
	identifier := string(id)
	unlock := s.lockPage(identifier)
	defer unlock()

	now := time.Now().UTC()
	mdPath := filepath.Join(s.pathToData, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	rawText, readErr := os.ReadFile(mdPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return os.ErrNotExist
		}
		return fmt.Errorf("failed to read Markdown file for page %s before trashing: %w", identifier, readErr)
	}

	_, deletedDir, err := s.createTrashDir(now)
	if err != nil {
		return err
	}

	deletedMdPath := filepath.Join(deletedDir, filepath.Base(mdPath))
	if err := os.Rename(mdPath, deletedMdPath); err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("failed to move Markdown file for page %s: %w", identifier, err)
	}

	metadata := trashMetadata{
		Identifier: identifier,
		Title:      trashTitle(identifier, rawText),
		DeletedAt:  now,
		DeletedBy:  deletedBy,
		PurgesAt:   now.Add(trashRetentionPeriod),
	}
	if err := writeTrashMetadata(deletedDir, metadata); err != nil {
		if restoreErr := os.Rename(deletedMdPath, mdPath); restoreErr != nil {
			return fmt.Errorf("failed to write trash metadata and restore page %s: %w", identifier, errors.Join(err, restoreErr))
		}
		return err
	}

	return nil
}

// ListTrash returns all restoreable trash entries. Expired entries are purged
// before the list is returned.
func (s *Store) ListTrash() ([]wikipage.TrashEntry, error) {
	if err := s.PurgeExpiredTrash(time.Now().UTC()); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.deletedRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read trash root: %w", err)
	}

	trashEntries := make([]wikipage.TrashEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		trashID := entry.Name()
		trashDir := filepath.Join(s.deletedRoot(), trashID)
		metadata, err := loadTrashMetadata(trashID, trashDir)
		if err != nil {
			continue
		}
		trashEntries = append(trashEntries, trashEntryFromMetadata(trashID, metadata))
	}

	return trashEntries, nil
}

// RestorePage moves a trashed page back into live page storage. Restore fails
// rather than overwriting a page that already exists at the original identifier.
func (s *Store) RestorePage(trashID string) error {
	trashDir, err := s.trashDir(trashID)
	if err != nil {
		return err
	}
	metadata, err := loadTrashMetadata(trashID, trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return err
	}
	identifier := metadata.Identifier

	unlock := s.lockPage(identifier)
	defer unlock()

	mungedPath, originalPath, _ := s.getFilePaths(identifier, "md")
	if _, err := os.Stat(mungedPath); err == nil {
		return fmt.Errorf("%w: %s already exists", wikipage.ErrPageRestoreConflict, identifier)
	}
	if _, err := os.Stat(originalPath); err == nil {
		return fmt.Errorf("%w: %s already exists", wikipage.ErrPageRestoreConflict, identifier)
	}

	trashMdPath, err := firstMarkdownInTrash(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return err
	}
	if err := os.Rename(trashMdPath, mungedPath); err != nil {
		return fmt.Errorf("restore page %s: %w", identifier, err)
	}
	if err := os.RemoveAll(trashDir); err != nil {
		return fmt.Errorf("remove restored trash directory: %w", err)
	}

	return nil
}

// PurgePage permanently removes a single trash entry.
func (s *Store) PurgePage(trashID string) error {
	trashDir, err := s.trashDir(trashID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(trashDir); err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("stat trash entry %s: %w", trashID, err)
	}
	if err := os.RemoveAll(trashDir); err != nil {
		return fmt.Errorf("purge trash entry %s: %w", trashID, err)
	}
	return nil
}

// EmptyTrash permanently removes every trash entry.
func (s *Store) EmptyTrash() (int, error) {
	entries, err := os.ReadDir(s.deletedRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read trash root: %w", err)
	}

	purged := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := s.PurgePage(entry.Name()); err != nil {
			return purged, err
		}
		purged++
	}

	return purged, nil
}

// PurgeExpiredTrash permanently removes trash entries whose retention window
// has elapsed.
func (s *Store) PurgeExpiredTrash(now time.Time) error {
	entries, err := os.ReadDir(s.deletedRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read trash root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		trashID := entry.Name()
		trashDir := filepath.Join(s.deletedRoot(), trashID)
		metadata, err := loadTrashMetadata(trashID, trashDir)
		if err != nil {
			continue
		}
		if metadata.PurgesAt.After(now) {
			continue
		}
		if err := s.PurgePage(trashID); err != nil {
			return err
		}
	}

	return nil
}
