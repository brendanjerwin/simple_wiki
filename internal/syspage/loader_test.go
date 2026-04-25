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

var _ = Describe("IsSystemPage", func() {
	Describe("when frontmatter has system = true", func() {
		It("should return true", func() {
			Expect(IsSystemPage(map[string]any{"system": true})).To(BeTrue())
		})
	})

	Describe("when frontmatter has system = false", func() {
		It("should return false", func() {
			Expect(IsSystemPage(map[string]any{"system": false})).To(BeFalse())
		})
	})

	Describe("when frontmatter omits system", func() {
		It("should return false", func() {
			Expect(IsSystemPage(map[string]any{"title": "x"})).To(BeFalse())
		})
	})

	Describe("when frontmatter is nil", func() {
		It("should return false", func() {
			Expect(IsSystemPage(nil)).To(BeFalse())
		})
	})

	Describe("when system value is the string \"true\"", func() {
		It("should return true (TOML coercion friendliness)", func() {
			Expect(IsSystemPage(map[string]any{"system": "true"})).To(BeTrue())
		})
	})
})

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
			Expect(IsSystemPage(p.Frontmatter)).To(BeTrue(),
				fmt.Sprintf("page %q should have system = true", p.Identifier))
		}
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
			store     *fakeStore
			pages     []Page
			id        wikipage.PageIdentifier
			writesPre int
			writes    int
		)

		BeforeEach(func() {
			pages, _ = LoadEmbedded()
			Expect(pages).NotTo(BeEmpty())

			store = newFakeStore()
			Expect(Sync(store, nullLogger{})).To(Succeed())

			// Simulate user/admin drift: replace the markdown of the first
			// embedded page with something that does not match the embedded
			// source.
			id = wikipage.PageIdentifier(pages[0].Identifier)
			store.markdown[id] = wikipage.Markdown("DRIFTED CONTENT")

			writesPre = totalWrites(store)
			Expect(Sync(store, nullLogger{})).To(Succeed())
			writes = totalWrites(store) - writesPre
		})

		It("should rewrite the drifted page", func() {
			Expect(string(store.markdown[id])).NotTo(Equal("DRIFTED CONTENT"))
		})

		It("should perform writes only for the drifted page", func() {
			// At least one write for the drifted page; allow the implementation
			// to call WriteFrontMatter and WriteMarkdown separately, but not to
			// rewrite untouched pages.
			Expect(writes).To(BeNumerically(">=", 1))
			Expect(writes).To(BeNumerically("<=", 2))
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

