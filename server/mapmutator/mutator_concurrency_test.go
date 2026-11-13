//revive:disable:dot-imports
package mapmutator

import (
	"context"
	"fmt"
	"sync/atomic"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pausingMapStore wraps a real pagestore.Store and intercepts
// ReadFrontMatter to pause after the read returns. This lets a test land a
// concurrent write inside the read→write window that the broken mutateMap
// left open.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed code path goes directly to the page lock without
// pausing — proving the race window is closed.
type pausingMapStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	paused     chan struct{}
	release    chan struct{}
}

// DeletePage and ReadMarkdown are not used by the stores under test; the stubs
// keep the type satisfying wikipage.PageReaderMutator (required by
// checklistmutator.New).
func (*pausingMapStore) DeletePage(wikipage.PageIdentifier) error { return nil }

func (*pausingMapStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (s *pausingMapStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("Mutator concurrency", func() {
	// Regression test for #1428: mutateMap used a whole-frontmatter
	// read-modify-write without holding the page lock. Any concurrent writer
	// (checklist AddItem, inventory, MergeFrontmatter) that committed between
	// the read and the write-back would be silently reverted.
	//
	// The fix migrates mutateMap onto ModifyFrontMatterAndMarkdown, which runs
	// the read, mutation, and write-back under the page-canonical lock.
	const page = wikipage.PageIdentifier("map_mutator_race_page")

	var (
		ctx           context.Context
		pausingStore  *pausingMapStore
		checklist     *checklistmutator.Mutator
		mapMutator    *Mutator
		human         = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		addMarkerErr  error
		appendedTexts []string
		list          *apiv1.Checklist
		listErr       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		appendedTexts = nil

		pausingStore = &pausingMapStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: page,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		checklist = checklistmutator.New(pausingStore, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())
		mapMutator = New(pausingStore, SystemClock{}, ulid.NewSystemGenerator())

		// Seed the page and an initial checklist item so the page exists.
		Expect(pausingStore.Store.WriteFrontMatter(page, wikipage.FrontMatter{}, wikipage.AnonymousIdentity)).To(Succeed())
		_, _, seedErr := checklist.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: "seed"}, human)
		Expect(seedErr).NotTo(HaveOccurred())
	})

	Describe("when AddMarker races a concurrent AddItem", func() {
		BeforeEach(func() {
			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _, addMarkerErr = mapMutator.AddMarker(ctx, string(page), "mymap", &apiv1.MapMarker{
					Label:    "test-marker",
					Position: &apiv1.GeoPoint{Lat: 40.0, Lon: -74.0},
				}, nil, human)
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

			// After the fix no pause fires, so appendedTexts is empty. Append
			// unconditionally so the assertion doesn't trivially pass on an
			// empty set, and to verify items added after the write also survive.
			if len(appendedTexts) == 0 {
				addOne()
			}
			pausingStore.armed.Store(false) // disarm before read-only operations

			list, listErr = checklist.ListItems(ctx, string(page), "todos")
		})

		It("should complete AddMarker without error", func() {
			Expect(addMarkerErr).NotTo(HaveOccurred())
		})

		It("should keep every concurrently added checklist item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElements(appendedTexts))
		})

		It("should keep the seed item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElement("seed"))
		})

		It("should persist the added marker", func() {
			mapState, getErr := mapMutator.GetMap(ctx, string(page), "mymap")
			Expect(getErr).NotTo(HaveOccurred())
			labels := make([]string, 0, len(mapState.Markers))
			for _, m := range mapState.Markers {
				labels = append(labels, m.GetLabel())
			}
			Expect(labels).To(ContainElement("test-marker"))
		})
	})
})
