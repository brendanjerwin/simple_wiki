//revive:disable:dot-imports
package syspage

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSyspage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syspage Suite")
}

// fakeStore is an in-memory wikipage.PageReaderMutator used by Sync tests.
type fakeStore struct {
	frontmatter map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown    map[wikipage.PageIdentifier]wikipage.Markdown
	writes      map[wikipage.PageIdentifier]int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		frontmatter: map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		markdown:    map[wikipage.PageIdentifier]wikipage.Markdown{},
		writes:      map[wikipage.PageIdentifier]int{},
	}
}

func (f *fakeStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	fm, ok := f.frontmatter[id]
	if !ok {
		// Mirror the real store: reads resolve the munged request back to
		// the raw-keyed entry when present.
		if munged, mungeErr := wikiidentifiers.MungeIdentifier(string(id)); mungeErr == nil {
			if munged != string(id) {
				if m2, ok2 := f.frontmatter[wikipage.PageIdentifier(munged)]; ok2 {
					return wikipage.PageIdentifier(munged), m2, nil
				}
			}
			// Reverse: requested munged, stored raw.
			for rawID, m2 := range f.frontmatter {
				m3, err3 := wikiidentifiers.MungeIdentifier(string(rawID))
				if err3 == nil && m3 == string(id) {
					return rawID, m2, nil
				}
			}
		}
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	md, ok := f.markdown[id]
	if !ok {
		return id, "", os.ErrNotExist
	}
	return id, md, nil
}

func (f *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter, _ wikipage.Identity) error {
	f.frontmatter[id] = fm
	f.writes[id]++
	return nil
}

func (f *fakeStore) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown, _ wikipage.Identity) error {
	f.markdown[id] = md
	f.writes[id]++
	return nil
}

func (f *fakeStore) DeletePage(id wikipage.PageIdentifier) error {
	// Mirror the real store: delete resolves the munged request to the
	// raw-keyed entry.
	munged, err := wikiidentifiers.MungeIdentifier(string(id))
	if err == nil {
		for rawID := range f.frontmatter {
			if m2, err2 := wikiidentifiers.MungeIdentifier(string(rawID)); err2 == nil && m2 == munged {
				id = rawID
				break
			}
		}
	}
	delete(f.frontmatter, id)
	delete(f.markdown, id)
	return nil
}

// ListAllPageIdentifiers enumerates the union of pages the fake knows about,
// returned as the MUNGED identifier (mirrors the real store listing by
// munged filename). Storing a page called "help-page-apps" therefore lists
// back as "help_page_apps" — exactly the rename-trap this file guards.
func (f *fakeStore) ListAllPageIdentifiers() ([]wikipage.PageIdentifier, error) {
	seen := map[wikipage.PageIdentifier]struct{}{}
	for id := range f.frontmatter {
		seen[id] = struct{}{}
	}
	for id := range f.markdown {
		seen[id] = struct{}{}
	}
	ids := make([]wikipage.PageIdentifier, 0, len(seen))
	for id := range seen {
		munged, err := wikiidentifiers.MungeIdentifier(string(id))
		if err != nil {
			// Fall back to the raw id if munging fails, as the real store
			// falls back to a legacy unmunged path.
			munged = string(id)
		}
		ids = append(ids, wikipage.PageIdentifier(munged))
	}
	return ids, nil
}

func (f *fakeStore) ModifyMarkdown(id wikipage.PageIdentifier, modifier func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.Markdown, error), _ wikipage.Identity) error {
	current := f.markdown[id]
	next, err := modifier(nil, current)
	if err != nil {
		return err
	}
	f.markdown[id] = next
	f.writes[id]++
	return nil
}

// nullLogger satisfies the syspage.Logger interface but discards messages.
type nullLogger struct{}

func (nullLogger) Info(_ string, _ ...any)  {}
func (nullLogger) Debug(_ string, _ ...any) {}

type emptyFrontmatterIndex struct{}

func (emptyFrontmatterIndex) QueryExactMatch(wikipage.DottedKeyPath, wikipage.Value) []wikipage.PageIdentifier {
	return nil
}

func (emptyFrontmatterIndex) QueryKeyExistence(wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return nil
}

func (emptyFrontmatterIndex) QueryPrefixMatch(wikipage.DottedKeyPath, string) []wikipage.PageIdentifier {
	return nil
}

func (emptyFrontmatterIndex) QueryExactMatchSortedBy(
	wikipage.DottedKeyPath,
	wikipage.Value,
	wikipage.DottedKeyPath,
	bool,
	int,
) []wikipage.PageIdentifier {
	return nil
}

func (emptyFrontmatterIndex) GetValue(wikipage.PageIdentifier, wikipage.DottedKeyPath) wikipage.Value {
	return ""
}

var _ = Describe("LoadEmbedded", func() {
	var (
		pages []Page
		err   error
	)

	BeforeEach(func() {
		pages, err = LoadEmbedded()
	})

	It("should not return an error", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return at least one page", func() {
		Expect(pages).NotTo(BeEmpty())
	})

	It("should return pages with non-empty identifiers", func() {
		for _, p := range pages {
			Expect(p.Identifier).NotTo(BeEmpty(), "page identifier should be non-empty")
		}
	})

	It("should mark every embedded page as a system page", func() {
		for _, p := range pages {
			Expect(wikipage.IsSystemPage(p.Frontmatter)).To(BeTrue(),
				fmt.Sprintf("page %q should have wiki.system = true", p.Identifier))
		}
	})

	Describe("the shipped profile_template page", func() {
		var profileTemplate *Page

		BeforeEach(func() {
			for i := range pages {
				if pages[i].Identifier == "profile_template" {
					profileTemplate = &pages[i]
					break
				}
			}
		})

		It("should be present in the embedded corpus", func() {
			Expect(profileTemplate).NotTo(BeNil(), "profile_template.md should ship in internal/syspage/embedded/")
		})

		It("should be marked as a template", func() {
			Expect(wikipage.IsTemplatePage(profileTemplate.Frontmatter)).To(BeTrue(),
				"profile_template should have wiki.template = true")
		})

		It("should be marked as a system page", func() {
			Expect(wikipage.IsSystemPage(profileTemplate.Frontmatter)).To(BeTrue(),
				"profile_template should have wiki.system = true")
		})
	})

	Describe("the handling large pages help page", func() {
		var helpPage *Page

		BeforeEach(func() {
			helpPage = nil
			for i := range pages {
				if pages[i].Identifier == "help_handling_large_pages" {
					helpPage = &pages[i]
					break
				}
			}
			Expect(helpPage).NotTo(BeNil(), "help_handling_large_pages.md should ship in internal/syspage/embedded/")
		})

		It("should be present in the embedded corpus", func() {
			Expect(helpPage.Identifier).To(Equal("help_handling_large_pages"))
		})

		It("should cover outline-first navigation", func() {
			Expect(helpPage.Markdown).To(ContainSubstring("api_v1_PageManagementService_ReadPageOutline"))
			Expect(helpPage.Markdown).To(ContainSubstring("api_v1_PageManagementService_ReadPageSection"))
			Expect(helpPage.Markdown).To(ContainSubstring("api_v1_PageManagementService_ReadPage"))
			Expect(helpPage.Markdown).To(ContainSubstring("does not accept byte offset or length fields"))
		})

		It("should cover stale tool recovery", func() {
			Expect(helpPage.Markdown).To(ContainSubstring("MCP tools are negotiated when the agent session starts"))
			Expect(helpPage.Markdown).To(ContainSubstring("restart the wiki process that launches page agents"))
		})

		It("should cover splitting and trimming", func() {
			Expect(helpPage.Markdown).To(ContainSubstring("Trim Strategies"))
			Expect(helpPage.Markdown).To(ContainSubstring("Splitting Strategies"))
		})
	})

	Describe("the map macro help page", func() {
		var (
			helpPage         *Page
			renderedMarkdown string
			renderErr        error
		)

		BeforeEach(func() {
			helpPage = nil
			for i := range pages {
				if pages[i].Identifier == "help_macro_map" {
					helpPage = &pages[i]
					break
				}
			}
			Expect(helpPage).NotTo(BeNil(), "help_macro_map.md should ship in internal/syspage/embedded/")

			var renderedBytes []byte
			renderedBytes, renderErr = templating.ExecuteTemplate(
				helpPage.Markdown,
				helpPage.Frontmatter,
				newFakeStore(),
				emptyFrontmatterIndex{},
			)
			renderedMarkdown = string(renderedBytes)
		})

		It("should render without template errors", func() {
			Expect(renderErr).NotTo(HaveOccurred())
		})

		It("should show the Map macro syntax in Go template format", func() {
			Expect(renderedMarkdown).To(ContainSubstring(`{{ Map "map-name" }}`))
			Expect(renderedMarkdown).To(ContainSubstring(`{{ Map "yard" }}`))
		})

		It("should only execute the live demo map macro", func() {
			Expect(strings.Count(renderedMarkdown, `page="help_macro_map"`)).To(Equal(1))
			Expect(renderedMarkdown).To(ContainSubstring(`<wiki-map name="demo" page="help_macro_map"></wiki-map>`))
		})
	})
})

var _ = Describe("Sync", func() {
	Describe("when invoked twice with no source change", func() {
		var (
			store  *fakeStore
			err1   error
			err2   error
			writes int
		)

		BeforeEach(func() {
			store = newFakeStore()
			err1 = Sync(store, nullLogger{})
			Expect(err1).NotTo(HaveOccurred())

			// Reset write counter so we measure only the second sync.
			startWrites := totalWrites(store)
			err2 = Sync(store, nullLogger{})
			writes = totalWrites(store) - startWrites
		})

		It("should succeed both times", func() {
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should perform zero writes the second time (idempotent)", func() {
			Expect(writes).To(Equal(0))
		})
	})

	Describe("when on-disk content has drifted", func() {
		var (
			store         *fakeStore
			pages         []Page
			driftedID     wikipage.PageIdentifier
			driftedWrites int
			otherIDs      []wikipage.PageIdentifier
			otherIDWrites map[wikipage.PageIdentifier]int
		)

		BeforeEach(func() {
			pages, _ = LoadEmbedded()
			Expect(pages).NotTo(BeEmpty())

			store = newFakeStore()
			Expect(Sync(store, nullLogger{})).To(Succeed())

			// Simulate user/admin drift: replace the markdown of the first
			// embedded page with something that does not match the embedded
			// source.
			driftedID = wikipage.PageIdentifier(pages[0].Identifier)
			store.markdown[driftedID] = wikipage.Markdown("DRIFTED CONTENT")

			// Snapshot per-page write counts so we can subtract first-Sync
			// writes from second-Sync writes per page.
			writesPre := map[wikipage.PageIdentifier]int{}
			for k, v := range store.writes {
				writesPre[k] = v
			}

			Expect(Sync(store, nullLogger{})).To(Succeed())

			driftedWrites = store.writes[driftedID] - writesPre[driftedID]

			otherIDs = nil
			otherIDWrites = map[wikipage.PageIdentifier]int{}
			for _, p := range pages {
				other := wikipage.PageIdentifier(p.Identifier)
				if other == driftedID {
					continue
				}
				otherIDs = append(otherIDs, other)
				otherIDWrites[other] = store.writes[other] - writesPre[other]
			}
		})

		It("should rewrite the drifted page", func() {
			Expect(string(store.markdown[driftedID])).NotTo(Equal("DRIFTED CONTENT"))
		})

		It("should sync the drifted page exactly once", func() {
			// A single logical Sync may emit one WriteFrontMatter and one
			// WriteMarkdown — two physical writes per logical sync. We accept
			// 1 (markdown-only update) or 2 (frontmatter + markdown) but not
			// more, which would indicate the page was synced more than once.
			Expect(driftedWrites).To(BeNumerically(">=", 1),
				"drifted page should receive at least one write")
			Expect(driftedWrites).To(BeNumerically("<=", 2),
				"drifted page should receive at most one logical sync (two physical writes: frontmatter + markdown)")
		})

		It("should not rewrite any non-drifted page", func() {
			for _, other := range otherIDs {
				Expect(otherIDWrites[other]).To(Equal(0),
					fmt.Sprintf("page %q should not have been rewritten on the second sync", other))
			}
		})
	})

	Describe("when the store reports a non-NotExist read error", func() {
		It("should propagate the error", func() {
			store := &explodingStore{err: errors.New("disk on fire")}
			err := Sync(store, nullLogger{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("disk on fire"))
		})
	})

	Describe("when a system page on disk is no longer in the embedded corpus", func() {
		var store *fakeStore

		BeforeEach(func() {
			store = newFakeStore()
			Expect(Sync(store, nullLogger{})).To(Succeed())

			// Simulate the retired help-artifacts-lang page: shipped by an
			// older binary, renamed since. The sync must remove it so agents
			// cannot find stale instructions naming dead tools.
			staleID := wikipage.PageIdentifier("help-artifacts-lang")
			store.frontmatter[staleID] = wikipage.FrontMatter{
				"identifier": "help-artifacts-lang",
				"wiki":       map[string]any{"system": true},
			}
			store.markdown[staleID] = wikipage.Markdown("stale help text")

			// A user page with no system flag must survive.
			userID := wikipage.PageIdentifier("regular_user_page")
			store.frontmatter[userID] = wikipage.FrontMatter{"identifier": "regular_user_page"}
			store.markdown[userID] = wikipage.Markdown("user content")

			Expect(Sync(store, nullLogger{})).To(Succeed())
		})

		It("should delete the retired system page", func() {
			_, _, err := store.ReadFrontMatter(wikipage.PageIdentifier("help-artifacts-lang"))
			Expect(err).To(MatchError(os.ErrNotExist))
		})

		It("should not prune hyphenated embedded pages (munged keep-set)", func() {
			// Regression: the store enumerates by munged filename and
			// MungeIdentifier snake-cases hyphens, so `help-page-apps` is
			// listed as `help_page_apps`. The keep-set must contain the
			// munged form or the prune step deletes the page the sync just
			// wrote.
			pages, _ := LoadEmbedded()
			for _, p := range pages {
				if !strings.Contains(p.Identifier, "-") {
					continue
				}
				munged, err := wikiidentifiers.MungeIdentifier(p.Identifier)
				Expect(err).NotTo(HaveOccurred())
				_, _, readErr := store.ReadFrontMatter(wikipage.PageIdentifier(munged))
				Expect(readErr).NotTo(HaveOccurred(),
					fmt.Sprintf("hyphenated embedded page %q (listed as munged %q) must not be pruned", p.Identifier, munged))
			}
		})

		It("should not delete user pages", func() {
			_, _, err := store.ReadFrontMatter(wikipage.PageIdentifier("regular_user_page"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should leave all embedded pages intact", func() {
			pages, loadErr := LoadEmbedded()
			Expect(loadErr).NotTo(HaveOccurred())
			for _, p := range pages {
				_, _, readErr := store.ReadFrontMatter(wikipage.PageIdentifier(p.Identifier))
				Expect(readErr).NotTo(HaveOccurred(),
					fmt.Sprintf("embedded page %q must not be pruned", p.Identifier))
			}
		})
	})
})

func totalWrites(s *fakeStore) int {
	total := 0
	for _, n := range s.writes {
		total += n
	}
	return total
}

// explodingStore is a PageReaderMutator that always returns a non-NotExist read
// error from ReadFrontMatter, used to exercise Sync's error propagation.
type explodingStore struct {
	err error
}

func (e *explodingStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return id, nil, e.err
}

func (*explodingStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", os.ErrNotExist
}

func (*explodingStore) WriteFrontMatter(_ wikipage.PageIdentifier, _ wikipage.FrontMatter, _ wikipage.Identity) error {
	return nil
}

func (*explodingStore) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown, _ wikipage.Identity) error {
	return nil
}

func (*explodingStore) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (*explodingStore) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.Markdown, error), _ wikipage.Identity) error {
	return nil
}
