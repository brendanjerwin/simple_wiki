//revive:disable:dot-imports
package chatbuffer

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Note: The test suite bootstrap is in buffer_test.go (TestChatBuffer).
// All var _ = Describe(...) blocks in this package are registered in the same Ginkgo registry.

var _ = Describe("Manager internal", func() {
	Describe("reclaimOnce", func() {
		var manager *Manager

		BeforeEach(func() {
			manager = NewManager()
			DeferCleanup(manager.Close)
		})

		When("a buffer has been inactive beyond the inactivity timeout", func() {
			var pageStillPresent bool

			BeforeEach(func() {
				// Create a buffer for the page
				manager.getOrCreateBuffer("stale-page")

				// Backdate the lastAccess time so it appears stale
				manager.mu.Lock()
				buf := manager.buffers["stale-page"]
				manager.mu.Unlock()

				buf.mu.Lock()
				buf.lastAccess = time.Now().Add(-(BufferInactivityTimeout + time.Minute))
				buf.mu.Unlock()

				// Trigger reclaim with the current time
				manager.reclaimOnce(time.Now())

				manager.mu.RLock()
				_, pageStillPresent = manager.buffers["stale-page"]
				manager.mu.RUnlock()
			})

			It("should remove the stale buffer", func() {
				Expect(pageStillPresent).To(BeFalse())
			})
		})

		When("a buffer has active event listeners", func() {
			var pageStillPresent bool

			BeforeEach(func() {
				// Subscribe to the page so it has an active listener
				_, unsubscribe := manager.SubscribeToPage("active-page")
				DeferCleanup(unsubscribe)

				// Backdate the lastAccess time to make it appear stale
				manager.mu.Lock()
				buf := manager.buffers["active-page"]
				manager.mu.Unlock()

				buf.mu.Lock()
				buf.lastAccess = time.Now().Add(-(BufferInactivityTimeout + time.Minute))
				buf.mu.Unlock()

				// Trigger reclaim
				manager.reclaimOnce(time.Now())

				manager.mu.RLock()
				_, pageStillPresent = manager.buffers["active-page"]
				manager.mu.RUnlock()
			})

			It("should not remove the buffer because it has active listeners", func() {
				Expect(pageStillPresent).To(BeTrue())
			})
		})

		When("a buffer has been accessed recently", func() {
			var pageStillPresent bool

			BeforeEach(func() {
				manager.getOrCreateBuffer("fresh-page")

				// Trigger reclaim with the current time (buffer was just created)
				manager.reclaimOnce(time.Now())

				manager.mu.RLock()
				_, pageStillPresent = manager.buffers["fresh-page"]
				manager.mu.RUnlock()
			})

			It("should not remove the recently-accessed buffer", func() {
				Expect(pageStillPresent).To(BeTrue())
			})
		})

		When("an instance request is older than 2 minutes", func() {
			var requestStillPresent bool

			BeforeEach(func() {
				// Record a stale instance request directly
				manager.instanceMu.Lock()
				manager.instanceRequests["stale-request-page"] = time.Now().Add(-3 * time.Minute)
				manager.instanceMu.Unlock()

				// Trigger reclaim
				manager.reclaimOnce(time.Now())

				manager.instanceMu.RLock()
				_, requestStillPresent = manager.instanceRequests["stale-request-page"]
				manager.instanceMu.RUnlock()
			})

			It("should remove the stale instance request", func() {
				Expect(requestStillPresent).To(BeFalse())
			})
		})

		When("an instance request is within 2 minutes", func() {
			var requestStillPresent bool

			BeforeEach(func() {
				// Record a recent instance request
				manager.instanceMu.Lock()
				manager.instanceRequests["fresh-request-page"] = time.Now().Add(-30 * time.Second)
				manager.instanceMu.Unlock()

				// Trigger reclaim
				manager.reclaimOnce(time.Now())

				manager.instanceMu.RLock()
				_, requestStillPresent = manager.instanceRequests["fresh-request-page"]
				manager.instanceMu.RUnlock()
			})

			It("should keep the recent instance request", func() {
				Expect(requestStillPresent).To(BeTrue())
			})
		})
	})
})
