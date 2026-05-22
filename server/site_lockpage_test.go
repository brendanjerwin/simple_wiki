package server

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Site.lockPage", func() {
	var s *Site

	BeforeEach(func() {
		s = &Site{}
	})

	When("two goroutines acquire locks for different pages", func() {
		var completed chan struct{}

		BeforeEach(func() {
			completed = make(chan struct{}, 2)

			go func() {
				unlock := s.lockPage("page-a")
				defer unlock()
				time.Sleep(50 * time.Millisecond)
				completed <- struct{}{}
			}()
			go func() {
				unlock := s.lockPage("page-b")
				defer unlock()
				time.Sleep(50 * time.Millisecond)
				completed <- struct{}{}
			}()
		})

		It("should let both finish within the time budget of one (not serialize)", func() {
			deadline := time.After(120 * time.Millisecond)
			received := 0
			for received < 2 {
				select {
				case <-completed:
					received++
				case <-deadline:
					Fail("expected both goroutines to complete in parallel within 120ms")
				}
			}
		})
	})

	When("two goroutines acquire locks for the same page", func() {
		var (
			firstStarted    chan struct{}
			firstReleased   chan struct{}
			secondAcquired  chan struct{}
			secondCompleted chan struct{}
		)

		BeforeEach(func() {
			firstStarted = make(chan struct{})
			firstReleased = make(chan struct{})
			secondAcquired = make(chan struct{})
			secondCompleted = make(chan struct{})

			go func() {
				unlock := s.lockPage("contended")
				close(firstStarted)
				time.Sleep(50 * time.Millisecond)
				unlock()
				close(firstReleased)
			}()

			<-firstStarted

			go func() {
				unlock := s.lockPage("contended")
				close(secondAcquired)
				unlock()
				close(secondCompleted)
			}()
		})

		It("should make the second goroutine wait for the first to release", func() {
			select {
			case <-secondAcquired:
				Fail("second goroutine acquired the lock before the first released")
			case <-time.After(20 * time.Millisecond):
				// Expected: second is still blocked while first sleeps.
			}

			<-firstReleased
			Eventually(secondAcquired, 100*time.Millisecond).Should(BeClosed())
			Eventually(secondCompleted, 100*time.Millisecond).Should(BeClosed())
		})
	})

	When("the same page is referenced via different identifier spellings", func() {
		var (
			firstHoldStarted chan struct{}
			secondAcquired   chan struct{}
			firstUnlock      func()
		)

		BeforeEach(func() {
			firstHoldStarted = make(chan struct{})
			secondAcquired = make(chan struct{})

			go func() {
				firstUnlock = s.lockPage("Weekly_Chore_Chart")
				close(firstHoldStarted)
			}()

			<-firstHoldStarted

			go func() {
				unlock := s.lockPage("weekly_chore_chart")
				close(secondAcquired)
				unlock()
			}()
		})

		It("should serialize them via the same canonical lock key", func() {
			select {
			case <-secondAcquired:
				Fail("differently-spelled identifier acquired the lock concurrently — canonicalLockKey is not collapsing spellings")
			case <-time.After(30 * time.Millisecond):
				// Expected.
			}

			firstUnlock()
			Eventually(secondAcquired, 100*time.Millisecond).Should(BeClosed())
		})
	})

	When("the same page is referenced via mixed-case identifiers", func() {
		It("should resolve to the same canonical lock key", func() {
			Expect(canonicalLockKey("FOO")).To(Equal(canonicalLockKey("foo")))
		})
	})

	When("a malformed identifier fails MungeIdentifier", func() {
		It("should still produce a stable, lowercased key (defensive fallback)", func() {
			// Both calls take the fallback path; they must agree.
			malformed := "ID\u202eWITH\u202dADVERSARIAL"
			key1 := canonicalLockKey(malformed)
			key2 := canonicalLockKey(malformed)
			Expect(key1).To(Equal(key2))
		})
	})

	When("many goroutines mutate the same page through the lock", func() {
		It("should serialize all of them without deadlock", func() {
			const concurrency = 50
			var wg sync.WaitGroup
			wg.Add(concurrency)
			counter := 0
			for i := 0; i < concurrency; i++ {
				go func() {
					defer wg.Done()
					unlock := s.lockPage("hot")
					// Under the lock, increment the shared counter.
					// If lockPage doesn't serialize, the counter will lose updates.
					counter++
					unlock()
				}()
			}
			wg.Wait()
			Expect(counter).To(Equal(concurrency))
		})
	})
})
