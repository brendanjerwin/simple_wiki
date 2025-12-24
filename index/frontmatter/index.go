// Package frontmatter provides functionality for indexing and querying page frontmatter.
package frontmatter

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

type (
	// DottedKeyPath represents a dot-separated path to a frontmatter key.
	DottedKeyPath = string
	// Value represents the value of a frontmatter key.
	Value = string
)

// IQueryFrontmatterIndex defines the interface for querying the frontmatter index.
type IQueryFrontmatterIndex interface {
	QueryExactMatch(dottedKeyPath DottedKeyPath, value Value) []wikipage.PageIdentifier
	QueryKeyExistence(dottedKeyPath DottedKeyPath) []wikipage.PageIdentifier
	QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier

	GetValue(identifier wikipage.PageIdentifier, dottedKeyPath DottedKeyPath) Value
}

// Index is a struct that maintains an inverted index of frontmatter keys and values.
type Index struct {
	InvertedIndex map[DottedKeyPath]map[Value][]wikipage.PageIdentifier
	PageKeyMap    map[wikipage.PageIdentifier]map[DottedKeyPath]map[Value]bool
	pageReader    wikipage.PageReader
	mu            sync.RWMutex // Protects concurrent access to maps
}

// NewIndex creates a new FrontmatterIndex.
func NewIndex(pageReader wikipage.PageReader) *Index {
	return &Index{
		InvertedIndex: make(map[DottedKeyPath]map[Value][]wikipage.PageIdentifier),
		PageKeyMap:    make(map[wikipage.PageIdentifier]map[DottedKeyPath]map[Value]bool),
		pageReader:    pageReader,
	}
}


// AddPageToIndex adds a page's frontmatter to the index.
func (f *Index) AddPageToIndex(requestedIdentifier wikipage.PageIdentifier) error {
	mungedIdentifier := wikiidentifiers.MungeIdentifier(requestedIdentifier)
	identifier, frontmatter, err := f.pageReader.ReadFrontMatter(requestedIdentifier)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.removePageFromIndexInternal(identifier)
	f.removePageFromIndexInternal(requestedIdentifier)
	f.removePageFromIndexInternal(mungedIdentifier)

	return f.recursiveAddFrontmatter(mungedIdentifier, "", frontmatter)
}

func (f *Index) saveToIndex(identifier wikipage.PageIdentifier, keyPath DottedKeyPath, value Value) {
	if f.InvertedIndex[keyPath] == nil {
		f.InvertedIndex[keyPath] = make(map[Value][]wikipage.PageIdentifier)
	}
	f.InvertedIndex[keyPath][value] = append(f.InvertedIndex[keyPath][value], identifier)

	if f.PageKeyMap[identifier] == nil {
		f.PageKeyMap[identifier] = make(map[DottedKeyPath]map[Value]bool)
	}
	if f.PageKeyMap[identifier][keyPath] == nil {
		f.PageKeyMap[identifier][keyPath] = make(map[Value]bool)
	}
	f.PageKeyMap[identifier][keyPath][value] = true
}

func (f *Index) recursiveAddFrontmatter(identifier wikipage.PageIdentifier, keyPath string, value any) error {
	if keyPath == "identifier" {
		return nil
	}

	// recursively build the dotted key path. a value in frontmatter can be either a string or a map[string]any
	// if it is a map[string]any, then we need to recurse
	switch v := value.(type) {
	case map[string]any:
		for key, val := range v {
			newKeyPath := keyPath
			if newKeyPath != "" {
				newKeyPath += "."
			}
			newKeyPath += key

			if _, isMap := val.(map[string]any); isMap {
				f.saveToIndex(identifier, newKeyPath, "") // This ensures that the QueryKeyExistence function works for all keys in the hierarchy
			}
			if err := f.recursiveAddFrontmatter(identifier, newKeyPath, val); err != nil {
				return err
			}
		}
	case string:
		f.saveToIndex(identifier, keyPath, v)
	case []any:
		// Save empty string to ensure QueryKeyExistence works for empty arrays
		if len(v) == 0 {
			f.saveToIndex(identifier, keyPath, "")
		}
		for _, array := range v {
			switch str := array.(type) {
			case string:
				f.saveToIndex(identifier, keyPath, str)
			default:
				return fmt.Errorf("frontmatter indexer: invalid array element type for page %q key %q (type: %T)", identifier, keyPath, array)
			}
		}
	case bool:
		// Only index true values - we rely on key existence for filtering containers
		if v {
			f.saveToIndex(identifier, keyPath, "true")
		}
	default:
		return fmt.Errorf("frontmatter indexer: invalid value type for page %q key %q (type: %T)", identifier, keyPath, v)
	}
	return nil
}

// RemovePageFromIndex removes a page's frontmatter from the index.
func (f *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removePageFromIndexInternal(identifier)
	return nil
}

func (f *Index) removePageFromIndexInternal(identifier wikipage.PageIdentifier) {
	identifier = wikiidentifiers.MungeIdentifier(identifier)
	for dottedKeyPath := range f.PageKeyMap[identifier] {
		for value := range f.PageKeyMap[identifier][dottedKeyPath] {
			identifiers := f.InvertedIndex[dottedKeyPath][value]
			identifiers = slices.DeleteFunc(identifiers, func(v string) bool { return v == identifier })
			f.InvertedIndex[dottedKeyPath][value] = identifiers
		}
	}
	delete(f.PageKeyMap, identifier)
}

// QueryExactMatch queries the index for exact matches of a key-value pair.
func (f *Index) QueryExactMatch(dottedKeyPath DottedKeyPath, value Value) []wikipage.PageIdentifier {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.InvertedIndex[dottedKeyPath][value]
}

// QueryKeyExistence queries the index for the existence of a key.
func (f *Index) QueryKeyExistence(dottedKeyPath DottedKeyPath) []wikipage.PageIdentifier {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var identifiersWithKey []wikipage.PageIdentifier
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		identifiersWithKey = append(identifiersWithKey, f.InvertedIndex[dottedKeyPath][indexedValue]...)
	}
	return identifiersWithKey
}

// QueryPrefixMatch queries the index for keys with a given prefix.
func (f *Index) QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var identifiersWithKey []wikipage.PageIdentifier
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		if strings.HasPrefix(indexedValue, valuePrefix) {
			identifiersWithKey = append(identifiersWithKey, f.InvertedIndex[dottedKeyPath][indexedValue]...)
		}
	}
	return identifiersWithKey
}

// GetValue retrieves the value of a frontmatter key for a given page.
func (f *Index) GetValue(identifier wikipage.PageIdentifier, dottedKeyPath DottedKeyPath) Value {
	f.mu.RLock()
	defer f.mu.RUnlock()
	identifier = wikiidentifiers.MungeIdentifier(identifier)
	for value := range f.PageKeyMap[identifier][dottedKeyPath] {
		return value
	}
	return ""
}
