// Package syspage embeds the wiki's help corpus into the binary and syncs it
// to the page store on startup. Pages marked with `system = true` in their
// frontmatter are sourced from the binary, not user-editable through the API.
package syspage

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

//go:embed embedded/**
var embeddedFS embed.FS

// embeddedRoot is the directory inside the embedded FS that holds the help
// corpus. Each `.md` file under this directory is one system page.
const embeddedRoot = "embedded"

// systemKey is the top-level frontmatter key that flags a page as
// system-owned. Mutation handlers consult this on the existing on-disk page
// before allowing a write.
const systemKey = "system"

// Page is one embedded help page parsed into its component parts.
type Page struct {
	Identifier  string
	Markdown    string
	Frontmatter map[string]any
}

// Logger is the minimal logger interface Sync needs. The wiki's existing
// logger satisfies this without any adaptation.
type Logger interface {
	Info(format string, args ...any)
	Debug(format string, args ...any)
}

// LoadEmbedded walks the package's embedded `.md` corpus and returns one
// Page per file. Files without `identifier` in their frontmatter are
// rejected — embedded pages must declare their canonical identifier so they
// don't collide with arbitrary user content.
func LoadEmbedded() ([]Page, error) {
	var pages []Page

	walkErr := fs.WalkDir(embeddedFS, embeddedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}

		raw, readErr := fs.ReadFile(embeddedFS, p)
		if readErr != nil {
			return fmt.Errorf("read embedded file %s: %w", p, readErr)
		}

		mdBytes, fm, parseErr := wikipage.ParseFrontmatterAndMarkdown(string(raw))
		if parseErr != nil {
			return fmt.Errorf("parse embedded file %s: %w", p, parseErr)
		}

		identifier, ok := fm["identifier"].(string)
		if !ok || identifier == "" {
			// Fall back to the filename stem so we still surface the file
			// path in error messages. Reject if absent — every embedded page
			// must declare its identifier explicitly.
			fileStem := strings.TrimSuffix(path.Base(p), ".md")
			return fmt.Errorf("embedded file %s is missing required `identifier` frontmatter (filename stem %q)", p, fileStem)
		}

		pages = append(pages, Page{
			Identifier:  identifier,
			Markdown:    string(mdBytes),
			Frontmatter: fm,
		})
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}
	return pages, nil
}

// IsSystemPage reports whether the given frontmatter map flags the page as
// system-owned (i.e. sourced from the binary, not user-editable). Recognises
// both bool true and the string "true" so TOML/JSON coercion variations are
// handled uniformly.
func IsSystemPage(fm map[string]any) bool {
	if fm == nil {
		return false
	}
	v, ok := fm[systemKey]
	if !ok {
		return false
	}
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

// Sync writes every embedded page to the store via the standard write path,
// but only when the embedded content differs from what's currently on disk.
// Writes go through the same WriteFrontMatter/WriteMarkdown methods that user
// edits use, so indexing and other downstream hooks fire identically.
func Sync(rw wikipage.PageReaderMutator, log Logger) error {
	pages, err := LoadEmbedded()
	if err != nil {
		return fmt.Errorf("load embedded system pages: %w", err)
	}

	for _, p := range pages {
		id := wikipage.PageIdentifier(p.Identifier)

		needsWrite, reason, readErr := pageDiffers(rw, id, p)
		if readErr != nil {
			return fmt.Errorf("compare system page %s: %w", p.Identifier, readErr)
		}

		if !needsWrite {
			log.Debug("System page unchanged: %s", p.Identifier)
			continue
		}

		if err := rw.WriteFrontMatter(id, p.Frontmatter); err != nil {
			return fmt.Errorf("write frontmatter for system page %s: %w", p.Identifier, err)
		}
		if err := rw.WriteMarkdown(id, wikipage.Markdown(p.Markdown)); err != nil {
			return fmt.Errorf("write markdown for system page %s: %w", p.Identifier, err)
		}
		log.Info("System page synced (%s): %s", reason, p.Identifier)
	}

	return nil
}

// pageDiffers reports whether the on-disk page differs from the embedded
// source. An absent on-disk page counts as a difference. The reason string
// is suitable for logging at Info level when a write happens.
func pageDiffers(rw wikipage.PageReaderMutator, id wikipage.PageIdentifier, embeddedPage Page) (bool, string, error) {
	_, fm, err := rw.ReadFrontMatter(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, "new", nil
		}
		return false, "", err
	}

	_, md, err := rw.ReadMarkdown(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, "new", nil
		}
		return false, "", err
	}

	if string(md) != embeddedPage.Markdown {
		return true, "markdown drift", nil
	}
	if !reflect.DeepEqual(coerceFrontmatter(fm), coerceFrontmatter(embeddedPage.Frontmatter)) {
		return true, "frontmatter drift", nil
	}
	return false, "", nil
}

// coerceFrontmatter normalizes frontmatter for comparison purposes. Both sides
// of the comparison go through this so we treat round-tripped values that
// only differ in their concrete Go type (e.g. int vs int64) as equal.
func coerceFrontmatter(fm map[string]any) map[string]any {
	if fm == nil {
		return nil
	}
	out := make(map[string]any, len(fm))
	for k, v := range fm {
		out[k] = coerceValue(v)
	}
	return out
}

func coerceValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return coerceFrontmatter(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = coerceValue(item)
		}
		return out
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float32:
		return float64(typed)
	default:
		return v
	}
}
