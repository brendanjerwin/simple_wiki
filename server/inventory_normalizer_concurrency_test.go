//revive:disable:dot-imports
package server_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jcelliott/lumber"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pausingNormalizerStore wraps a real pagestore.Store and intercepts
// ReadFrontMatter to pause after the read returns. This lets a test land a
// concurrent write inside the read→write window that the broken
// ensureIsContainerField left open.
//
// A fired flag ensures the pause fires at most once, so a second
// ReadFrontMatter call (from GetContainerItems in NormalizePage) does not
// deadlock the test.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed code path goes directly to the page lock without
// pausing — proving the race window is closed.
type pausingNormalizerStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	fired      atomic.Bool // only pause the first ReadFrontMatter on targetPage
	paused     chan struct{}
	release    chan struct{}
}

// DeletePage and ReadMarkdown are not used by the stores under test; the stubs
// keep the type satisfying wikipage.PageReaderMutator (required by
// checklistmutator.New and NewInventoryNormalizer).
func (*pausingNormalizerStore) DeletePage(wikipage.PageIdentifier) error { return nil }

func (*pausingNormalizerStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (s *pausingNormalizerStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() && !s.fired.Swap(true) {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("InventoryNormalizer concurrency", func() {
	// Regression test for #1428: ensureIsContainerField used an unlocked
	// ReadFrontMatter → mutate in memory → WriteFrontMatter sequence.
	// Any concurrent writer (checklist AddItem, mapmutator, etc.) that committed
	// between the read and the write-back would be silently reverted.
	//
	// The fix migrates ensureIsContainerField onto ModifyFrontMatterAndMarkdown,
	// which runs the read, mutation, and write-back under the page-canonical lock.
	const page = wikipage.PageIdentifier("normalizer_race_page")

	var (
		ctx           context.Context
		pausingStore  *pausingNormalizerStore
		checklist     *checklistmutator.Mutator
		human         = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		appendedTexts []string
		list          *apiv1.Checklist
		listErr       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		appendedTexts = nil

		pausingStore = &pausingNormalizerStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: page,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		checklist = checklistmutator.New(pausingStore, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())

		// Seed the page with an inventory.items array so ensureIsContainerField
		// will try to set is_container = true. Also add a checklist seed item so
		// we can detect whether the FM write-back overwrites concurrent checklist
		// mutations.
		Expect(pausingStore.Store.WriteFrontMatter(page, wikipage.FrontMatter{
			"inventory": map[string]any{
				"items": []any{"widget"},
			},
		}, wikipage.AnonymousIdentity)).To(Succeed())
		_, _, seedErr := checklist.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: "seed"}, human)
		Expect(seedErr).NotTo(HaveOccurred())
	})

	Describe("when NormalizePage races a concurrent AddItem", func() {
		BeforeEach(func() {
			logger := lumber.NewConsoleLogger(lumber.WARN)
			normalizer := server.NewInventoryNormalizer(pausingStore, logger)

			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = normalizer.NormalizePage(page)
			}()

			addOne := func() {
				pausingStore.armed.Store(false)
				text := fmt.Sprintf("concurrent-%d", len(appendedTexts))
				_, _, addErr := checklist.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: text}, human)
				Expect(addErr).NotTo(HaveOccurred())
				appendedTexts = append(appendedTexts, text)
				pausingStore.armed.Store(true)
			}

			// If a pause fires, the broken code is in its read→write window.
			// Land an AddItem inside that window, then release.
			// If no pause fires, the fixed code uses ModifyFrontMatterAndMarkdown
			// and closes the window completely.
			select {
			case <-pausingStore.paused:
				addOne()
				pausingStore.release <- struct{}{}
				<-done
			case <-done:
				// Fixed code path: no window, goroutine completed directly.
			}
			pausingStore.armed.Store(false)

			// After the fix no pause fires in ensureIsContainerField, so the first
			// pause (if any) fires in GetContainerItems. Either way, add at least
			// one concurrent item so the assertion is non-trivial.
			if len(appendedTexts) == 0 {
				addOne()
			}
			pausingStore.armed.Store(false)

			list, listErr = checklist.ListItems(ctx, string(page), "todos")
		})

		It("should keep the seed checklist item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElement("seed"))
		})

		It("should keep every concurrently added checklist item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElements(appendedTexts))
		})
	})
})
