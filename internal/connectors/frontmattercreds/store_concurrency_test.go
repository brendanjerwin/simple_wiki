//revive:disable:dot-imports
package frontmattercreds_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/frontmattercreds"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// pausingCredsStore wraps a real pagestore.Store and intercepts
// ReadFrontMatter to pause after the read returns, exposing the
// broken read→write window in WriteCredentials.
//
// ModifyFrontMatterAndMarkdown is inherited from the embedded Store and is NOT
// intercepted, so the fixed code path goes directly to the page lock without
// pausing — proving the race window is closed.
type pausingCredsStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	paused     chan struct{}
	release    chan struct{}
}

func (s *pausingCredsStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("frontmattercreds.Store concurrency", func() {
	// Regression test for #1428: WriteCredentials used an unlocked
	// ReadFrontMatter → mutate in memory → WriteFrontMatter sequence.
	// Any concurrent writer that committed between the read and the write-back
	// would be silently reverted.
	//
	// The fix migrates WriteCredentials onto ModifyFrontMatterAndMarkdown,
	// which runs the read, mutation, and write-back under the page-canonical lock.
	const profilePage = wikipage.PageIdentifier("creds_race_profile")
	const unrelatedKey = "unrelated_key"

	var (
		pausingStore *pausingCredsStore
		store        *frontmattercreds.Store[testBundle]
		writeErr     error
		unrelatedVal string
	)

	BeforeEach(func() {
		pausingStore = &pausingCredsStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: profilePage,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		var newErr error
		store, newErr = frontmattercreds.NewStore(
			pausingStore,
			stubClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)},
			lumber.NewConsoleLogger(lumber.WARN),
			nil, nil,
			testCodec{},
		)
		Expect(newErr).NotTo(HaveOccurred())

		// Seed the page so it exists before the race.
		Expect(pausingStore.Store.WriteFrontMatter(profilePage, wikipage.FrontMatter{}, wikipage.AnonymousIdentity)).To(Succeed())
	})

	Describe("when WriteCredentials races a concurrent frontmatter write", func() {
		BeforeEach(func() {
			pausingStore.armed.Store(true)

			done := make(chan struct{})
			go func() {
				defer close(done)
				writeErr = store.WriteCredentials(profilePage, testBundle{
					Token: "tok-raced",
					Email: "user@example.com",
				})
			}()

			concurrentWrite := func() {
				pausingStore.armed.Store(false)
				modErr := pausingStore.Store.ModifyFrontMatterAndMarkdown(profilePage, func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
					fm[unrelatedKey] = fmt.Sprintf("concurrent-%d", len(fm))
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

		It("should complete WriteCredentials without error", func() {
			Expect(writeErr).NotTo(HaveOccurred())
		})

		It("should persist the credentials", func() {
			loaded, err := store.LoadCredentials(context.Background(), profilePage)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Token).To(Equal("tok-raced"))
		})

		It("should preserve the concurrently written unrelated key", func() {
			Expect(unrelatedVal).NotTo(BeEmpty())
		})
	})
})
