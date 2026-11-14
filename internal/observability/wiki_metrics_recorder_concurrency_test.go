//revive:disable:dot-imports
package observability_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jcelliott/lumber"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pausingMetricsStore wraps a real pagestore.Store and intercepts
// ReadFrontMatter to pause after the read returns. This lets a test land a
// concurrent write inside the read→write window that the broken Persist
// (readExistingPage + writeFrontmatter) left open.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed persistAtomic path goes directly to the page lock
// without pausing — proving the race window is closed.
type pausingMetricsStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	paused     chan struct{}
	release    chan struct{}
}

// DeletePage and ReadMarkdown are not used by the stores under test; the stubs
// keep the type satisfying wikipage.PageReaderMutator (required by
// checklistmutator.New).
func (*pausingMetricsStore) DeletePage(wikipage.PageIdentifier) error { return nil }

func (*pausingMetricsStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (s *pausingMetricsStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("WikiMetricsRecorder concurrency", func() {
	// Regression test for #1428: Persist used an unlocked
	// ReadFrontMatter (readExistingPage) → build metrics → WriteFrontMatter
	// sequence. Any concurrent writer that committed between the read and the
	// write-back would be silently reverted.
	//
	// The fix migrates Persist onto ModifyFrontMatterAndMarkdown (persistAtomic),
	// which runs the read, mutation, and write-back under the page-canonical lock.
	const metricsPage = observability.ObservabilityMetricsPage

	var (
		ctx           context.Context
		pausingStore  *pausingMetricsStore
		checklist     *checklistmutator.Mutator
		recorder      *observability.WikiMetricsRecorder
		human         = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		appendedTexts []string
		list          *apiv1.Checklist
		listErr       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		appendedTexts = nil

		pausingStore = &pausingMetricsStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: metricsPage,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		checklist = checklistmutator.New(pausingStore, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())

		coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
		var newErr error
		recorder, newErr = observability.NewWikiMetricsRecorder(pausingStore, pausingStore, coordinator, nil)
		Expect(newErr).NotTo(HaveOccurred())

		// Seed the metrics page with a checklist item so we can detect whether
		// Persist's FM write-back overwrites concurrent checklist mutations.
		Expect(pausingStore.Store.WriteFrontMatter(metricsPage, wikipage.FrontMatter{}, wikipage.AnonymousIdentity)).To(Succeed())
		_, _, seedErr := checklist.AddItem(ctx, string(metricsPage), "todos", checklistmutator.AddItemArgs{Text: "seed"}, human)
		Expect(seedErr).NotTo(HaveOccurred())

		// Mark dirty so Persist actually runs.
		recorder.RecordHTTPRequest()
	})

	Describe("when Persist races a concurrent AddItem", func() {
		BeforeEach(func() {
			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = recorder.Persist()
			}()

			addOne := func() {
				pausingStore.armed.Store(false)
				text := fmt.Sprintf("concurrent-%d", len(appendedTexts))
				_, _, addErr := checklist.AddItem(ctx, string(metricsPage), "todos", checklistmutator.AddItemArgs{Text: text}, human)
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

			// After the fix no pause fires, so appendedTexts is empty. Append
			// unconditionally so the assertion doesn't trivially pass on an
			// empty set, and to verify items added after the write also survive.
			if len(appendedTexts) == 0 {
				addOne()
			}
			pausingStore.armed.Store(false)

			list, listErr = checklist.ListItems(ctx, string(metricsPage), "todos")
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
