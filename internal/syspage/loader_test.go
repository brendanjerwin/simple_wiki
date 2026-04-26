//revive:disable:dot-imports
package syspage

import (
	"errors"
	"fmt"
	"os"
	"testing"

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

func (f *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.frontmatter[id] = fm
	f.writes[id]++
	return nil
}

func (f *fakeStore) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	f.markdown[id] = md
	f.writes[id]++
	return nil
}

func (f *fakeStore) DeletePage(id wikipage.PageIdentifier) error {
	delete(f.frontmatter, id)
	delete(f.markdown, id)
	return nil
}

func (f *fakeStore) ModifyMarkdown(id wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	current := f.markdown[id]
	next, err := modifier(current)
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
			store           *fakeStore
			pages           []Page
			driftedID       wikipage.PageIdentifier
			driftedWrites   int
			otherIDs        []wikipage.PageIdentifier
			otherIDWrites   map[wikipage.PageIdentifier]int
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

func (*explodingStore) WriteFrontMatter(_ wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	return nil
}

func (*explodingStore) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}

func (*explodingStore) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (*explodingStore) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

