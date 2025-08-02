package core

import (
)

// FrontmatterIndexIndexQueryer is an interface for querying the frontmatter index.
type FrontmatterIndexIndexQueryer interface {
	QueryExactMatch(dottedKeyPath string, value string) []string
	QueryKeyExistence(dottedKeyPath string) []string
	QueryPrefixMatch(dottedKeyPath string, valuePrefix string) []string
	GetValue(identifier string, dottedKeyPath string) string
}

// MarkdownRenderer is an interface that abstracts the rendering process
type MarkdownRenderer interface {
	Render(input []byte) ([]byte, error)
}

// FrontmatterMigrationApplicator is an interface for applying frontmatter migrations.
type FrontmatterMigrationApplicator interface {
	ApplyMigrations(content []byte) ([]byte, error)
}

// Logger is an interface for logging.
type Logger interface {
	Trace(format string, v ...any)
	Debug(format string, v ...any)
	Info(format string, v ...any)
	Warn(format string, v ...any)
	Error(format string, v ...any)
	Fatal(format string, v ...any)
}

// IndexMaintainer is an interface for maintaining the page index.
type IndexMaintainer interface {
	AddPageToIndex(identifier string) error
	RemovePageFromIndex(identifier string) error
}

// FrontmatterIndexQueryer is an interface for querying the frontmatter index.
type FrontmatterIndexQueryer interface {
	QueryExactMatch(dottedKeyPath string, value string) []string
	QueryKeyExistence(dottedKeyPath string) []string
	QueryPrefixMatch(dottedKeyPath string, valuePrefix string) []string
	GetValue(identifier string, dottedKeyPath string) string
}

// BleveIndexQueryer is an interface for querying the Bleve index.
type BleveIndexQueryer interface {
	Query(query string) ([]string, error)
}


