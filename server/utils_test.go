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

	html := MarkdownToHtml(markdown, true)

	if strings.Contains(html, "sample:") {
		t.Errorf("Did not remove frontmatter.")
	}

	if !strings.Contains(html, "<h1>Hello</h1") {
		t.Errorf("Did not include HTML")
	}
}
