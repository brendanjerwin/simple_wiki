// Package history provides a persistent Bleve index over page version history.
package history

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search/query"
	"github.com/brendanjerwin/simple_wiki/wikipage"

	// Register the keyword analyzer for exact-match fields.
	_ "github.com/blevesearch/bleve/analysis/analyzer/keyword"
)

const (
	fieldPageIdentifier = "page_identifier"
	fieldVersionID      = "version_id"
	fieldContent        = "content"
	fieldAuthor         = "author"
	fieldCreatedAt      = "created_at"
	fieldIsAgent        = "is_agent"
	fieldSource         = "source"
	fieldSHA256         = "sha256"
	fieldByteSize       = "byte_size"
)

// PageVersionMetadata is the metadata for a single historical page version.
type PageVersionMetadata struct {
	VersionID      string
	PageIdentifier string
	CreatedAt      time.Time
	Author         string
	IsAgent        bool
	Source         string
	SHA256         string
	ByteSize       int64
}

// HistoryReader is the read-only surface this index needs from the page store.
type HistoryReader interface {
	// ListVersions returns metadata for every version of a page, newest-first.
	ListVersions(identifier wikipage.PageIdentifier) ([]PageVersionMetadata, error)

	// ReadVersion returns the full content of a specific version.
	ReadVersion(identifier wikipage.PageIdentifier, versionID string) (string, error)
}

// HistorySearchFilter holds optional filters for a global history search.
type HistorySearchFilter struct {
	Query          string
	PageFilter string
	AuthorFilter   string
	From           time.Time
	To             time.Time
}

// HistorySearchResult is a single result from a history search.
type HistorySearchResult struct {
	Page string
	Version  PageVersionMetadata
	Snippet  string
}

// Index is a Bleve-backed index of page version history.
type Index struct {
	index  bleve.Index
	reader HistoryReader
	mu     sync.Mutex
}

// NewIndex creates a new history index backed by an in-memory Bleve index.
func NewIndex(reader HistoryReader) (*Index, error) {
	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "en"

	keyword := bleve.NewTextFieldMapping()
	keyword.Analyzer = "keyword"
	mapping.DefaultMapping.AddFieldMappingsAt(fieldPageIdentifier, keyword)
	mapping.DefaultMapping.AddFieldMappingsAt(fieldVersionID, keyword)
	mapping.DefaultMapping.AddFieldMappingsAt(fieldAuthor, keyword)
	mapping.DefaultMapping.AddFieldMappingsAt(fieldSource, keyword)
	mapping.DefaultMapping.AddFieldMappingsAt(fieldSHA256, keyword)

	dateMapping := bleve.NewDateTimeFieldMapping()
	mapping.DefaultMapping.AddFieldMappingsAt(fieldCreatedAt, dateMapping)

	boolMapping := bleve.NewBooleanFieldMapping()
	mapping.DefaultMapping.AddFieldMappingsAt(fieldIsAgent, boolMapping)

	numMapping := bleve.NewNumericFieldMapping()
	mapping.DefaultMapping.AddFieldMappingsAt(fieldByteSize, numMapping)

	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create history bleve index: %w", err)
	}

	return &Index{
		index:  idx,
		reader: reader,
	}, nil
}

// AddPageToIndex indexes every historical version of the requested page.
func (i *Index) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	versions, err := i.reader.ListVersions(identifier)
	if err != nil {
		return fmt.Errorf("failed to list versions for %s: %w", identifier, err)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if err := i.removeVersionsForPageLocked(identifier); err != nil {
		return err
	}

	for _, v := range versions {
		content, err := i.reader.ReadVersion(identifier, v.VersionID)
		if err != nil {
			return fmt.Errorf("failed to read version %s for %s: %w", v.VersionID, identifier, err)
		}

		doc := map[string]interface{}{
			fieldPageIdentifier: string(identifier),
			fieldVersionID:      v.VersionID,
			fieldContent:        content,
			fieldAuthor:         v.Author,
			fieldCreatedAt:      v.CreatedAt,
			fieldIsAgent:        v.IsAgent,
			fieldSource:         v.Source,
			fieldSHA256:         v.SHA256,
			fieldByteSize:       v.ByteSize,
		}

		id := docID(identifier, v.VersionID)
		if err := i.index.Index(id, doc); err != nil {
			return fmt.Errorf("failed to index version %s for %s: %w", v.VersionID, identifier, err)
		}
	}

	return nil
}

// RemovePageFromIndex removes every indexed version of the requested page.
func (i *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.removeVersionsForPageLocked(identifier)
}

func (i *Index) removeVersionsForPageLocked(identifier wikipage.PageIdentifier) error {
	ids, err := i.docIDsForPageLocked(identifier)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := i.index.Delete(id); err != nil {
			return fmt.Errorf("failed to remove history version %s for %s: %w", id, identifier, err)
		}
	}
	return nil
}

func (i *Index) docIDsForPageLocked(identifier wikipage.PageIdentifier) ([]string, error) {
	q := bleve.NewTermQuery(string(identifier))
	q.SetField(fieldPageIdentifier)
	req := bleve.NewSearchRequest(q)
	req.Fields = []string{}
	req.Size = 10000
	res, err := i.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("failed to find history versions for %s: %w", identifier, err)
	}
	ids := make([]string, 0, len(res.Hits))
	for _, hit := range res.Hits {
		ids = append(ids, hit.ID)
	}
	return ids, nil
}

func docID(identifier wikipage.PageIdentifier, versionID string) string {
	return string(identifier) + "/" + versionID
}

// SearchPageHistory searches within the history of a single page.
func (i *Index) SearchPageHistory(identifier wikipage.PageIdentifier, queryText string) ([]HistorySearchResult, error) {
	return i.SearchHistory(HistorySearchFilter{
		Query:          queryText,
		PageFilter: string(identifier),
	})
}

// SearchHistory searches across all indexed history with optional filters.
func (i *Index) SearchHistory(filter HistorySearchFilter) ([]HistorySearchResult, error) {
	var clauses []query.Query

	if filter.PageFilter != "" {
		pq := bleve.NewTermQuery(filter.PageFilter)
		pq.SetField(fieldPageIdentifier)
		clauses = append(clauses, pq)
	}

	var textQuery query.Query
	if strings.TrimSpace(filter.Query) != "" {
		mq := bleve.NewMatchQuery(filter.Query)
		mq.SetField(fieldContent)
		textQuery = mq
	} else {
		textQuery = bleve.NewMatchAllQuery()
	}
	clauses = append(clauses, textQuery)

	if filter.AuthorFilter != "" {
		aq := bleve.NewTermQuery(filter.AuthorFilter)
		aq.SetField(fieldAuthor)
		clauses = append(clauses, aq)
	}

	if !filter.From.IsZero() || !filter.To.IsZero() {
		drq := bleve.NewDateRangeQuery(filter.From, filter.To)
		drq.SetField(fieldCreatedAt)
		clauses = append(clauses, drq)
	}

	q := bleve.NewConjunctionQuery(clauses...)

	req := bleve.NewSearchRequest(q)
	req.Fields = []string{
		fieldPageIdentifier,
		fieldVersionID,
		fieldAuthor,
		fieldCreatedAt,
		fieldIsAgent,
		fieldSource,
		fieldSHA256,
		fieldByteSize,
	}
	req.Highlight = bleve.NewHighlightWithStyle("html")
	req.Highlight.AddField(fieldContent)
	req.Size = 10000

	res, err := i.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("history search failed: %w", err)
	}

	results := make([]HistorySearchResult, 0, len(res.Hits))
	for _, hit := range res.Hits {
		v := PageVersionMetadata{
			VersionID:      fieldString(hit.Fields, fieldVersionID),
			PageIdentifier: fieldString(hit.Fields, fieldPageIdentifier),
			Author:         fieldString(hit.Fields, fieldAuthor),
			Source:         fieldString(hit.Fields, fieldSource),
			SHA256:         fieldString(hit.Fields, fieldSHA256),
			CreatedAt:      fieldTime(hit.Fields, fieldCreatedAt),
			IsAgent:        fieldBool(hit.Fields, fieldIsAgent),
			ByteSize:       fieldInt64(hit.Fields, fieldByteSize),
		}

		snippet := strings.Join(hit.Fragments[fieldContent], " ... ")
		if snippet == "" {
			snippet = fieldString(hit.Fields, fieldContent)
		}

		results = append(results, HistorySearchResult{
			Page: v.PageIdentifier,
			Version:  v,
			Snippet:  snippet,
		})
	}

	return results, nil
}

func fieldString(fields map[string]interface{}, name string) string {
	if v, ok := fields[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func fieldTime(fields map[string]interface{}, name string) time.Time {
	if v, ok := fields[name]; ok {
		switch t := v.(type) {
		case time.Time:
			return t
		case *time.Time:
			if t != nil {
				return *t
			}
		case string:
			if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func fieldBool(fields map[string]interface{}, name string) bool {
	if v, ok := fields[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func fieldInt64(fields map[string]interface{}, name string) int64 {
	if v, ok := fields[name]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return 0
}
