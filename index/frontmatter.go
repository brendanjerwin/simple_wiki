package index

import "github.com/brendanjerwin/simple_wiki/common"

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
}

type FrontmatterIndex struct {
	InvertedIndex map[DottedKeyPath]map[Value][]PageIdentifier
	PageKeyMap    map[PageIdentifier]map[DottedKeyPath]bool
}

func NewFrontmatterIndex() *FrontmatterIndex {
	return &FrontmatterIndex{
		InvertedIndex: make(map[DottedKeyPath]map[Value][]PageIdentifier),
		PageKeyMap:    make(map[PageIdentifier]map[DottedKeyPath]bool),
	}
}

func (f *FrontmatterIndex) AddFrontmatterToIndex(identifier PageIdentifier, frontmatter common.FrontMatter) {
	f.recursiveAdd("", identifier, frontmatter)
}

func (f *FrontmatterIndex) recursiveAdd(keyPath string, identifier PageIdentifier, value interface{}) {
	if keyPath == "identifier" {
		return
	}

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
			f.recursiveAdd(newKeyPath, identifier, val)
		}
	case string:
		if f.InvertedIndex[keyPath] == nil {
			f.InvertedIndex[keyPath] = make(map[Value][]PageIdentifier)
		}
		if f.InvertedIndex[keyPath][v] == nil {
			f.InvertedIndex[keyPath][v] = make([]PageIdentifier, 0)
		}

		f.InvertedIndex[keyPath][v] = append(f.InvertedIndex[keyPath][v], identifier)

		if f.PageKeyMap[identifier] == nil {
			f.PageKeyMap[identifier] = make(map[DottedKeyPath]bool)
		}
		f.PageKeyMap[identifier][keyPath] = true

	case []interface{}:
		for _, array := range v {
			switch str := array.(type) {
			case string:
				if f.InvertedIndex[keyPath] == nil {
					f.InvertedIndex[keyPath] = make(map[Value][]PageIdentifier)
				}
				if f.InvertedIndex[keyPath][str] == nil {
					f.InvertedIndex[keyPath][str] = make([]PageIdentifier, 0)
				}

				f.InvertedIndex[keyPath][str] = append(f.InvertedIndex[keyPath][str], identifier)

				if f.PageKeyMap[identifier] == nil {
					f.PageKeyMap[identifier] = make(map[DottedKeyPath]bool)
				}
				f.PageKeyMap[identifier][keyPath] = true
			default:
				panic("frontmatter can only be a string, []string, or a map[string]interface{}")
			}
		}
	default:
		panic("frontmatter can only be a string, []string, or a map[string]interface{}")
	}
}

func (f *FrontmatterIndex) RemoveFrontmatterFromIndex(identifier PageIdentifier) {
	for key := range f.PageKeyMap[identifier] {
		delete(f.InvertedIndex[key], identifier)
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
		if indexed_value[:len(value_prefix)] == value_prefix {
			identifiers_with_key = append(identifiers_with_key, f.InvertedIndex[dotted_key_path][indexed_value]...)
		}
	}
	return identifiers_with_key
}
