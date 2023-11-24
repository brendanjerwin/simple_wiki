package server

import (
	"strings"
	"testing"
)

func BenchmarkAlliterativeAnimal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomAlliterateCombo()
	}
}

func TestReverseList(t *testing.T) {
	s := []int64{1, 10, 2, 20}
	if reverseSliceInt64(s)[0] != 20 {
		t.Errorf("Could not reverse: %v", s)
	}
	s2 := []string{"a", "b", "d", "c"}
	if reverseSliceString(s2)[0] != "c" {
		t.Errorf("Could not reverse: %v", s2)
	}
}

func TestHashing(t *testing.T) {
	p := HashPassword("1234")
	err := CheckPasswordHash("1234", p)
	if err != nil {
		t.Errorf("Should be correct password")
	}
	err = CheckPasswordHash("1234lkjklj", p)
	if err == nil {
		t.Errorf("Should NOT be correct password")
	}
}

func TestMarkdownToHtmlWithFrontmatter(t *testing.T) {
	markdown := `
---
sample: "value"
---

# Hello
	`

	html, _ := MarkdownToHtmlAndJsonFrontmatter(markdown, true, &Site{})

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

	rendered, err := ExecuteTemplate(templateHtml, []byte(frontmatter), &Site{})

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

	rendered, err := ExecuteTemplate(templateHtml, []byte(frontmatter), &Site{})

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
