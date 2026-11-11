//revive:disable:dot-imports
package checklistmutator_test

import (
	"context"
	"sync"
	"testing"

	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// interleavingStore reproduces the lost-update window from #1411 without any
// goroutine scheduling luck: a page-level writer commits an unrelated
// frontmatter key immediately *after* a checklist read returns its snapshot.
//
// WriteFrontMatter deliberately mimics the real store — it re-reads only the
// markdown under its lock and takes the caller's frontmatter verbatim — so a
// caller that writes back a snapshot taken before the page-level write
// silently reverts that key.
//
// ModifyFrontMatterAndMarkdown mimics the atomic path: read current
// frontmatter, apply the callback, store the result, all under one lock.
type interleavingStore struct {
	mu       sync.Mutex
	fm       wikipage.FrontMatter
	markdown wikipage.Markdown
	injected bool
}

func newInterleavingStore() *interleavingStore {
	return &interleavingStore{fm: wikipage.FrontMatter{}}
}

// injectConcurrentWrite commits a key that the checklist mutator must not
// disturb. Called from ReadFrontMatter so the write provably lands between the
// mutator's read and its write-back.
func (s *interleavingStore) injectConcurrentWrite() {
	if s.injected {
		return
	}
	s.injected = true
	s.fm["inventory"] = map[string]any{"sku": "publix-bogo-1"}
}

func (s *interleavingStore) ReadFrontMatter(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := deepCopyFM(s.fm)
	// The concurrent page-level writer wins the race right here, after the
	// snapshot has been taken and before the checklist write-back happens.
	s.injectConcurrentWrite()
	return "", snapshot, nil
}

func (s *interleavingStore) WriteFrontMatter(_ wikipage.PageIdentifier, fm wikipage.FrontMatter, _ wikipage.Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fm = deepCopyFM(fm)
	return nil
}

func (s *interleavingStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return id, s.markdown, nil
}

func (s *interleavingStore) WriteMarkdown(_ wikipage.PageIdentifier, md wikipage.Markdown, _ wikipage.Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markdown = md
	return nil
}

func (*interleavingStore) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (s *interleavingStore) ModifyMarkdown(id wikipage.PageIdentifier, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.Markdown, error), _ wikipage.Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := fn(deepCopyFM(s.fm), s.markdown)
	if err != nil {
		return err
	}
	s.markdown = next
	return nil
}

func (s *interleavingStore) ModifyFrontMatterAndMarkdown(_ wikipage.PageIdentifier, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error), _ wikipage.Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	nextFM, nextMD, err := fn(deepCopyFM(s.fm), s.markdown)
	if err != nil {
		return err
	}
	s.fm = deepCopyFM(nextFM)
	s.markdown = nextMD
	return nil
}

// TestChecklistMutationPreservesConcurrentFrontmatterKey is the regression test
// for #1411: a checklist write must not revert a frontmatter key that another
// writer committed while the checklist mutation was in flight.
func TestChecklistMutationPreservesConcurrentFrontmatterKey(t *testing.T) {
	store := newInterleavingStore()
	mutator := checklistmutator.New(
		store,
		checklistmutator.SystemClock{},
		ulid.NewSystemGenerator(),
	)
	human := tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")

	if _, _, err := mutator.AddItem(context.Background(), "shopping_lists", "Grocery",
		checklistmutator.AddItemArgs{Text: "Buy milk"}, human); err != nil {
		t.Fatalf("AddItem returned error: %v", err)
	}

	if _, ok := store.fm["inventory"]; !ok {
		t.Fatalf("concurrent frontmatter key was reverted by the checklist write: frontmatter = %#v", store.fm)
	}
	if _, ok := store.fm["wiki"]; !ok {
		t.Fatalf("checklist write did not land: frontmatter = %#v", store.fm)
	}
}
