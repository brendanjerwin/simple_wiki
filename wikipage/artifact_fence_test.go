package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("SplitArtifactFences", func() {
	When("the body has no artifact fence", func() {
		It("returns empty artifact source and the body unchanged as prose", func() {
			body := "# Hello\n\nSome prose.\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeFalse())
			Expect(artifact).To(Equal(""))
			Expect(prose).To(Equal(body))
		})

		It("returns empty inputs unchanged", func() {
			artifact, prose, found := wikipage.SplitArtifactFences("")
			Expect(found).To(BeFalse())
			Expect(artifact).To(Equal(""))
			Expect(prose).To(Equal(""))
		})

		It("returns an empty body unchanged", func() {
			artifact, prose, found := wikipage.SplitArtifactFences("")
			Expect(found).To(BeFalse())
			Expect(artifact).To(Equal(""))
			Expect(prose).To(Equal(""))
		})
	})

	When("the body has a wiki-artifact fence", func() {
		It("extracts the program and removes the fence from the prose remainder", func() {
			body := "Intro prose.\n\n```wiki-artifact\n@Card {\n  Text [\"hi\"]\n}\n```\n\nOutro prose.\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeTrue())
			Expect(artifact).To(Equal("@Card {\n  Text [\"hi\"]\n}"))
			// The program bytes must not appear in the prose remainder.
			Expect(prose).NotTo(ContainSubstring("wiki-artifact"))
			Expect(prose).NotTo(ContainSubstring("@Card"))
			// Prose = body minus fence, exact.
			Expect(prose).To(Equal("Intro prose.\n\n\nOutro prose.\n"))
		})

		It("closes an unterminated fence at end of body instead of eating the rest silently", func() {
			body := "Prose.\n\n```wiki-artifact\n@Card {\n  Text [\"hi\"]\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeTrue())
			// Invariant: artifact source == exact fence-content text. The
			// unterminated fence's own trailing newline belongs to it.
			Expect(artifact).To(Equal("@Card {\n  Text [\"hi\"]\n"))
			Expect(prose).To(Equal("Prose.\n"))
		})
	})

	When("the body has multiple wiki-artifact fences", func() {
		It("keeps only the first fence as the artifact and leaves later fences out of prose", func() {
			body := "```wiki-artifact\n@First\n```\n\nmid prose\n\n```wiki-artifact\n@Second\n```\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeTrue())
			Expect(artifact).To(Equal("@First"))
			// Later fences are also artifact fences: excluded from prose, not rendered.
			Expect(prose).NotTo(ContainSubstring("@Second"))
			Expect(prose).To(ContainSubstring("mid prose"))
		})
	})

	When("the body has other fenced code blocks", func() {
		It("leaves ordinary fences in the prose remainder", func() {
			body := "```go\nfmt.Println(\"hi\")\n```\n\nprose after\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeFalse())
			Expect(artifact).To(Equal(""))
			Expect(prose).To(ContainSubstring("fmt.Println"))
			Expect(prose).To(ContainSubstring("prose after"))
		})

		It("leaves look-alike info strings in the prose remainder", func() {
			body := "```wiki-artifacts\n@NotAnArtifact\n```\n\nprose\n"
			artifact, prose, found := wikipage.SplitArtifactFences(body)
			Expect(found).To(BeFalse())
			Expect(artifact).To(Equal(""))
			Expect(prose).To(ContainSubstring("@NotAnArtifact"))
		})
	})
})
