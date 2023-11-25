package utils

import (
	"reflect"
	"strings"
	"testing"
)

type MockReadFrontMatter struct {
	Frontmatter map[string]interface{}
}

func (m *MockReadFrontMatter) ReadFrontMatter(markdown string) (map[string]interface{}, error) {
	return m.Frontmatter, nil
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

	html, _ := MarkdownToHtmlAndJsonFrontmatter(markdown, true, &MockReadFrontMatter{})

	if strings.Contains(string(html), "sample:") {
		t.Errorf("Did not remove frontmatter.")
	}

	if !strings.Contains(string(html), "<h1>Hello</h1") {
		t.Errorf("Did not include HTML")
	}
}

func TestExecuteTemplate(t *testing.T) {

	frontmatter := `
{
"identifier": "1234"
}
	`

	templateHtml := `
{{ .Identifier }}
	`

	rendered, err := ExecuteTemplate(templateHtml, []byte(frontmatter), &MockReadFrontMatter{})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(rendered), "1234") {
		t.Error("Did not render identifer into output")
	}
}

func TestExecuteTemplateUnstructured(t *testing.T) {

	frontmatter := `
{
"identifier": "1234",
"foobar": "baz"
}
	`

	templateHtml := `
{{ index .Map "foobar" }}
	`

	rendered, err := ExecuteTemplate(templateHtml, []byte(frontmatter), &MockReadFrontMatter{})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(rendered), "baz") {
		t.Error("Did not render data into output")
	}
}
func TestRandomStringOfLength(t *testing.T) {
	tests := []struct {
		length int
	}{
		{length: 5},
		{length: 8},
		{length: 10},
		{length: 15},
	}

	for _, tt := range tests {
		result, _ := RandomStringOfLength(tt.length)
		result2, err := RandomStringOfLength(tt.length)
		if err != nil {
			t.Errorf("Error generating random string: %s", err)
		}
		if len(result) != tt.length {
			t.Errorf("Expected string of length %d, but got length %d", tt.length, len(result))
		}
		if result == result2 {
			t.Errorf("Expected two different strings, but got the same: %s", result)
		}
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
func TestMarkdownToHTML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "Test with simple markdown",
			input:    "**bold**",
			expected: []byte("<p><strong>bold</strong></p>\n"),
		},
		{
			name:     "Test with markdown link",
			input:    "[link](http://example.com)",
			expected: []byte("<p><a href=\"http://example.com\" rel=\"nofollow\">link</a></p>\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MarkdownToHTML(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected: %s, got: %s", tc.expected, result)
			}
		})
	}
}
