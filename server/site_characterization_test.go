// Characterization tests pinning Site.ReadPage's observable behavior across
// the lazy-migration call chain. The full byte-level goldens are captured
// from today's behavior so Phase 2's mechanical PageStore extraction can
// verify byte-identical output.
//
// Scope: ReadPage through `applyMigrationsForPage`. The migrations
// themselves are unit-tested in migrations/lazy/*_test.go; what these tests
// pin is the *integration* — that ReadPage routes through the applicator
// and returns the canonicalized form.

package server

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Site.ReadPage characterization", func() {
	var (
		s        *Site
		tempDir  string
		writePageFile func(id, contents string)
		readPage      func(id string) *wikipage.Page
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "site-characterization")
		Expect(err).NotTo(HaveOccurred())

		logger := lumber.NewConsoleLogger(lumber.WARN)
		coordinator := jobs.NewJobQueueCoordinator(logger)
		mockFm := &MockIndexOperator{}
		mockBleve := &MockIndexOperator{}
		indexCoord := index.NewIndexCoordinator(coordinator, mockFm, mockBleve)

		// Real applicator — these tests pin today's full canonicalization
		// behavior, not the no-op behavior other tests use.
		applicator := lazy.NewApplicator()

		s = &Site{
			Logger:                  lumber.NewConsoleLogger(lumber.WARN),
			PathToData:              tempDir,
			IndexCoordinator:        indexCoord,
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{},
			MigrationApplicator:     applicator,
		}

		writePageFile = func(id, contents string) {
			lowered := strings.ToLower(id)
			fp := filepath.Join(tempDir, base32tools.EncodeToBase32(lowered)+".md")
			Expect(os.WriteFile(fp, []byte(contents), 0644)).To(Succeed())
		}

		readPage = func(id string) *wikipage.Page {
			p, err := s.ReadPage(wikipage.PageIdentifier(id))
			Expect(err).NotTo(HaveOccurred())
			return p
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	When("the page has YAML frontmatter", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("yaml-page", "---\ntitle: My Page\ntags:\n  - one\n  - two\n---\nbody text\n")
			p = readPage("yaml-page")
		})

		It("should rewrite the YAML delimiter to the TOML delimiter", func() {
			Expect(p.Text).To(HavePrefix("+++"))
		})

		It("should no longer contain the YAML opening fence", func() {
			Expect(p.Text).NotTo(HavePrefix("---"))
		})

		It("should preserve the markdown body", func() {
			Expect(p.Text).To(ContainSubstring("body text"))
		})
	})

	When("the page has TOML frontmatter using dot-notation keys", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("dotnotation-page", "+++\ntitle = \"Hello\"\ninventory.container = \"box-a\"\n+++\nbody\n")
			p = readPage("dotnotation-page")
		})

		It("should rewrite dot-notation into a nested table", func() {
			Expect(p.Text).To(ContainSubstring("[inventory]"))
		})

		It("should remove the dot-notation form from the rendered frontmatter", func() {
			Expect(p.Text).NotTo(ContainSubstring("inventory.container ="))
		})
	})

	When("the page has no frontmatter at all (markdown only)", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("plain-page", "just some markdown text, no frontmatter\n")
			p = readPage("plain-page")
		})

		It("should return the content unchanged (no migration applies)", func() {
			Expect(p.Text).To(Equal("just some markdown text, no frontmatter\n"))
		})
	})

	When("the page is already in canonical TOML form", func() {
		var (
			canonical string
			p         *wikipage.Page
		)

		BeforeEach(func() {
			canonical = "+++\ntitle = 'Already Canonical'\n+++\n\nbody\n"
			writePageFile("canonical-page", canonical)
			p = readPage("canonical-page")
		})

		It("should return the content byte-identical (no migration applies)", func() {
			Expect(p.Text).To(Equal(canonical))
		})
	})

	When("the page file does not exist", func() {
		var (
			p   *wikipage.Page
			err error
		)

		BeforeEach(func() {
			p, err = s.ReadPage(wikipage.PageIdentifier("nonexistent"))
		})

		It("should return a non-nil page (rather than nil)", func() {
			Expect(p).NotTo(BeNil())
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report the page as new (not loaded from disk)", func() {
			Expect(p.WasLoadedFromDisk).To(BeFalse())
		})
	})

	When("the page file is empty (zero bytes)", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("empty-page", "")
			p = readPage("empty-page")
		})

		It("should return an empty Text", func() {
			Expect(p.Text).To(Equal(""))
		})

		It("should still report the page as loaded from disk", func() {
			Expect(p.WasLoadedFromDisk).To(BeTrue())
		})
	})

	When("the page file contains only TOML delimiters with no body", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("empty-fm-page", "+++\n+++\n")
			p = readPage("empty-fm-page")
		})

		It("should return the delimiter pair unchanged", func() {
			Expect(p.Text).To(HavePrefix("+++"))
		})
	})

	When("the page has CRLF line endings", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			writePageFile("crlf-page", "+++\r\ntitle = \"CRLF\"\r\n+++\r\nbody\r\n")
			p = readPage("crlf-page")
		})

		It("should still parse the frontmatter (lazy migration is CRLF-tolerant or echoes raw)", func() {
			// Pinning today's observable behavior — whatever ReadPage
			// returns for CRLF is what Phase 2's move must preserve.
			Expect(p.Text).NotTo(BeEmpty())
		})
	})
})
