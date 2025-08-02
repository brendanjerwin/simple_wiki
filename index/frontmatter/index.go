// Package frontmatter provides functionality for indexing and querying page frontmatter.
package frontmatter

import (
	"fmt"
	"log"
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

const (
	// MaxRecursionDepth is the maximum allowed recursion depth to prevent stack overflow
	MaxRecursionDepth = 50
)

// recursionContext tracks visited paths and recursion depth to prevent infinite loops
type recursionContext struct {
	visitedObjects map[string]bool // Track object pointers to detect cycles
	depth          int             // Current recursion depth
	maxDepth       int             // Maximum allowed depth
}

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

// GetIndexName returns the name of this index for progress tracking.
func (*Index) GetIndexName() string {
	return "frontmatter"
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

	f.removePageFromIndexUnsafe(identifier)
	f.removePageFromIndexUnsafe(requestedIdentifier)
	f.removePageFromIndexUnsafe(mungedIdentifier)

	// Initialize recursion context for loop detection
	ctx := &recursionContext{
		visitedObjects: make(map[string]bool),
		depth:          0,
		maxDepth:       MaxRecursionDepth,
	}
	
	f.recursiveAddWithContext(mungedIdentifier, "", frontmatter, ctx)
	return nil
}

func (f *Index) saveToIndexUnsafe(identifier wikipage.PageIdentifier, keyPath DottedKeyPath, value Value) {
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

func (f *Index) recursiveAddWithContext(identifier wikipage.PageIdentifier, keyPath string, value any, ctx *recursionContext) {
	if keyPath == "identifier" {
		return
	}

	// Check recursion depth limit
	if ctx.depth > ctx.maxDepth {
		log.Printf("Maximum recursion depth (%d) exceeded for page %v at path %v - stopping to prevent stack overflow", 
			ctx.maxDepth, identifier, keyPath)
		return
	}

	// Increment depth for this recursion level
	ctx.depth++
	defer func() { ctx.depth-- }()

	// recursively build the dotted key path. a value in frontmatter can be either a string or a map[string]any
	// if it is a map[string]any, then we need to recurse
	switch v := value.(type) {
	case map[string]any:
		// Generate unique key for this object to detect cycles
		objPtr := fmt.Sprintf("%p", v)
		
		// Check if we've already visited this object anywhere in the current path
		if ctx.visitedObjects[objPtr] {
			log.Printf("Circular reference detected for page %v: object %s already visited (now at path %v) - skipping to prevent infinite loop", 
				identifier, objPtr, keyPath)
			return
		}
		
		// Mark this object as visited for the duration of this subtree traversal
		ctx.visitedObjects[objPtr] = true
		defer func() { delete(ctx.visitedObjects, objPtr) }()

		for key, val := range v {
			newKeyPath := keyPath
			if newKeyPath != "" {
				newKeyPath += "."
			}
			newKeyPath += key

			if _, isMap := val.(map[string]any); isMap {
				f.saveToIndexUnsafe(identifier, newKeyPath, "") // This ensures that the QueryKeyExistence function works for all keys in the hierarchy
			}
			f.recursiveAddWithContext(identifier, newKeyPath, val, ctx)
		}
	case string:
		f.saveToIndexUnsafe(identifier, keyPath, v)
	case []any:
		for _, array := range v {
			switch str := array.(type) {
			case string:
				f.saveToIndexUnsafe(identifier, keyPath, str)
			default:
				log.Printf("frontmatter can only be a string, []string, or a map[string]any. Page: %v Key: %v (type: %T)", identifier, keyPath, array)
			}
		}
	default:
		log.Printf("frontmatter can only be a string, []string, or a map[string]any. Page: %v Key: %v (type: %T)", identifier, keyPath, v)
	}
}

// RemovePageFromIndex removes a page's frontmatter from the index.
func (f *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removePageFromIndexUnsafe(identifier)
	return nil
}

func (f *Index) removePageFromIndexUnsafe(identifier wikipage.PageIdentifier) {
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
	
	// Use a map to deduplicate identifiers
	identifierSet := make(map[wikipage.PageIdentifier]bool)
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		for _, identifier := range f.InvertedIndex[dottedKeyPath][indexedValue] {
			identifierSet[identifier] = true
		}
	}
	
	// Convert map to slice
	var identifiersWithKey []wikipage.PageIdentifier
	for identifier := range identifierSet {
		identifiersWithKey = append(identifiersWithKey, identifier)
	}
	return identifiersWithKey
}

// QueryPrefixMatch queries the index for keys with a given prefix.
func (f *Index) QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Use a map to deduplicate identifiers
	identifierSet := make(map[wikipage.PageIdentifier]bool)
	for indexedValue := range f.InvertedIndex[dottedKeyPath] {
		if strings.HasPrefix(indexedValue, valuePrefix) {
			for _, identifier := range f.InvertedIndex[dottedKeyPath][indexedValue] {
				identifierSet[identifier] = true
			}
		}
	}
	
	// Convert map to slice
	var identifiersWithKey []wikipage.PageIdentifier
	for identifier := range identifierSet {
		identifiersWithKey = append(identifiersWithKey, identifier)
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
