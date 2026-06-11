package pagestore

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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
		When("the modifier succeeds", func() {
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

		When("the modifier returns an error", func() {
			var (
				modifyErr   error
				page        *wikipage.Page
				modifierErr = errors.New("modifier refused")
			)

			BeforeEach(func() {
				Expect(store.WriteMarkdown("modify-both-error", "old body\n")).To(Succeed())
				Expect(store.WriteFrontMatter("modify-both-error", wikipage.FrontMatter{"title": "Old"})).To(Succeed())

				modifyErr = store.ModifyFrontMatterAndMarkdown(
					"modify-both-error",
					func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
						return nil, "", modifierErr
					},
				)
				var readErr error
				page, readErr = store.ReadPage("modify-both-error")
				Expect(readErr).NotTo(HaveOccurred())
			})

			It("should return the modifier error", func() {
				Expect(modifyErr).To(MatchError(modifierErr))
			})

			It("should not write partial changes", func() {
				Expect(page.Text).To(ContainSubstring("Old"))
				Expect(page.Text).To(ContainSubstring("old body"))
			})
		})

		When("frontmatter cannot be parsed", func() {
			var modifyErr error

			BeforeEach(func() {
				fp := filepath.Join(tempDir, base32tools.EncodeToBase32("broken-fm")+".md")
				Expect(os.WriteFile(fp, []byte("+++\ntitle = [invalid\n+++\nbody\n"), 0644)).To(Succeed())
				modifyErr = store.ModifyFrontMatterAndMarkdown(
					"broken-fm",
					func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
						return fm, md, nil
					},
				)
			})

			It("should return a frontmatter parse error", func() {
				Expect(modifyErr).To(MatchError(ContainSubstring("failed to parse frontmatter for page modification")))
			})
		})
	})

	Describe("SoftDeletePage", func() {
		When("the page does not exist", func() {
			var trashEntries []wikipage.TrashEntry

			BeforeEach(func() {
				deleteErr := store.SoftDeletePage("ghost")
				Expect(deleteErr).To(MatchError(os.ErrNotExist))
				var listErr error
				trashEntries, listErr = store.ListTrash()
				Expect(listErr).NotTo(HaveOccurred())
			})

			It("should leave trash empty", func() {
				Expect(trashEntries).To(BeEmpty())
			})
		})

		When("the page exists", func() {
			var (
				err          error
				trashEntries []wikipage.TrashEntry
			)

			BeforeEach(func() {
				Expect(store.WriteFrontMatter("doomed", wikipage.FrontMatter{"title": "Doomed Page"})).To(Succeed())
				Expect(store.WriteMarkdown("doomed", "body\n")).To(Succeed())
				err = store.SoftDeletePage("doomed")
				trashEntries, err = store.ListTrash()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the original .md file", func() {
				fp := filepath.Join(tempDir, base32tools.EncodeToBase32("doomed")+".md")
				_, statErr := os.Stat(fp)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})

			It("should list the page in trash", func() {
				Expect(trashEntries).To(HaveLen(1))
				Expect(trashEntries[0].Identifier).To(Equal(wikipage.PageIdentifier("doomed")))
				Expect(trashEntries[0].Title).To(Equal("Doomed Page"))
				Expect(trashEntries[0].PurgesAt.Sub(trashEntries[0].DeletedAt)).To(Equal(30 * 24 * time.Hour))
			})
		})

		When("the page title cannot be read", func() {
			var trashEntries []wikipage.TrashEntry

			BeforeEach(func() {
				mdPath := filepath.Join(tempDir, base32tools.EncodeToBase32("untitled")+".md")
				Expect(os.WriteFile(mdPath, []byte("body without frontmatter\n"), 0644)).To(Succeed())
				Expect(store.SoftDeletePage("untitled")).To(Succeed())
				var err error
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should leave the title empty", func() {
				Expect(trashEntries).To(HaveLen(1))
				Expect(trashEntries[0].Title).To(BeEmpty())
			})
		})
	})

	Describe("ListTrash", func() {
		When("one trash entry has corrupted metadata", func() {
			var trashEntries []wikipage.TrashEntry

			BeforeEach(func() {
				Expect(store.WriteMarkdown("visible-trash", "body\n")).To(Succeed())
				Expect(store.SoftDeletePage("visible-trash")).To(Succeed())
				brokenDir := filepath.Join(tempDir, deletedDirName, "broken")
				Expect(os.MkdirAll(brokenDir, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(brokenDir, trashMetadataName), []byte("identifier = [broken\n"), 0644)).To(Succeed())

				var err error
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the readable entries", func() {
				Expect(trashEntries).To(HaveLen(1))
				Expect(trashEntries[0].Identifier).To(Equal(wikipage.PageIdentifier("visible-trash")))
			})
		})

		When("a legacy trash entry has no metadata", func() {
			var trashEntries []wikipage.TrashEntry

			BeforeEach(func() {
				legacyTrashID := strconv.FormatInt(time.Now().UTC().Add(24*time.Hour).Unix(), decimalBase)
				legacyDir := filepath.Join(tempDir, deletedDirName, legacyTrashID)
				Expect(os.MkdirAll(legacyDir, 0755)).To(Succeed())
				mdPath := filepath.Join(legacyDir, base32tools.EncodeToBase32("legacy-trash")+".md")
				Expect(os.WriteFile(mdPath, []byte("legacy body\n"), 0644)).To(Succeed())

				var err error
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should derive metadata from the markdown filename", func() {
				Expect(trashEntries).To(HaveLen(1))
				Expect(trashEntries[0].Identifier).To(Equal(wikipage.PageIdentifier("legacy-trash")))
			})
		})
	})

	Describe("RestorePage", func() {
		When("the trash entry exists", func() {
			var (
				restoreErr error
				readErr    error
				markdown   wikipage.Markdown
			)

			BeforeEach(func() {
				Expect(store.WriteMarkdown("restore-me", "restored body\n")).To(Succeed())
				Expect(store.SoftDeletePage("restore-me")).To(Succeed())
				trashEntries, err := store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
				Expect(trashEntries).To(HaveLen(1))

				restoreErr = store.RestorePage(trashEntries[0].TrashID)
				page, pageErr := store.ReadPage("restore-me")
				Expect(pageErr).NotTo(HaveOccurred())
				markdown, readErr = page.GetMarkdown()
			})

			It("should not return an error", func() {
				Expect(restoreErr).NotTo(HaveOccurred())
			})

			It("should restore the page content", func() {
				Expect(readErr).NotTo(HaveOccurred())
				Expect(markdown).To(Equal(wikipage.Markdown("restored body\n")))
			})
		})

		When("the page already exists", func() {
			var restoreErr error

			BeforeEach(func() {
				Expect(store.WriteMarkdown("conflict-page", "old body\n")).To(Succeed())
				Expect(store.SoftDeletePage("conflict-page")).To(Succeed())
				trashEntries, err := store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
				Expect(trashEntries).To(HaveLen(1))
				Expect(store.WriteMarkdown("conflict-page", "new body\n")).To(Succeed())

				restoreErr = store.RestorePage(trashEntries[0].TrashID)
			})

			It("should return a conflict error", func() {
				Expect(restoreErr).To(MatchError(MatchRegexp("page restore conflict: conflict-page already exists")))
				Expect(errors.Is(restoreErr, wikipage.ErrPageRestoreConflict)).To(BeTrue())
			})
		})

		When("the trash id is invalid", func() {
			var restoreErr error

			BeforeEach(func() {
				restoreErr = store.RestorePage("../bad")
			})

			It("should return an invalid trash id error", func() {
				Expect(restoreErr).To(MatchError(ContainSubstring("invalid trash id")))
			})
		})
	})

	Describe("PurgePage", func() {
		When("the trash entry exists", func() {
			var (
				purgeErr     error
				trashEntries []wikipage.TrashEntry
			)

			BeforeEach(func() {
				Expect(store.WriteMarkdown("purge-me", "body\n")).To(Succeed())
				Expect(store.SoftDeletePage("purge-me")).To(Succeed())
				initialEntries, err := store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
				Expect(initialEntries).To(HaveLen(1))

				purgeErr = store.PurgePage(initialEntries[0].TrashID)
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(purgeErr).NotTo(HaveOccurred())
			})

			It("should remove the trash entry", func() {
				Expect(trashEntries).To(BeEmpty())
			})
		})

		When("the trash entry does not exist", func() {
			var purgeErr error

			BeforeEach(func() {
				purgeErr = store.PurgePage("missing-trash-id")
			})

			It("should return not found", func() {
				Expect(purgeErr).To(MatchError(os.ErrNotExist))
			})
		})

		When("the trash id is invalid", func() {
			var purgeErr error

			BeforeEach(func() {
				purgeErr = store.PurgePage("../bad")
			})

			It("should return an invalid trash id error", func() {
				Expect(purgeErr).To(MatchError(ContainSubstring("invalid trash id")))
			})
		})
	})

	Describe("EmptyTrash", func() {
		When("trash is empty", func() {
			var (
				count    int
				emptyErr error
			)

			BeforeEach(func() {
				count, emptyErr = store.EmptyTrash()
			})

			It("should not return an error", func() {
				Expect(emptyErr).NotTo(HaveOccurred())
			})

			It("should report zero purged entries", func() {
				Expect(count).To(Equal(0))
			})
		})

		When("trash contains entries", func() {
			var (
				count        int
				emptyErr     error
				trashEntries []wikipage.TrashEntry
			)

			BeforeEach(func() {
				Expect(store.WriteMarkdown("first-trash", "body\n")).To(Succeed())
				Expect(store.SoftDeletePage("first-trash")).To(Succeed())
				Expect(store.WriteMarkdown("second-trash", "body\n")).To(Succeed())
				Expect(store.SoftDeletePage("second-trash")).To(Succeed())

				count, emptyErr = store.EmptyTrash()
				var err error
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(emptyErr).NotTo(HaveOccurred())
			})

			It("should report the purged entries", func() {
				Expect(count).To(Equal(2))
			})

			It("should remove all trash entries", func() {
				Expect(trashEntries).To(BeEmpty())
			})
		})
	})

	Describe("PurgeExpiredTrash", func() {
		When("a trash entry is past retention", func() {
			var trashEntries []wikipage.TrashEntry

			BeforeEach(func() {
				Expect(store.WriteMarkdown("expired-page", "body\n")).To(Succeed())
				Expect(store.SoftDeletePage("expired-page")).To(Succeed())

				err := store.PurgeExpiredTrash(time.Now().UTC().Add(31 * 24 * time.Hour))
				Expect(err).NotTo(HaveOccurred())
				trashEntries, err = store.ListTrash()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the trash entry", func() {
				Expect(trashEntries).To(BeEmpty())
			})
		})

		When("one expired trash entry has corrupted metadata", func() {
			var purgeErr error

			BeforeEach(func() {
				brokenDir := filepath.Join(tempDir, deletedDirName, "broken")
				Expect(os.MkdirAll(brokenDir, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(brokenDir, trashMetadataName), []byte("identifier = [broken\n"), 0644)).To(Succeed())
				purgeErr = store.PurgeExpiredTrash(time.Now().UTC().Add(31 * 24 * time.Hour))
			})

			It("should not return an error", func() {
				Expect(purgeErr).NotTo(HaveOccurred())
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
