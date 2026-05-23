package canonicalize

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCanonicalize(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Canonicalize Suite")
}

var _ = Describe("FormatCanonicalizer", func() {
	var c *FormatCanonicalizer

	BeforeEach(func() {
		c = NewFormatCanonicalizer()
	})

	When("content has YAML frontmatter", func() {
		var result []byte
		var err error

		BeforeEach(func() {
			input := []byte("---\ntitle: My Page\n---\nbody\n")
			result, err = c.Canonicalize(input)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should rewrite the delimiter to TOML form", func() {
			Expect(string(result)).To(HavePrefix("+++"))
		})

		It("should preserve the body content", func() {
			Expect(string(result)).To(ContainSubstring("body"))
		})
	})

	When("content has TOML frontmatter with dot-notation keys", func() {
		var result []byte
		var err error

		BeforeEach(func() {
			input := []byte("+++\ntitle = \"Hello\"\ninventory.container = \"box-a\"\n+++\nbody\n")
			result, err = c.Canonicalize(input)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should rewrite dot-notation to a nested table", func() {
			Expect(string(result)).To(ContainSubstring("[inventory]"))
		})
	})

	When("content has no frontmatter", func() {
		var result []byte
		var err error

		BeforeEach(func() {
			input := []byte("just markdown content\n")
			result, err = c.Canonicalize(input)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the content unchanged", func() {
			Expect(string(result)).To(Equal("just markdown content\n"))
		})
	})

	When("content is already canonical", func() {
		var result []byte
		var err error
		var input []byte

		BeforeEach(func() {
			input = []byte("+++\ntitle = 'Hello'\n+++\nbody\n")
			result, err = c.Canonicalize(input)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the content unchanged (idempotency)", func() {
			Expect(string(result)).To(Equal(string(input)))
		})
	})

	When("Canonicalize is called twice on the same input", func() {
		// Pins migration idempotency: running the chain twice produces the
		// same result. Failure here means a migration's AppliesTo is
		// non-idempotent, which would cause a runaway loop in the read or
		// write path.
		var first, second []byte
		var err error

		BeforeEach(func() {
			input := []byte("---\ntitle: Hello\n---\n# Body\n")
			first, err = c.Canonicalize(input)
			Expect(err).NotTo(HaveOccurred())
			second, err = c.Canonicalize(first)
		})

		It("should not return an error on the second pass", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce byte-identical output across both passes (idempotent)", func() {
			Expect(string(second)).To(Equal(string(first)))
		})
	})
})
