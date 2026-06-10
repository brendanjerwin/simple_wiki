package pagestore

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// WriteFrontMatter atomically reads the page's current markdown body,
// combines it with fm, and writes the result back under the page's lock.
// Side effects beyond bytes-on-disk (indexing, scheduling) are NOT
// performed here — those are the caller's responsibility.
func (s *Store) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	return s.ModifyOrCreatePage(string(id), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}
		md, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for frontmatter write: %w", err)
		}
		return wikipage.CombineFrontMatterAndMarkdown(fm, md)
	})
}

// WriteMarkdown atomically reads the page's current frontmatter, combines
// it with md, and writes the result back under the page's lock.
func (s *Store) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	return s.ModifyMarkdown(id, func(_ wikipage.Markdown) (wikipage.Markdown, error) {
		return md, nil
	})
}

// ModifyMarkdown atomically reads the markdown section, calls fn, and
// writes the result back while preserving the existing frontmatter.
// The full read-modify-write is held under the page's lock.
func (s *Store) ModifyMarkdown(id wikipage.PageIdentifier, fn func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return s.ModifyOrCreatePage(string(id), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}

		currentMD, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for modification: %w", err)
		}

		newMD, err := fn(currentMD)
		if err != nil {
			return "", err
		}

		currentFM, err := p.GetFrontMatter()
		if err != nil {
			return "", fmt.Errorf("failed to parse frontmatter during markdown modification: %w", err)
		}

		return wikipage.CombineFrontMatterAndMarkdown(currentFM, newMD)
	})
}

// ModifyFrontMatterAndMarkdown atomically reads both page sections, calls fn,
// and writes both returned sections under the page lock.
func (s *Store) ModifyFrontMatterAndMarkdown(id wikipage.PageIdentifier, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error)) error {
	return s.ModifyOrCreatePage(string(id), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}

		currentFM, err := p.GetFrontMatter()
		if err != nil {
			return "", fmt.Errorf("failed to parse frontmatter for page modification: %w", err)
		}
		currentMD, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for page modification: %w", err)
		}

		newFM, newMD, err := fn(currentFM, currentMD)
		if err != nil {
			return "", err
		}
		return wikipage.CombineFrontMatterAndMarkdown(newFM, newMD)
	})
}
