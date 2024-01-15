package utils

import (
	"strings"
	"testing"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/templating"
)

type MockReadPages struct {
	Markdown    string
	Frontmatter common.FrontMatter
}

func (m *MockReadPages) ReadMarkdown(identifier string) (string, string, error) {
	return identifier, m.Markdown, nil
}

func (m *MockReadPages) ReadFrontMatter(identifier string) (string, common.FrontMatter, error) {
	return identifier, m.Frontmatter, nil
}

// Mocks index.IQueryFrontmatterIndex
type MockQueryFrontmatterIndex struct {
	Results map[string][]string
}

func (m *MockQueryFrontmatterIndex) QueryExactMatch(keyPath string, value string) []string {
	return m.Results[keyPath]
}

func (m *MockQueryFrontmatterIndex) QueryKeyExistence(keyPath string) []string {
	return m.Results[keyPath]
}

func (m *MockQueryFrontmatterIndex) QueryPrefixMatch(keyPath string, valuePrefix string) []string {
	return m.Results[keyPath]
}

func (m *MockQueryFrontmatterIndex) GetValue(identifier string, keyPath string) string {
	return ""
}

func BenchmarkAlliterativeAnimal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RandomAlliterateCombo()
	}
}

func TestReverseList(t *testing.T) {
	s := []int64{1, 10, 2, 20}
	if ReverseSliceInt64(s)[0] != 20 {
		t.Errorf("Could not reverse: %v", s)
	}
	s2 := []string{"a", "b", "d", "c"}
	if ReverseSliceString(s2)[0] != "c" {
		t.Errorf("Could not reverse: %v", s2)
	}
}

func TestMarkdownToHtmlWithFrontmatter(t *testing.T) {
	markdown := `
---
sample: "value"
---

# Hello
	`

	//(s string, handleFrontMatter bool, site common.IReadPages, renderer IRenderMarkdownToHtml, query frontmatterIdx.IQueryFrontmatterIndex)
	html, _, _ := MarkdownToHtmlAndJsonFrontmatter(markdown, true, &MockReadPages{}, &GoldmarkRenderer{}, &MockQueryFrontmatterIndex{})

	if strings.Contains(string(html), "sample:") {
		t.Errorf("Did not remove frontmatter.")
	}

	if !strings.Contains(string(html), ">Hello</h1") {
		t.Errorf("Did not include HTML")
	}
}

func TestExecuteTemplate(t *testing.T) {

	frontmatter := common.FrontMatter{"identifier": "1234"}

	templateHtml := `
{{ .Identifier }}
	`

	rendered, err := templating.ExecuteTemplate(templateHtml, frontmatter, &MockReadPages{}, &MockQueryFrontmatterIndex{})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(rendered), "1234") {
		t.Error("Did not render identifer into output")
	}
}

func TestExecuteTemplateUnstructured(t *testing.T) {

	frontmatter := common.FrontMatter{"identifier": "1234", "foobar": "baz"}

	templateHtml := `
{{ index .Map "foobar" }}
	`

	rendered, err := templating.ExecuteTemplate(templateHtml, frontmatter, &MockReadPages{}, &MockQueryFrontmatterIndex{})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(rendered), "baz") {
		t.Error("Did not render data into output")
	}
}
func TestStripFrontmatter(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Test with frontmatter",
			input:    "---\ntitle: Test\n---\nThis is a test",
			expected: "This is a test",
		},
		{
			name:     "Test without frontmatter",
			input:    "This is a test",
			expected: "This is a test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := StripFrontmatter(tc.input)
			if result != tc.expected {
				t.Errorf("expected: %s, got: %s", tc.expected, result)
			}
		})
	}
}

func TestRandomAlliterateCombo(t *testing.T) {
	combo := RandomAlliterateCombo()
	if len(combo) == 0 {
		t.Errorf("Expected a non-empty string, got an empty string")
	}
}

func TestStringInSlice(t *testing.T) {
	strings := []string{"apple", "banana", "cherry"}
	if !StringInSlice("banana", strings) {
		t.Errorf("Expected 'banana' to be in the slice")
	}
	if StringInSlice("orange", strings) {
		t.Errorf("Did not expect 'orange' to be in the slice")
	}
}

func TestContentTypeFromName(t *testing.T) {
	if ContentTypeFromName("file.md") != "text/markdown; charset=utf-8" {
		t.Errorf("Expected 'text/markdown', got '%s'", ContentTypeFromName("file.md"))
	}
	if ContentTypeFromName("image.heic") != "image/heic" {
		t.Errorf("Expected 'image/heic', got '%s'", ContentTypeFromName("image.heic"))
	}
}

func TestRandomStringOfLength(t *testing.T) {
	str, err := RandomStringOfLength(10)
	if err != nil {
		t.Errorf("Expected no error, got '%s'", err)
	}
	if len(str) != 10 {
		t.Errorf("Expected length 10, got '%d'", len(str))
	}
}

func TestExists(t *testing.T) {
	if !Exists("./utils.go") {
		t.Errorf("Expected './utils.go' to exist")
	}
	if Exists("./nonexistent_file.go") {
		t.Errorf("Did not expect './nonexistent_file.go' to exist")
	}
}

func TestEncodeToBase32(t *testing.T) {
	if EncodeToBase32("hello") != "NBSWY3DP" {
		t.Error("Expected 'NBSWY3DP", EncodeToBase32("hello"))
	}
}

func TestDecodeFromBase32(t *testing.T) {
	str, err := DecodeFromBase32("NBSWY3DP")
	if err != nil {
		t.Errorf("Expected no error, got '%s'", err)
	}
	if str != "hello" {
		t.Errorf("Expected 'hello', got '%s'", str)
	}
}

func TestReverseSliceInt64(t *testing.T) {
	slice := []int64{1, 2, 3, 4, 5}
	reversed := ReverseSliceInt64(slice)
	expected := []int64{5, 4, 3, 2, 1}
	for i, v := range reversed {
		if v != expected[i] {
			t.Errorf("Expected '%d', got '%d'", expected[i], v)
		}
	}
}

func TestReverseSliceString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}
	reversed := ReverseSliceString(slice)
	expected := []string{"cherry", "banana", "apple"}
	for i, v := range reversed {
		if v != expected[i] {
			t.Errorf("Expected '%s', got '%s'", expected[i], v)
		}
	}
}

func TestReverseSliceInt(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	reversed := ReverseSliceInt(slice)
	expected := []int{5, 4, 3, 2, 1}
	for i, v := range reversed {
		if v != expected[i] {
			t.Errorf("Expected '%d', got '%d'", expected[i], v)
		}
	}
}
