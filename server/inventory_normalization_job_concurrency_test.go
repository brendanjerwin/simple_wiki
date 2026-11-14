//revive:disable:dot-imports
package server_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jcelliott/lumber"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pausingJobStore wraps a real pagestore.Store and intercepts ReadFrontMatter
// to pause after the read returns. This lets a test land a concurrent write
// inside the read→write window that the broken migrateContainerIfNeeded left open.
//
// A fired flag ensures the pause fires at most once, so a second ReadFrontMatter
// call (from GetContainerItems in createMissingItemPages) does not deadlock.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed code path goes directly to the page lock without
// pausing — proving the race window is closed.
type pausingJobStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	fired      atomic.Bool // only pause the first ReadFrontMatter on targetPage
	paused     chan struct{}
	release    chan struct{}
}

// DeletePage and ReadMarkdown are not used by the stores under test; the stubs
// keep the type satisfying wikipage.PageReaderMutator.
func (*pausingJobStore) DeletePage(wikipage.PageIdentifier) error { return nil }

func (*pausingJobStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (s *pausingJobStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() && !s.fired.Swap(true) {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

// jobConcurrencyFmIndex is a minimal IQueryFrontmatterIndex for the job
// concurrency test. It reports the container page as having inventory.items,
// which causes Execute to migrate it and scan for missing item pages.
type jobConcurrencyFmIndex struct {
	containerPage wikipage.PageIdentifier
}

func (*jobConcurrencyFmIndex) QueryExactMatch(key, value string) []wikipage.PageIdentifier {
	return nil
}

func (m *jobConcurrencyFmIndex) QueryKeyExistence(key string) []wikipage.PageIdentifier {
	if key == "inventory.items" {
		return []wikipage.PageIdentifier{m.containerPage}
	}
	return nil
}

func (*jobConcurrencyFmIndex) QueryPrefixMatch(key, prefix string) []wikipage.PageIdentifier {
	return nil
}

func (*jobConcurrencyFmIndex) QueryExactMatchSortedBy(matchKey, matchValue, sortByKey string, ascending bool, maxResults int) []wikipage.PageIdentifier {
	return nil
}

func (*jobConcurrencyFmIndex) GetValue(id wikipage.PageIdentifier, key string) string {
	return ""
}

var _ frontmatter.IQueryFrontmatterIndex = (*jobConcurrencyFmIndex)(nil)

var _ = Describe("InventoryNormalizationJob concurrency", func() {
	// Regression test for #1428: migrateContainerIfNeeded used an unlocked
	// ReadFrontMatter → mutate in memory → WriteFrontMatter sequence.
	// Any concurrent writer (checklist AddItem, mapmutator, etc.) that committed
	// between the read and the write-back would be silently reverted.
	//
	// The fix migrates migrateContainerIfNeeded onto ModifyFrontMatterAndMarkdown,
	// which runs the read, mutation, and write-back under the page-canonical lock.
	const page = wikipage.PageIdentifier("job_race_container")

	var (
		ctx           context.Context
		pausingStore  *pausingJobStore
		checklist     *checklistmutator.Mutator
		human         = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		appendedTexts []string
		executeErr    error
		list          *apiv1.Checklist
		listErr       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		appendedTexts = nil

		pausingStore = &pausingJobStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: page,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		checklist = checklistmutator.New(pausingStore, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())

		// Seed: container page without is_container (so the migrate step fires)
		// and a checklist seed item so we can detect FM write-back overwrites.
		Expect(pausingStore.Store.WriteFrontMatter(page, wikipage.FrontMatter{
			"inventory": map[string]any{
				"items": []any{"widget"},
			},
		}, wikipage.AnonymousIdentity)).To(Succeed())
		_, _, seedErr := checklist.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: "seed"}, human)
		Expect(seedErr).NotTo(HaveOccurred())
	})

	Describe("when Execute races a concurrent AddItem", func() {
		BeforeEach(func() {
			logger := lumber.NewConsoleLogger(lumber.WARN)
			fmIndex := &jobConcurrencyFmIndex{containerPage: page}
			job, newErr := server.NewInventoryNormalizationJob(pausingStore, fmIndex, logger)
			Expect(newErr).NotTo(HaveOccurred())

			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				executeErr = job.Execute()
			}()

			addOne := func() {
				pausingStore.armed.Store(false)
				text := fmt.Sprintf("concurrent-%d", len(appendedTexts))
				_, _, addErr := checklist.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: text}, human)
				Expect(addErr).NotTo(HaveOccurred())
				appendedTexts = append(appendedTexts, text)
				pausingStore.armed.Store(true)
			}

			// If a pause fires, the broken code is in its read→write window for
			// migrateContainerIfNeeded. Land an AddItem inside that window, then release.
			// If no pause fires in the migrate step, the fixed code uses
			// ModifyFrontMatterAndMarkdown and closes the window; any subsequent pause
			// (from GetContainerItems) is harmless because the FM write has already committed.
			select {
			case <-pausingStore.paused:
				addOne()
				pausingStore.release <- struct{}{}
				<-done
			case <-done:
				// Fixed code path: no window in migration, goroutine completed directly.
			}
			pausingStore.armed.Store(false)

			// After the fix the first pause fires in GetContainerItems (not migration),
			// so appendedTexts may still be empty here. Add unconditionally so the
			// assertion is non-trivial.
			if len(appendedTexts) == 0 {
				addOne()
			}
			pausingStore.armed.Store(false)

			list, listErr = checklist.ListItems(ctx, string(page), "todos")
		})

		It("should complete Execute without error", func() {
			Expect(executeErr).NotTo(HaveOccurred())
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
