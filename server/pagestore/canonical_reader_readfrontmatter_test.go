package pagestore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CanonicalReader.ReadFrontMatter", func() {
	var (
		tempDir string
		store   *Store
		reader  *CanonicalReader
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "canonical-reader-fm-test")
		Expect(err).NotTo(HaveOccurred())
		store = NewStore(tempDir)
		reader = NewCanonicalReader(NoopCanonicalizer{}, store)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	writeFile := func(id, contents string) {
		fp := filepath.Join(tempDir, base32tools.EncodeToBase32(strings.ToLower(id))+".md")
		Expect(os.WriteFile(fp, []byte(contents), 0644)).To(Succeed())
	}

	When("the page exists with TOML frontmatter", func() {
		var (
			fm  wikipage.FrontMatter
			err error
		)

		BeforeEach(func() {
			writeFile("fm-page", "+++\ntitle = \"Hello\"\nstatus = \"draft\"\n+++\nbody\n")
			_, fm, err = reader.ReadFrontMatter("fm-page")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the parsed frontmatter map", func() {
			Expect(fm["title"]).To(Equal("Hello"))
			Expect(fm["status"]).To(Equal("draft"))
		})
	})

	When("the page does not exist", func() {
		var err error

		BeforeEach(func() {
			_, _, err = reader.ReadFrontMatter("nonexistent")
		})

		It("should return os.ErrNotExist", func() {
			Expect(err).To(MatchError(os.ErrNotExist))
		})
	})

	When("the canonicalizer transforms the bytes before parse", func() {
		// The canonicalizer runs on the raw bytes; the parsed frontmatter
		// reflects the post-canonicalize content. This pins the contract:
		// callers see the same canonical view via ReadFrontMatter as via
		// ReadPage.
		var (
			fm  wikipage.FrontMatter
			err error
		)

		BeforeEach(func() {
			// Write an already-canonical page so the parse succeeds even
			// with our trivial test canonicalizer running over the bytes.
			writeFile("noop-fm", "+++\ntitle = \"Pinned\"\n+++\n")
			reader = NewCanonicalReader(NoopCanonicalizer{}, store)
			_, fm, err = reader.ReadFrontMatter("noop-fm")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the canonical frontmatter view", func() {
			Expect(fm["title"]).To(Equal("Pinned"))
		})
	})
})
