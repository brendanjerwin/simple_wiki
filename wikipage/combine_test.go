//revive:disable:dot-imports
package wikipage

import (
	"bytes"
	"errors"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// errWriter is an io.Writer that always returns the given error.
type errWriter struct{ err error }

func (e *errWriter) Write(_ []byte) (int, error) { return 0, e.err }

// writerFunc adapts a plain function to the io.Writer interface.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

var _ io.Writer = writerFunc(nil)

var _ = Describe("CombineFrontMatterAndMarkdown", func() {
	When("both frontmatter and markdown are empty", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = CombineFrontMatterAndMarkdown(FrontMatter{}, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty string", func() {
			Expect(result).To(Equal(""))
		})
	})

	When("only frontmatter is present", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := FrontMatter{"title": "Hello"}
			result, err = CombineFrontMatterAndMarkdown(fm, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce TOML-delimited frontmatter and no body", func() {
			Expect(result).To(HavePrefix(tomlDelimiter))
			Expect(result).To(ContainSubstring(`title = 'Hello'`))
			Expect(result).To(HaveSuffix(tomlDelimiter))
		})
	})

	When("only markdown is present", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = CombineFrontMatterAndMarkdown(FrontMatter{}, "# Hello\n")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return just the markdown body (no delimiters)", func() {
			Expect(result).To(Equal("# Hello\n"))
		})
	})

	When("both frontmatter and markdown are present", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := FrontMatter{"title": "Test"}
			result, err = CombineFrontMatterAndMarkdown(fm, "# My Page\n\nSome content.")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should wrap frontmatter in delimiters and concatenate the body", func() {
			Expect(result).To(HavePrefix(tomlDelimiter))
			Expect(result).To(ContainSubstring(`title = 'Test'`))
			Expect(result).To(ContainSubstring("# My Page"))
			Expect(result).To(ContainSubstring("Some content."))
		})
	})

	When("the frontmatter cannot be marshalled", func() {
		// toml.Marshal handles essentially everything we throw at it; this
		// test exercises the error-path code shape via a value that toml
		// cannot serialize (a channel).
		var result string
		var err error

		BeforeEach(func() {
			fm := FrontMatter{"bad": make(chan int)}
			result, err = CombineFrontMatterAndMarkdown(fm, "# Hello\n")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty string on error", func() {
			Expect(result).To(BeEmpty())
		})
	})
})

var _ = Describe("writeFrontmatterToBuffer", func() {
	When("fmBytes does not end with a newline", func() {
		var err error
		var buf bytes.Buffer
		var output string

		BeforeEach(func() {
			buf.Reset()
			err = writeFrontmatterToBuffer(&buf, []byte(`a = "b"`))
			output = buf.String()
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should insert a newline before the closing delimiter", func() {
			Expect(output).To(ContainSubstring("\"b\"\n" + tomlDelimiter))
		})

		It("should wrap the content in TOML delimiters", func() {
			Expect(output).To(HavePrefix(tomlDelimiter))
			Expect(output).To(HaveSuffix(tomlDelimiter))
		})
	})

	When("fmBytes already ends with a newline", func() {
		var err error
		var buf bytes.Buffer
		var output string

		BeforeEach(func() {
			buf.Reset()
			err = writeFrontmatterToBuffer(&buf, []byte("a = \"b\"\n"))
			output = buf.String()
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not insert an extra newline", func() {
			Expect(output).To(Equal(tomlDelimiter + "a = \"b\"\n" + tomlDelimiter))
		})
	})

	When("the writer returns an error on the first write", func() {
		var err error
		var writeErr error

		BeforeEach(func() {
			writeErr = errors.New("write failed")
			err = writeFrontmatterToBuffer(&errWriter{err: writeErr}, []byte("a = \"b\"\n"))
		})

		It("should propagate the write error", func() {
			Expect(err).To(MatchError(writeErr))
		})
	})

	When("the writer accepts WriteString but fails on Write of fmBytes", func() {
		var err error

		BeforeEach(func() {
			callCount := 0
			w := writerFunc(func(p []byte) (int, error) {
				callCount++
				if callCount > 1 {
					return 0, errors.New("body write failed")
				}
				return len(p), nil
			})
			err = writeFrontmatterToBuffer(w, []byte("a = \"b\"\n"))
		})

		It("should return the write error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("the writer fails on the newline write (content has no trailing newline)", func() {
		var err error

		BeforeEach(func() {
			callCount := 0
			w := writerFunc(func(p []byte) (int, error) {
				callCount++
				if callCount >= 3 {
					return 0, errors.New("newline write failed")
				}
				return len(p), nil
			})
			err = writeFrontmatterToBuffer(w, []byte(`a = "b"`))
		})

		It("should return the newline write error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("the writer fails on the closing delimiter write", func() {
		var err error

		BeforeEach(func() {
			callCount := 0
			w := writerFunc(func(p []byte) (int, error) {
				callCount++
				if callCount >= 3 {
					return 0, errors.New("closing delimiter write failed")
				}
				return len(p), nil
			})
			err = writeFrontmatterToBuffer(w, []byte("a = \"b\"\n"))
		})

		It("should return the closing delimiter write error", func() {
			Expect(err).To(HaveOccurred())
		})
	})
})
