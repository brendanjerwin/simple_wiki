package pagestore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPagestore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pagestore Suite")
}

var _ = Describe("Store", func() {
	var (
		store   *Store
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "pagestore-test")
		Expect(err).NotTo(HaveOccurred())
		store = NewStore(tempDir)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("ReadPage", func() {
		When("the page does not exist", func() {
			var (
				p   *wikipage.Page
				err error
			)

			BeforeEach(func() {
				p, err = store.ReadPage("nonexistent")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a non-nil page", func() {
				Expect(p).NotTo(BeNil())
			})

			It("should report the page as new", func() {
				Expect(p.WasLoadedFromDisk).To(BeFalse())
			})
		})

		When("the page exists on disk", func() {
			var p *wikipage.Page

			BeforeEach(func() {
				contents := "+++\ntitle = \"Hello\"\n+++\nbody\n"
				fp := filepath.Join(tempDir, base32tools.EncodeToBase32("hello")+".md")
				Expect(os.WriteFile(fp, []byte(contents), 0644)).To(Succeed())

				var err error
				p, err = store.ReadPage("hello")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page text as-is (pure read, no migration)", func() {
				Expect(p.Text).To(Equal("+++\ntitle = \"Hello\"\n+++\nbody\n"))
			})

			It("should mark the page as loaded from disk", func() {
				Expect(p.WasLoadedFromDisk).To(BeTrue())
			})
		})
	})

	Describe("WriteFrontMatter + ReadFrontMatter round trip", func() {
		var roundTripped wikipage.FrontMatter

		BeforeEach(func() {
			Expect(store.WriteMarkdown("rt", "body content\n")).To(Succeed())
			Expect(store.WriteFrontMatter("rt", wikipage.FrontMatter{"title": "Round Trip"})).To(Succeed())

			var err error
			_, roundTripped, err = store.ReadFrontMatter("rt")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the written frontmatter on subsequent read", func() {
			Expect(roundTripped["title"]).To(Equal("Round Trip"))
		})
	})

	Describe("ModifyFrontMatterAndMarkdown", func() {
		var (
			page      *wikipage.Page
			modifyErr error
		)

		BeforeEach(func() {
			Expect(store.WriteMarkdown("modify-both", "old body\n")).To(Succeed())
			Expect(store.WriteFrontMatter("modify-both", wikipage.FrontMatter{"title": "Old"})).To(Succeed())

			modifyErr = store.ModifyFrontMatterAndMarkdown(
				"modify-both",
				func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
					fm["title"] = "New"
					return fm, md + "new body\n", nil
				},
			)
			var readErr error
			page, readErr = store.ReadPage("modify-both")
			Expect(readErr).NotTo(HaveOccurred())
		})

		It("should not return an error", func() {
			Expect(modifyErr).NotTo(HaveOccurred())
		})

		It("should write the modified frontmatter", func() {
			Expect(page.Text).To(ContainSubstring("New"))
		})

		It("should write the modified markdown", func() {
			Expect(page.Text).To(ContainSubstring("old body\nnew body"))
		})
	})

	Describe("SoftDeletePage", func() {
		When("the page does not exist", func() {
			It("should return os.ErrNotExist", func() {
				err := store.SoftDeletePage("ghost")
				Expect(err).To(MatchError(os.ErrNotExist))
			})
		})

		When("the page exists", func() {
			var err error

			BeforeEach(func() {
				Expect(store.WriteMarkdown("doomed", "body\n")).To(Succeed())
				err = store.SoftDeletePage("doomed")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the original .md file", func() {
				fp := filepath.Join(tempDir, base32tools.EncodeToBase32("doomed")+".md")
				_, statErr := os.Stat(fp)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})
	})

	Describe("CanonicalLockKey", func() {
		It("should lowercase identifiers", func() {
			Expect(CanonicalLockKey("FOO")).To(Equal(CanonicalLockKey("foo")))
		})

		It("should return the same key for raw vs. canonically-munged identifiers", func() {
			// Both should reach MungeIdentifier and produce a single canonical form.
			Expect(CanonicalLockKey("Hello_World")).To(Equal(CanonicalLockKey("hello_world")))
		})
	})
})
