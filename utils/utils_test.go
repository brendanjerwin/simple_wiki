package utils_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

type MockReadFrontMatter struct {
	Frontmatter wikipage.FrontMatter
	Markdown    wikipage.Markdown
}

func (m *MockReadFrontMatter) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return identifier, m.Frontmatter, nil
}

func (m *MockReadFrontMatter) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return identifier, m.Markdown, nil
}

// Mocks index.IQueryFrontmatterIndex
type MockQueryFrontmatterIndex struct {
	Results        map[string][]wikipage.PageIdentifier
	GetValueResult frontmatter.Value
}

func (m *MockQueryFrontmatterIndex) QueryExactMatch(keyPath frontmatter.DottedKeyPath, value frontmatter.Value) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) QueryKeyExistence(keyPath frontmatter.DottedKeyPath) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) QueryPrefixMatch(keyPath frontmatter.DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) GetValue(identifier wikipage.PageIdentifier, keyPath frontmatter.DottedKeyPath) frontmatter.Value {
	return m.GetValueResult
}

var _ = Describe("Utils", func() {
	Describe("ReverseSlice", func() {
		Describe("ReverseSliceInt64", func() {
			It("should reverse a slice of int64", func() {
				slice := []int64{1, 2, 3, 4, 5}
				reversed := utils.ReverseSliceInt64(slice)
				Expect(reversed).To(Equal([]int64{5, 4, 3, 2, 1}))
			})
		})

		Describe("ReverseSliceString", func() {
			It("should reverse a slice of strings", func() {
				slice := []string{"apple", "banana", "cherry"}
				reversed := utils.ReverseSliceString(slice)
				Expect(reversed).To(Equal([]string{"cherry", "banana", "apple"}))
			})
		})

		Describe("ReverseSliceInt", func() {
			It("should reverse a slice of int", func() {
				slice := []int{1, 2, 3, 4, 5}
				reversed := utils.ReverseSliceInt(slice)
				Expect(reversed).To(Equal([]int{5, 4, 3, 2, 1}))
			})
		})
	})

	Describe("utils.MarkdownToHTMLAndJSONFrontmatter", func() {
		var (
			markdown string
			html     []byte
		)

		BeforeEach(func() {
			markdown = `
---
sample: "value"
---

# Hello
	`
			var err error
			html, _, err = utils.MarkdownToHTMLAndJSONFrontmatter(markdown, &MockReadFrontMatter{}, &utils.GoldmarkRenderer{}, &MockQueryFrontmatterIndex{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove frontmatter from the output", func() {
			Expect(string(html)).NotTo(ContainSubstring("sample:"))
		})

		It("should render the markdown to HTML", func() {
			Expect(string(html)).To(ContainSubstring(">Hello</h1"))
		})
	})

	Describe("templating.ExecuteTemplate", func() {
		var (
			theFrontmatter wikipage.FrontMatter
			rendered       []byte
			err            error
		)
		var site wikipage.PageReader = &MockReadFrontMatter{}
		var query frontmatter.IQueryFrontmatterIndex = &MockQueryFrontmatterIndex{}

		Describe("When using a simple template", func() {
			BeforeEach(func() {
				theFrontmatter = wikipage.FrontMatter{"identifier": "1234"}
				templateHTML := `{{ .Identifier }}`
				rendered, err = templating.ExecuteTemplate(templateHTML, theFrontmatter, site, query)
			})
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
			It("should render identifier into output", func() {
				Expect(string(rendered)).To(ContainSubstring("1234"))
			})
		})

		Describe("When using an unstructured map", func() {
			BeforeEach(func() {
				theFrontmatter = wikipage.FrontMatter{"identifier": "1234", "foobar": "baz"}
				templateHTML := `{{ index .FrontmatterMap "foobar" }}`
				rendered, err = templating.ExecuteTemplate(templateHTML, theFrontmatter, site, query)
			})
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
			It("should render unstructured data into output", func() {
				Expect(string(rendered)).To(ContainSubstring("baz"))
			})
		})
	})

	Describe("utils.StripFrontmatter", func() {
		DescribeTable("when stripping frontmatter",
			func(input string, expected string) {
				Expect(utils.StripFrontmatter(input)).To(Equal(expected))
			},
			Entry("with frontmatter", "---\ntitle: Test\n---\nThis is a test", "This is a test"),
			Entry("without frontmatter", "This is a test", "This is a test"),
		)
	})

	Describe("utils.RandomAlliterateCombo", func() {
		It("should return a non-empty string", func() {
			combo := utils.RandomAlliterateCombo()
			Expect(combo).NotTo(BeEmpty())
		})
	})

	Describe("utils.StringInSlice", func() {
		var sl []string
		BeforeEach(func() {
			sl = []string{"apple", "banana", "cherry"}
		})
		Describe("When the string is in the slice", func() {
			It("should return true", func() {
				Expect(utils.StringInSlice("banana", sl)).To(BeTrue())
			})
		})
		Describe("When the string is not in the slice", func() {
			It("should return false", func() {
				Expect(utils.StringInSlice("orange", sl)).To(BeFalse())
			})
		})
	})

	Describe("utils.ContentTypeFromName", func() {
		DescribeTable("when given a filename",
			func(filename, expectedContentType string) {
				Expect(utils.ContentTypeFromName(filename)).To(Equal(expectedContentType))
			},
			Entry("for a markdown file", "file.md", "text/markdown; charset=utf-8"),
			Entry("for a heic file", "image.heic", "image/heic"),
		)
	})

	Describe("utils.RandomStringOfLength", func() {
		It("should return a string of the specified length", func() {
			str, err := utils.RandomStringOfLength(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(str)).To(Equal(10))
		})
	})

	Describe("utils.Exists", func() {
		Describe("When the file exists", func() {
			It("should return true", func() {
				Expect(utils.Exists("./utils.go")).To(BeTrue(), "Expected file utils.go to exist in current directory for test.")
			})
		})
		Describe("When the file does not exist", func() {
			It("should return false", func() {
				Expect(utils.Exists("./nonexistent_file.go")).To(BeFalse())
			})
		})
	})

	Describe("Base32 encoding/decoding", func() {
		Describe("utils.EncodeToBase32", func() {
			It("should encode a string to base32", func() {
				Expect(utils.EncodeToBase32("hello")).To(Equal("NBSWY3DP"))
			})
		})
		Describe("utils.DecodeFromBase32", func() {
			It("should decode a base32 string", func() {
				str, err := utils.DecodeFromBase32("NBSWY3DP")
				Expect(err).NotTo(HaveOccurred())
				Expect(str).To(Equal("hello"))
			})
		})
	})
})
