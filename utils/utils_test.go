package utils_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Utils Suite")
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

func (m *MockQueryFrontmatterIndex) QueryExactMatch(keyPath frontmatter.DottedKeyPath, _ frontmatter.Value) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) QueryKeyExistence(keyPath frontmatter.DottedKeyPath) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) QueryPrefixMatch(keyPath frontmatter.DottedKeyPath, _ string) []wikipage.PageIdentifier {
	return m.Results[string(keyPath)]
}

func (m *MockQueryFrontmatterIndex) GetValue(_ wikipage.PageIdentifier, _ frontmatter.DottedKeyPath) frontmatter.Value {
	return m.GetValueResult
}

var _ = ginkgo.Describe("Utils", func() {
	ginkgo.Describe("ReverseSlice", func() {
		ginkgo.Describe("ReverseSliceInt64", func() {
			ginkgo.It("should reverse a slice of int64", func() {
				slice := []int64{1, 2, 3, 4, 5}
				reversed := utils.ReverseSliceInt64(slice)
				gomega.Expect(reversed).To(gomega.Equal([]int64{5, 4, 3, 2, 1}))
			})
		})

		ginkgo.Describe("ReverseSliceString", func() {
			ginkgo.It("should reverse a slice of strings", func() {
				slice := []string{"apple", "banana", "cherry"}
				reversed := utils.ReverseSliceString(slice)
				gomega.Expect(reversed).To(gomega.Equal([]string{"cherry", "banana", "apple"}))
			})
		})

		ginkgo.Describe("ReverseSliceInt", func() {
			ginkgo.It("should reverse a slice of int", func() {
				slice := []int{1, 2, 3, 4, 5}
				reversed := utils.ReverseSliceInt(slice)
				gomega.Expect(reversed).To(gomega.Equal([]int{5, 4, 3, 2, 1}))
			})
		})
	})

	ginkgo.Describe("MarkdownToHTMLAndJSONFrontmatter", func() {
		var (
			markdown string
			html     []byte
		)

		ginkgo.BeforeEach(func() {
			markdown = `
---
sample: "value"
---

# Hello
	`
			var err error
			html, _, err = utils.MarkdownToHTMLAndJSONFrontmatter(markdown, &MockReadFrontMatter{}, &goldmarkrenderer.GoldmarkRenderer{}, &MockQueryFrontmatterIndex{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should remove frontmatter from the output", func() {
			gomega.Expect(string(html)).NotTo(gomega.ContainSubstring("sample:"))
		})

		ginkgo.It("should render the markdown to HTML", func() {
			gomega.Expect(string(html)).To(gomega.ContainSubstring(">Hello</h1"))
		})
	})

	ginkgo.Describe("templating.ExecuteTemplate", func() {
		var (
			theFrontmatter wikipage.FrontMatter
			rendered       []byte
			err            error
		)
		var site wikipage.PageReader = &MockReadFrontMatter{}
		var query frontmatter.IQueryFrontmatterIndex = &MockQueryFrontmatterIndex{}

		ginkgo.Describe("When using a simple template", func() {
			ginkgo.BeforeEach(func() {
				theFrontmatter = wikipage.FrontMatter{"identifier": "1234"}
				templateHTML := `{{ .Identifier }}`
				rendered, err = templating.ExecuteTemplate(templateHTML, theFrontmatter, site, query)
			})
			ginkgo.It("should not return an error", func() {
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("should render identifier into output", func() {
				gomega.Expect(string(rendered)).To(gomega.ContainSubstring("1234"))
			})
		})

		ginkgo.Describe("When using an unstructured map", func() {
			ginkgo.BeforeEach(func() {
				theFrontmatter = wikipage.FrontMatter{"identifier": "1234", "foobar": "baz"}
				templateHTML := `{{ index .FrontmatterMap "foobar" }}`
				rendered, err = templating.ExecuteTemplate(templateHTML, theFrontmatter, site, query)
			})
			ginkgo.It("should not return an error", func() {
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("should render unstructured data into output", func() {
				gomega.Expect(string(rendered)).To(gomega.ContainSubstring("baz"))
			})
		})
	})

	ginkgo.Describe("StripFrontmatter", func() {
		ginkgo.DescribeTable("when stripping frontmatter",
			func(input string, expected string) {
				gomega.Expect(utils.StripFrontmatter(input)).To(gomega.Equal(expected))
			},
			ginkgo.Entry("with frontmatter", "---\ntitle: Test\n---\nThis is a test", "This is a test"),
			ginkgo.Entry("without frontmatter", "This is a test", "This is a test"),
		)
	})

	ginkgo.Describe("utils.RandomAlliterateCombo", func() {
		ginkgo.It("should return a non-empty string", func() {
			combo := utils.RandomAlliterateCombo()
			gomega.Expect(combo).NotTo(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("utils.StringInSlice", func() {
		var sl []string
		ginkgo.BeforeEach(func() {
			sl = []string{"apple", "banana", "cherry"}
		})
		ginkgo.Describe("When the string is in the slice", func() {
			ginkgo.It("should return true", func() {
				gomega.Expect(utils.StringInSlice("banana", sl)).To(gomega.BeTrue())
			})
		})
		ginkgo.Describe("When the string is not in the slice", func() {
			ginkgo.It("should return false", func() {
				gomega.Expect(utils.StringInSlice("orange", sl)).To(gomega.BeFalse())
			})
		})
	})

	ginkgo.Describe("ContentTypeFromName", func() {
		ginkgo.DescribeTable("when given a filename",
			func(filename, expectedContentType string) {
				gomega.Expect(utils.ContentTypeFromName(filename)).To(gomega.Equal(expectedContentType))
			},
			ginkgo.Entry("for a markdown file", "file.md", "text/markdown; charset=utf-8"),
			ginkgo.Entry("for a heic file", "image.heic", "image/heic"),
		)
	})

	ginkgo.Describe("utils.RandomStringOfLength", func() {
		ginkgo.It("should return a string of the specified length", func() {
			str, err := utils.RandomStringOfLength(10)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(len(str)).To(gomega.Equal(10))
		})
	})

	ginkgo.Describe("Exists", func() {
		ginkgo.Describe("When the file exists", func() {
			ginkgo.It("should return true", func() {
				gomega.Expect(utils.Exists("./utils.go")).To(gomega.BeTrue(), "Expected file utils.go to exist in current directory for test.")
			})
		})
		ginkgo.Describe("When the file does not exist", func() {
			ginkgo.It("should return false", func() {
				gomega.Expect(utils.Exists("./nonexistent_file.go")).To(gomega.BeFalse())
			})
		})
	})

	ginkgo.Describe("Base32 encoding/decoding", func() {
		ginkgo.Describe("utils.EncodeToBase32", func() {
			ginkgo.It("should encode a string to base32", func() {
				gomega.Expect(utils.EncodeToBase32("hello")).To(gomega.Equal("NBSWY3DP"))
			})
		})
		ginkgo.Describe("utils.DecodeFromBase32", func() {
			ginkgo.It("should decode a base32 string", func() {
				str, err := utils.DecodeFromBase32("NBSWY3DP")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(str).To(gomega.Equal("hello"))
			})
		})
	})
})
