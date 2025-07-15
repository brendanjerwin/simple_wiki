package frontmatter

import (
	"log"
	"slices"
	"strings"

	"github.com/brendanjerwin/simple_wiki/common"
)

type (
	DottedKeyPath = string
	Value         = string
)

type IQueryFrontmatterIndex interface {
	QueryExactMatch(dotted_key_path DottedKeyPath, value Value) []common.PageIdentifier
	QueryKeyExistence(dotted_key_path DottedKeyPath) []common.PageIdentifier
	QueryPrefixMatch(dotted_key_path DottedKeyPath, value_prefix string) []common.PageIdentifier

	GetValue(identifier common.PageIdentifier, dotted_key_path DottedKeyPath) Value
}

type FrontmatterIndex struct {
	InvertedIndex map[DottedKeyPath]map[Value][]common.PageIdentifier
	PageKeyMap    map[common.PageIdentifier]map[DottedKeyPath]map[Value]bool
	pageReader    common.PageReader
}

func NewFrontmatterIndex(pageReader common.PageReader) *FrontmatterIndex {
	return &FrontmatterIndex{
		InvertedIndex: make(map[DottedKeyPath]map[Value][]common.PageIdentifier),
		PageKeyMap:    make(map[common.PageIdentifier]map[DottedKeyPath]map[Value]bool),
		pageReader:    pageReader,
	}
}

func (f *FrontmatterIndex) AddPageToIndex(requested_identifier common.PageIdentifier) error {
	munged_identifier := common.MungeIdentifier(requested_identifier)
	identifier, frontmatter, err := f.pageReader.ReadFrontMatter(requested_identifier)
	if err != nil {
		return err
	}

	f.RemovePageFromIndex(identifier)
	f.RemovePageFromIndex(requested_identifier)
	f.RemovePageFromIndex(munged_identifier)

	f.recursiveAdd(munged_identifier, "", frontmatter)
	return nil
}

func (f *FrontmatterIndex) saveToIndex(identifier common.PageIdentifier, keyPath DottedKeyPath, value Value) {
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

func (f *FrontmatterIndex) recursiveAdd(identifier common.PageIdentifier, keyPath string, value any) {
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

func (f *FrontmatterIndex) RemovePageFromIndex(identifier common.PageIdentifier) error {
	identifier = common.MungeIdentifier(identifier)
	for dotted_key_path := range f.PageKeyMap[identifier] {
		for value := range f.PageKeyMap[identifier][dotted_key_path] {
			identifiers := f.InvertedIndex[dotted_key_path][value]
			identifiers = slices.DeleteFunc(identifiers, func(v string) bool { return v == identifier })
			f.InvertedIndex[dotted_key_path][value] = identifiers
		}
	}
	delete(f.PageKeyMap, identifier)
	return nil
}

func (f *FrontmatterIndex) QueryExactMatch(dotted_key_path DottedKeyPath, value Value) []common.PageIdentifier {
	return f.InvertedIndex[dotted_key_path][value]
}

func (f *FrontmatterIndex) QueryKeyExistence(dotted_key_path DottedKeyPath) []common.PageIdentifier {
	var identifiers_with_key []common.PageIdentifier
	for indexed_value := range f.InvertedIndex[dotted_key_path] {
		identifiers_with_key = append(identifiers_with_key, f.InvertedIndex[dotted_key_path][indexed_value]...)
	}
	return identifiers_with_key
}

func (f *FrontmatterIndex) QueryPrefixMatch(dotted_key_path DottedKeyPath, value_prefix string) []common.PageIdentifier {
	var identifiers_with_key []common.PageIdentifier
	for indexed_value := range f.InvertedIndex[dotted_key_path] {
		if strings.HasPrefix(indexed_value, value_prefix) {
			identifiers_with_key = append(identifiers_with_key, f.InvertedIndex[dotted_key_path][indexed_value]...)
		}
	}
	return identifiers_with_key
}

func (f *FrontmatterIndex) GetValue(identifier common.PageIdentifier, dotted_key_path DottedKeyPath) Value {
	identifier = common.MungeIdentifier(identifier)
	for value := range f.PageKeyMap[identifier][dotted_key_path] {
		return value
	}
	return ""
}
