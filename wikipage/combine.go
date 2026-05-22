package wikipage

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pelletier/go-toml/v2"
)

// Delimiters and separators used when serializing a page (frontmatter +
// body) back to the on-disk form. The TOML delimiter is "+++" with a
// trailing newline so consumers can rely on a known structure.
const (
	tomlDelimiter = "+++\n"
	newline       = "\n"
)

// CombineFrontMatterAndMarkdown serializes a frontmatter map and a markdown
// body into the canonical on-disk representation. The inverse operation
// is *Page.GetFrontMatter / *Page.GetMarkdown.
//
// Empty inputs produce an empty string (no delimiters); a non-empty
// frontmatter is bracketed with `+++` delimiters before the body.
//
// Moved out of the server package so the pagestore package can serialize
// pages without circular dependencies. The combine logic is page-shape
// knowledge, not server knowledge.
func CombineFrontMatterAndMarkdown(fm FrontMatter, md Markdown) (string, error) {
	fmBytes, err := toml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	// If there's no content, no need to write anything.
	if len(fm) == 0 && len(md) == 0 {
		return "", nil
	}

	var content bytes.Buffer
	if len(fm) > 0 {
		if err := writeFrontmatterToBuffer(&content, fmBytes); err != nil {
			return "", err
		}
	}
	if _, err := content.WriteString(string(md)); err != nil {
		return "", err
	}
	return content.String(), nil
}

// writeFrontmatterToBuffer writes `+++\n<fmBytes>\n+++\n` to the writer.
// If fmBytes already ends in a newline, the second newline is elided.
func writeFrontmatterToBuffer(content io.Writer, fmBytes []byte) error {
	if _, err := io.WriteString(content, tomlDelimiter); err != nil {
		return err
	}
	if _, err := content.Write(fmBytes); err != nil {
		return err
	}
	if !bytes.HasSuffix(fmBytes, []byte(newline)) {
		if _, err := io.WriteString(content, newline); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(content, tomlDelimiter); err != nil {
		return err
	}
	return nil
}
