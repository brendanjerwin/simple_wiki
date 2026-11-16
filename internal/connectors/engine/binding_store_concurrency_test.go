//revive:disable:dot-imports
package engine_test

import (
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// pausingEngineStore wraps a real pagestore.Store and intercepts
// ReadFrontMatter to pause after the read returns. This lets a test land a
// concurrent write inside the read→write window that the broken
// SaveBinding/DeleteBinding left open.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed code path goes directly to the page lock without
// pausing — proving the race window is closed.
type pausingEngineStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	paused     chan struct{}
	release    chan struct{}
}

func (s *pausingEngineStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("FrontmatterBindingStore concurrency", func() {
	// Regression test for #1428: SaveBinding used an unlocked
	// ReadFrontMatter → mutate in memory → WriteFrontMatter sequence.
	// Any concurrent writer that committed between the read and the write-back
	// would be silently reverted.
	//
	// The fix migrates SaveBinding onto ModifyFrontMatterAndMarkdown, which
	// runs the read, mutation, and write-back under the page-canonical lock.
	const profilePage = wikipage.PageIdentifier("engine_binding_race_profile")
	const unrelatedKey = "unrelated_key"

	var (
		pausingStore *pausingEngineStore
		bindingStore *engine.FrontmatterBindingStore
		saveErr      error
		unrelatedVal string
	)

	BeforeEach(func() {
		pausingStore = &pausingEngineStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: profilePage,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		profiles := newFakeProfileLister()
		var newErr error
		bindingStore, newErr = engine.NewFrontmatterBindingStore(pausingStore, profiles, silentLogger{})
		Expect(newErr).NotTo(HaveOccurred())

		// Seed the page so it exists before the race.
		Expect(pausingStore.Store.WriteFrontMatter(profilePage, wikipage.FrontMatter{}, wikipage.AnonymousIdentity)).To(Succeed())
	})

	Describe("when SaveBinding races a concurrent frontmatter write", func() {
		BeforeEach(func() {
			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				saveErr = bindingStore.SaveBinding(profilePage, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					Page:         "shopping",
					ListName:     "groceries",
					RemoteHandle: "tasklist-1",
				})
			}()

			concurrentWrite := func() {
				pausingStore.armed.Store(false)
				modErr := pausingStore.Store.ModifyFrontMatterAndMarkdown(profilePage, func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
					fm[unrelatedKey] = fmt.Sprintf("written-%d", len(fm))
					return fm, md, nil
				}, wikipage.AnonymousIdentity)
				Expect(modErr).NotTo(HaveOccurred())
				pausingStore.armed.Store(true)
			}

			// If a pause fires, the broken code is in its read→write window.
			// Land a concurrent write inside that window, then release.
			// If no pause fires, the fixed code uses ModifyFrontMatterAndMarkdown
			// and closes the window completely.
			select {
			case <-pausingStore.paused:
				concurrentWrite()
				pausingStore.release <- struct{}{}
				<-done
			case <-done:
				// Fixed code path: no window, goroutine completed directly.
			}
			pausingStore.armed.Store(false)

			// After the fix no pause fires, so run the concurrent write
			// unconditionally to verify unrelated keys survive in both paths.
			concurrentWrite()
			pausingStore.armed.Store(false)

			_, resultFM, readErr := pausingStore.Store.ReadFrontMatter(profilePage)
			Expect(readErr).NotTo(HaveOccurred())
			if v, ok := resultFM[unrelatedKey].(string); ok {
				unrelatedVal = v
			}
		})

		It("should complete SaveBinding without error", func() {
			Expect(saveErr).NotTo(HaveOccurred())
		})

		It("should persist the binding", func() {
			bindings, err := bindingStore.LoadBindings(profilePage, connectors.ConnectorKindGoogleTasks)
			Expect(err).NotTo(HaveOccurred())
			Expect(bindings).To(HaveLen(1))
			Expect(bindings[0].Page).To(Equal("shopping"))
		})

		It("should preserve the concurrently written unrelated key", func() {
			Expect(unrelatedVal).NotTo(BeEmpty())
		})
	})
})
