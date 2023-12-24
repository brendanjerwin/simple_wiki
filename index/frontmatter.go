package index

import (
	"log"
	"slices"
	"strings"

	"github.com/brendanjerwin/simple_wiki/common"
)

type DottedKeyPath = string
type Value = string
type PageIdentifier = string

type IMaintainFrontmatterIndex interface {
	AddFrontmatterToIndex(identifier PageIdentifier, frontmatter common.FrontMatter)
	RemoveFrontmatterFromIndex(identifier PageIdentifier)
}

type IQueryFrontmatterIndex interface {
	QueryExactMatch(dotted_key_path DottedKeyPath, value Value) []PageIdentifier
	QueryKeyExistence(dotted_key_path DottedKeyPath) []PageIdentifier
	QueryPrefixMatch(dotted_key_path DottedKeyPath, value_prefix string) []PageIdentifier

	GetValue(identifier PageIdentifier, dotted_key_path DottedKeyPath) Value
}

type FrontmatterIndex struct {
	InvertedIndex map[DottedKeyPath]map[Value][]PageIdentifier
	PageKeyMap    map[PageIdentifier]map[DottedKeyPath]map[Value]bool
}

func NewFrontmatterIndex() *FrontmatterIndex {
	return &FrontmatterIndex{
		InvertedIndex: make(map[DottedKeyPath]map[Value][]PageIdentifier),
		PageKeyMap:    make(map[PageIdentifier]map[DottedKeyPath]map[Value]bool),
	}
}

func (f *FrontmatterIndex) AddFrontmatterToIndex(identifier PageIdentifier, frontmatter common.FrontMatter) {
	f.recursiveAdd(identifier, "", frontmatter)
}

func (f *FrontmatterIndex) saveToIndex(identifier PageIdentifier, keyPath DottedKeyPath, value Value) {
	if f.InvertedIndex[keyPath] == nil {
		f.InvertedIndex[keyPath] = make(map[Value][]PageIdentifier)
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

func (f *FrontmatterIndex) recursiveAdd(identifier PageIdentifier, keyPath string, value interface{}) {
	if keyPath == "identifier" {
		return
	}

	identifier = common.MungeIdentifier(identifier)
	//recursively build the dotted key path. a value in frontmatter can be either a string or a map[string]interface{}
	//if it is a map[string]interface{}, then we need to recurse
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			newKeyPath := keyPath
			if newKeyPath != "" {
				newKeyPath += "."
			}
			newKeyPath += key

			if _, isMap := val.(map[string]interface{}); isMap {
				f.saveToIndex(identifier, newKeyPath, "") //This ensures that the QueryKeyExistence function works for all keys in the hierarchy
			}
			f.recursiveAdd(identifier, newKeyPath, val)
		}
	case string:
		f.saveToIndex(identifier, keyPath, v)
	case []interface{}:
		for _, array := range v {
			switch str := array.(type) {
			case string:
				f.saveToIndex(identifier, keyPath, str)
			default:
				log.Printf("frontmatter can only be a string, []string, or a map[string]interface{}. Page: %v Key: %v", identifier, keyPath)
			}
		}
	default:
		log.Printf("frontmatter can only be a string, []string, or a map[string]interface{}. Page: %v Key: %v", identifier, keyPath)
	}
}

func (f *FrontmatterIndex) RemoveFrontmatterFromIndex(identifier PageIdentifier) {
	identifier = common.MungeIdentifier(identifier)
	for dotted_key_path := range f.PageKeyMap[identifier] {
		for value := range f.PageKeyMap[identifier][dotted_key_path] {
			identifiers := f.InvertedIndex[dotted_key_path][value]
			identifiers = slices.DeleteFunc(identifiers, func(v string) bool { return v == identifier })
			f.InvertedIndex[dotted_key_path][value] = identifiers
		}
	}
	delete(f.PageKeyMap, identifier)
}

func (f *FrontmatterIndex) QueryExactMatch(dotted_key_path DottedKeyPath, value Value) []PageIdentifier {
	return f.InvertedIndex[dotted_key_path][value]
}

func (f *FrontmatterIndex) QueryKeyExistence(dotted_key_path DottedKeyPath) []PageIdentifier {
	var identifiers_with_key []PageIdentifier
	for indexed_value := range f.InvertedIndex[dotted_key_path] {
		identifiers_with_key = append(identifiers_with_key, f.InvertedIndex[dotted_key_path][indexed_value]...)
	}
	return identifiers_with_key
}

func (f *FrontmatterIndex) QueryPrefixMatch(dotted_key_path DottedKeyPath, value_prefix string) []PageIdentifier {
	var identifiers_with_key []PageIdentifier
	for indexed_value := range f.InvertedIndex[dotted_key_path] {
		if strings.HasPrefix(indexed_value, value_prefix) {
			identifiers_with_key = append(identifiers_with_key, f.InvertedIndex[dotted_key_path][indexed_value]...)
		}
	}
	return identifiers_with_key
}

func (f *FrontmatterIndex) GetValue(identifier PageIdentifier, dotted_key_path DottedKeyPath) Value {
	identifier = common.MungeIdentifier(identifier)
	for value := range f.PageKeyMap[identifier][dotted_key_path] {
		return value
	}
	return ""
}
