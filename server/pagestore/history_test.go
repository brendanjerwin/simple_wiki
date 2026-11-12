//revive:disable:dot-imports
package pagestore_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pagestore history", func() {
	var (
		tmpDir string
		store  *pagestore.Store
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "pagestore-history-test")
		Expect(err).NotTo(HaveOccurred())
		store = pagestore.NewStore(tmpDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("capture via ModifyOrCreatePage", func() {
		When("a page is written for the first time", func() {
			BeforeEach(func() {
				err := store.WriteMarkdown("test-page", "# Hello", wikipage.AnonymousIdentity)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not capture a history version (no prior content)", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(BeEmpty())
			})
		})

		When("a page is modified", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Version 1", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Version 2", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Version 3", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should capture a version for each change", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(2))
			})

			It("should list versions newest-first", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions[0].VersionID > versions[1].VersionID).To(BeTrue())
			})

			It("should record the correct source", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions[0].Source).To(Equal("write_markdown"))
			})

			It("should record the content hash", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions[0].SHA256).NotTo(BeEmpty())
				Expect(versions[0].ByteSize).To(BeNumerically(">", 0))
			})
		})

		When("a page is written with identical content (no-op)", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Same content", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Same content", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should not capture a version for the no-op write", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(BeEmpty())
			})
		})

		When("WriteFrontMatter is used", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Body", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteFrontMatter("test-page", wikipage.FrontMatter{"title": "New Title"}, wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should capture with source write_frontmatter", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(1))
				Expect(versions[0].Source).To(Equal("write_frontmatter"))
			})
		})

		When("ModifyMarkdown is used", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Body", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.ModifyMarkdown("test-page", func(_ wikipage.FrontMatter, md wikipage.Markdown) (wikipage.Markdown, error) {
					return md + " modified", nil
				}, wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should capture with source modify_markdown", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(1))
				Expect(versions[0].Source).To(Equal("modify_markdown"))
			})
		})
	})

	Describe("ReadVersion", func() {
		When("a version exists", func() {
			var versionID string

			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Original content", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Changed content", wikipage.AnonymousIdentity)).To(Succeed())
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(1))
				versionID = versions[0].VersionID
			})

			It("should return the content of that version", func() {
				content, err := store.ReadVersion("test-page", versionID)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(ContainSubstring("# Original content"))
			})
		})

		When("the version does not exist", func() {
			It("should return an error", func() {
				_, err := store.ReadVersion("test-page", "nonexistent-version-id")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("RestoreVersion", func() {
		When("restoring a prior version", func() {
			var originalVersionID string

			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Original", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Changed", wikipage.AnonymousIdentity)).To(Succeed())
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				originalVersionID = versions[0].VersionID
			})

			It("should restore the historical content as live", func() {
				Expect(store.RestoreVersion("test-page", originalVersionID, wikipage.AnonymousIdentity)).To(Succeed())

				// Read the live page
				content, err := store.ReadPage("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(content.Text).To(ContainSubstring("# Original"))
			})

			It("should capture the current live content as a new version before restoring", func() {
				Expect(store.RestoreVersion("test-page", originalVersionID, wikipage.AnonymousIdentity)).To(Succeed())

				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				// Should now have: original version + the version captured during restore
				Expect(versions).To(HaveLen(2))
				// One of the versions should have source "restore" (the capture from the restore)
				sources := []string{versions[0].Source, versions[1].Source}
				Expect(sources).To(ContainElement("restore"))
			})
		})
	})

	Describe("DiffVersions", func() {
		When("diffing two versions", func() {
			var oldID, newID string

			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Title\n\nLine 1\nLine 2\n", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Title\n\nLine 1 modified\nLine 2\nLine 3\n", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# Title\n\nLine 1 modified\nLine 2\nLine 3\nLine 4\n", wikipage.AnonymousIdentity)).To(Succeed())

				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(2))
				// versions[0] is newest (version 3's prior), versions[1] is oldest (version 1)
				newID = versions[0].VersionID
				oldID = versions[1].VersionID
			})

			It("should return a unified diff with @@ markers", func() {
				diff, err := store.DiffVersions("test-page", oldID, newID)
				Expect(err).NotTo(HaveOccurred())
				Expect(diff).To(ContainSubstring("@@"))
			})

			It("should show removed and added lines", func() {
				diff, err := store.DiffVersions("test-page", oldID, newID)
				Expect(err).NotTo(HaveOccurred())
				Expect(diff).To(ContainSubstring("-Line 1"))
				Expect(diff).To(ContainSubstring("+Line 1 modified"))
			})
		})
	})

	Describe("ListVersions", func() {
		When("the page has no history", func() {
			It("should return an empty slice", func() {
				versions, err := store.ListVersions("nonexistent-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(BeEmpty())
			})
		})

		When("the page has history", func() {
			BeforeEach(func() {
				for i := range 5 {
					Expect(store.WriteMarkdown("test-page", wikipage.Markdown("# Version "+string(rune('0'+i))), wikipage.AnonymousIdentity)).To(Succeed())
				}
			})

			It("should return all versions sorted newest-first", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(4))

				for i := 1; i < len(versions); i++ {
					Expect(versions[i-1].VersionID > versions[i].VersionID).To(BeTrue())
				}
			})

			It("should include metadata fields", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions[0].VersionID).NotTo(BeEmpty())
				Expect(versions[0].CreatedAt).NotTo(BeZero())
				Expect(versions[0].SHA256).NotTo(BeEmpty())
				Expect(versions[0].ByteSize).To(BeNumerically(">", 0))
			})
		})
		When("many versions are captured", func() {
			BeforeEach(func() {
				for i := range 10 {
					Expect(store.WriteMarkdown("test-page", wikipage.Markdown("# V"+string(rune('0'+i))), wikipage.AnonymousIdentity)).To(Succeed())
				}
			})

			It("should return all captured versions (one less than write count — first write has no prior)", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(9)) // 10 writes, 9 captures (first write has no prior)
			})
		})
	})

	Describe("history on disk", func() {
		When("a version is captured", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("test-page", "# Content", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("test-page", "# New content", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should create __history__ directory", func() {
				historyDir := filepath.Join(tmpDir, "__history__")
				info, err := os.Stat(historyDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.IsDir()).To(BeTrue())
			})

			It("should create .md and .meta.json files for each version", func() {
				versions, err := store.ListVersions("test-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(1))

				historyDir := filepath.Join(tmpDir, "__history__")
				entries, err := os.ReadDir(historyDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))

				pageDir := filepath.Join(historyDir, entries[0].Name())
				mdFile := filepath.Join(pageDir, versions[0].VersionID+".md")
				metaFile := filepath.Join(pageDir, versions[0].VersionID+".meta.json")
				Expect(mdFile).To(BeAnExistingFile())
				Expect(metaFile).To(BeAnExistingFile())
			})
		})
	})

	Describe("history opt-out", func() {
		When("a page has wiki.history.opt_out = true in frontmatter", func() {
			BeforeEach(func() {
				// Write a page with the opt-out flag, then modify it
				Expect(store.WriteFrontMatter("metrics-page", wikipage.FrontMatter{
					"identifier": "metrics-page",
					"wiki": map[string]any{
						"history": map[string]any{
							"opt_out": true,
						},
					},
				}, wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("metrics-page", "# Metrics v1", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("metrics-page", "# Metrics v2", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should not capture any history versions", func() {
				versions, err := store.ListVersions("metrics-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(BeEmpty())
			})
		})

		When("a page does not have the opt-out flag", func() {
			BeforeEach(func() {
				Expect(store.WriteMarkdown("normal-page", "# V1", wikipage.AnonymousIdentity)).To(Succeed())
				Expect(store.WriteMarkdown("normal-page", "# V2", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should capture history versions normally", func() {
				versions, err := store.ListVersions("normal-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).To(HaveLen(1))
			})
		})

		// Regression guard for the shouldSkipHistoryCapture fast path (#1409).
		// A large checklist page (no opt_out key anywhere in its content) must
		// still capture history on write even after the fast-path optimisation.
		When("a large checklist page without opt_out is modified", func() {
			BeforeEach(func() {
				// Build realistic checklist TOML: 50 items + 200 events, totalling
				// tens of KB. The key property is that neither "opt_out" nor the
				// [wiki.history] section appear anywhere in the content, so the
				// fast-path string search short-circuits without a TOML parse and
				// must still return false (i.e. capture history).
				fm := wikipage.FrontMatter{
					"wiki": map[string]any{
						"checklists": map[string]any{
							"Groceries": map[string]any{
								"sync_token": int64(200),
								"max_seq":    int64(200),
								"updated_at": "2026-01-01T00:00:00Z",
								"items": func() []any {
									out := make([]any, 50)
									for i := range 50 {
										out[i] = map[string]any{
											"uid":        fmt.Sprintf("01JTEST%08X", i),
											"text":       fmt.Sprintf("Item %d", i),
											"checked":    false,
											"sort_order": int64((i + 1) * 1000),
										}
									}
									return out
								}(),
								"events": func() []any {
									out := make([]any, 200)
									for i := range 200 {
										out[i] = map[string]any{
											"seq": int64(i + 1),
											"src": "user:alice@example.com",
											"op":  "add",
											"uid": fmt.Sprintf("01JTEST%08X", i%50),
										}
									}
									return out
								}(),
							},
						},
					},
				}
				pageText, err := wikipage.CombineFrontMatterAndMarkdown(fm, "# Grocery list\n")
				Expect(err).NotTo(HaveOccurred())

				// First write: establish the page
				Expect(store.ModifyOrCreatePage("checklist-page", wikipage.AnonymousIdentity, "setup", func(_ string) (string, error) {
					return pageText, nil
				})).To(Succeed())

				// Second write: modify it so a history entry is captured
				Expect(store.WriteMarkdown("checklist-page", "# Updated grocery list\n", wikipage.AnonymousIdentity)).To(Succeed())
			})

			It("should capture a history version for the large checklist page", func() {
				versions, err := store.ListVersions("checklist-page")
				Expect(err).NotTo(HaveOccurred())
				Expect(versions).NotTo(BeEmpty())
			})
		})
	})
})
