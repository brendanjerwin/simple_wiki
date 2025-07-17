// Package frontmatter provides functionality for indexing and querying page frontmatter.
package frontmatter

import (
	"log"
	"slices"
	"strings"

	"github.com/brendanjerwin/simple_wiki/common"
)

type (
	// DottedKeyPath represents a dot-separated path to a frontmatter key.
	DottedKeyPath = string
	// Value represents the value of a frontmatter key.
	Value = string
)

// IQueryFrontmatterIndex defines the interface for querying the frontmatter index.
type IQueryFrontmatterIndex interface {
	QueryExactMatch(dottedKeyPath DottedKeyPath, value Value) []common.PageIdentifier
	QueryKeyExistence(dottedKeyPath DottedKeyPath) []common.PageIdentifier
	QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []common.PageIdentifier

	GetValue(identifier common.PageIdentifier, dottedKeyPath DottedKeyPath) Value
}

// Index is a struct that maintains an inverted index of frontmatter keys and values.
type Index struct {
	InvertedIndex map[DottedKeyPath]map[Value][]common.PageIdentifier
	PageKeyMap    map[common.PageIdentifier]map[DottedKeyPath]map[Value]bool
	pageReader    common.PageReader
}

// NewIndex creates a new FrontmatterIndex.
func NewIndex(pageReader common.PageReader) *Index {
	return &Index{
		InvertedIndex: make(map[DottedKeyPath]map[Value][]common.PageIdentifier),
		PageKeyMap:    make(map[common.PageIdentifier]map[DottedKeyPath]map[Value]bool),
		pageReader:    pageReader,
	}
}

// AddPageToIndex adds a page's frontmatter to the index.
func (f *Index) AddPageToIndex(requestedIdentifier common.PageIdentifier) error {
	mungedIdentifier := common.MungeIdentifier(requestedIdentifier)
	identifier, frontmatter, err := f.pageReader.ReadFrontMatter(requestedIdentifier)
	if err != nil {
		return err
	}

	_ = f.RemovePageFromIndex(identifier)
	_ = f.RemovePageFromIndex(requestedIdentifier)
	_ = f.RemovePageFromIndex(mungedIdentifier)

	f.recursiveAdd(mungedIdentifier, "", frontmatter)
	return nil
}

func (f *Index) saveToIndex(identifier common.PageIdentifier, keyPath DottedKeyPath, value Value) {
	if f.InvertedIndex[keyPath] == nil {
		f.InvertedIndex[keyPath] = make(map[Value][]common.PageIdentifier)
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

func (f *Index) recursiveAdd(identifier common.PageIdentifier, keyPath string, value any) {
	if keyPath == "identifier" {
		return
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
			f.recursiveAdd(identifier, newKeyPath, val)
		}
	case string:
		f.saveToIndex(identifier, keyPath, v)
	case []any:
		for _, array := range v {
			switch str := array.(type) {
			case string:
				f.saveToIndex(identifier, keyPath, str)
			default:
				log.Printf("frontmatter can only be a string, []string, or a map[string]any. Page: %v Key: %v (type: %T)", identifier, keyPath, array)
			}
		}
	default:
		log.Printf("frontmatter can only be a string, []string, or a map[string]any. Page: %v Key: %v (type: %T)", identifier, keyPath, v)
	}
}

// RemovePageFromIndex removes a page's frontmatter from the index.
func (f *Index) RemovePageFromIndex(identifier common.PageIdentifier) error {
	identifier = common.MungeIdentifier(identifier)
	for dottedKeyPath := range f.PageKeyMap[identifier] {
		for value := range f.PageKeyMap[identifier][dottedKeyPath] {
			identifiers := f.InvertedIndex[dottedKeyPath][value]
			identifiers = slices.DeleteFunc(identifiers, func(v string) bool { return v == identifier })
			f.InvertedIndex[dottedKeyPath][value] = identifiers
		}
	}
	delete(f.PageKeyMap, identifier)
	return nil
}

// QueryExactMatch queries the index for exact matches of a key-value pair.
func (f *Index) QueryExactMatch(dottedKeyPath DottedKeyPath, value Value) []common.PageIdentifier {
	return f.InvertedIndex[dottedKeyPath][value]
}

// QueryKeyExistence queries the index for the existence of a key.
func (f *Index) QueryKeyExistence(dottedKeyPath DottedKeyPath) []common.PageIdentifier {
	var identifiersWithKey []common.PageIdentifier
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		identifiersWithKey = append(identifiersWithKey, f.InvertedIndex[dottedKeyPath][indexedValue]...)
	}
	return identifiersWithKey
}

// QueryPrefixMatch queries the index for keys with a given prefix.
func (f *Index) QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []common.PageIdentifier {
	var identifiersWithKey []common.PageIdentifier
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		if strings.HasPrefix(indexedValue, valuePrefix) {
			identifiersWithKey = append(identifiersWithKey, f.InvertedIndex[dottedKeyPath][indexedValue]...)
		}
	}
	return identifiersWithKey
}

// GetValue retrieves the value of a frontmatter key for a given page.
func (f *Index) GetValue(identifier common.PageIdentifier, dottedKeyPath DottedKeyPath) Value {
	identifier = common.MungeIdentifier(identifier)
	for value := range f.PageKeyMap[identifier][dottedKeyPath] {
		return value
	}
	return ""
}
