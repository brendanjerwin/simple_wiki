//revive:disable:dot-imports
package server

import (
	"bytes"
	"errors"
	"io"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// errWriter is an io.Writer that always returns the given error.
type errWriter struct{ err error }

func (e *errWriter) Write(_ []byte) (int, error) { return 0, e.err }

var _ = Describe("combineFrontmatterAndMarkdown", func() {
	When("both frontmatter and markdown are empty", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = combineFrontmatterAndMarkdown(wikipage.FrontMatter{}, "")
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
			fm := wikipage.FrontMatter{"title": "Test Page"}
			result, err = combineFrontmatterAndMarkdown(fm, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include TOML delimiters", func() {
			Expect(result).To(ContainSubstring(tomlDelimiter))
		})

		It("should include the frontmatter key", func() {
			Expect(result).To(ContainSubstring("title"))
		})
	})

	When("only markdown is present", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = combineFrontmatterAndMarkdown(wikipage.FrontMatter{}, "# Hello\n")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include the markdown content", func() {
			Expect(result).To(ContainSubstring("# Hello"))
		})

		It("should not include TOML delimiters", func() {
			Expect(result).NotTo(ContainSubstring(tomlDelimiter))
		})
	})

	When("both frontmatter and markdown are present", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"title": "My Page"}
			result, err = combineFrontmatterAndMarkdown(fm, "# My Page\n\nSome content.")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include both TOML delimiters and markdown content", func() {
			Expect(result).To(ContainSubstring(tomlDelimiter))
			Expect(result).To(ContainSubstring("# My Page"))
		})
	})

	When("frontmatter contains an unmarshalable value", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := wikipage.FrontMatter{
				"bad": make(chan int), // channels cannot be TOML-serialized
			}
			result, err = combineFrontmatterAndMarkdown(fm, "# Hello\n")
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should return an empty string", func() {
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
			// Pass raw TOML bytes without a trailing newline to exercise the newline-appending branch
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
			// A writer that fails after the first successful write (the opening delimiter)
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
			// Fail on the 3rd write call: opening delimiter (1), fmBytes (2), newline (3 — fails)
			callCount := 0
			w := writerFunc(func(p []byte) (int, error) {
				callCount++
				if callCount >= 3 {
					return 0, errors.New("newline write failed")
				}
				return len(p), nil
			})
			// fmBytes without trailing newline triggers the newline-write branch
			err = writeFrontmatterToBuffer(w, []byte(`a = "b"`))
		})

		It("should return the newline write error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("the writer fails on the closing delimiter write", func() {
		var err error

		BeforeEach(func() {
			// Fail on the 3rd write call when fmBytes already has trailing newline:
			// opening delimiter (1), fmBytes (2), closing delimiter (3 — fails)
			callCount := 0
			w := writerFunc(func(p []byte) (int, error) {
				callCount++
				if callCount >= 3 {
					return 0, errors.New("closing delimiter write failed")
				}
				return len(p), nil
			})
			// fmBytes with trailing newline skips the newline-write branch
			err = writeFrontmatterToBuffer(w, []byte("a = \"b\"\n"))
		})

		It("should return the closing delimiter write error", func() {
			Expect(err).To(HaveOccurred())
		})
	})
})

// writerFunc adapts a plain function to the io.Writer interface.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

var _ io.Writer = writerFunc(nil)
