//revive:disable:dot-imports
package connectors_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

var _ = Describe("LeaseTable", func() {
	var lt *connectors.LeaseTable

	BeforeEach(func() {
		lt = connectors.NewLeaseTable()
	})

	When("Take is called on an unowned checklist", func() {
		var err error

		BeforeEach(func() {
			err = lt.Take(
				connectors.ChecklistKey{Page: "groceries", ListName: "this_week"},
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_alice"},
			)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record the owner", func() {
			owner, ok := lt.LookupOwner(connectors.ChecklistKey{Page: "groceries", ListName: "this_week"})
			Expect(ok).To(BeTrue())
			Expect(owner).To(Equal(connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_alice"}))
		})
	})

	When("Take is called twice with the same owner", func() {
		var err error

		BeforeEach(func() {
			key := connectors.ChecklistKey{Page: "groceries", ListName: "this_week"}
			owner := connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_alice"}
			Expect(lt.Take(key, owner)).To(Succeed())
			err = lt.Take(key, owner)
		})

		It("should succeed (idempotent)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("Take is called with a different owner on a leased checklist", func() {
		var err error

		BeforeEach(func() {
			key := connectors.ChecklistKey{Page: "groceries", ListName: "this_week"}
			Expect(lt.Take(
				key,
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_alice"},
			)).To(Succeed())
			err = lt.Take(
				key,
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleTasks, ProfileID: "profile_bob"},
			)
		})

		It("should return ErrChecklistAlreadyLeased", func() {
			Expect(errors.Is(err, connectors.ErrChecklistAlreadyLeased)).To(BeTrue())
		})

		It("should preserve the original owner", func() {
			owner, ok := lt.LookupOwner(connectors.ChecklistKey{Page: "groceries", ListName: "this_week"})
			Expect(ok).To(BeTrue())
			Expect(owner.ProfileID).To(Equal("profile_alice"))
		})
	})

	When("Release is called on a leased checklist", func() {
		BeforeEach(func() {
			key := connectors.ChecklistKey{Page: "groceries", ListName: "this_week"}
			Expect(lt.Take(
				key,
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_alice"},
			)).To(Succeed())
			lt.Release(key)
		})

		It("should remove the owner record", func() {
			_, ok := lt.LookupOwner(connectors.ChecklistKey{Page: "groceries", ListName: "this_week"})
			Expect(ok).To(BeFalse())
		})

		It("should allow a new owner to take the lease", func() {
			err := lt.Take(
				connectors.ChecklistKey{Page: "groceries", ListName: "this_week"},
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleTasks, ProfileID: "profile_bob"},
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("LookupOwner is called on an unowned checklist", func() {
		It("should report not present", func() {
			_, ok := lt.LookupOwner(connectors.ChecklistKey{Page: "anything", ListName: "anything"})
			Expect(ok).To(BeFalse())
		})
	})

	When("WithChecklistLock is used by concurrent goroutines", func() {
		It("should serialize their critical sections", func() {
			key := connectors.ChecklistKey{Page: "p", ListName: "l"}
			var counter int
			var observed []int
			var mu sync.Mutex
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = lt.WithChecklistLock(key, func() error {
						mu.Lock()
						counter++
						observed = append(observed, counter)
						mu.Unlock()
						// brief work window so any races would surface
						time.Sleep(time.Millisecond)
						mu.Lock()
						counter--
						mu.Unlock()
						return nil
					})
				}()
			}
			wg.Wait()
			// Every observed counter value should be exactly 1 — no
			// concurrent critical section ever saw a second goroutine
			// inside.
			for _, v := range observed {
				Expect(v).To(Equal(1))
			}
		})
	})

	When("WithChecklistLock fn returns an error", func() {
		var err error

		BeforeEach(func() {
			err = lt.WithChecklistLock(
				connectors.ChecklistKey{Page: "p", ListName: "l"},
				func() error { return errors.New("boom") },
			)
		})

		It("should propagate the error", func() {
			Expect(err).To(MatchError("boom"))
		})
	})

	When("WaitReady is called before SignalReady", func() {
		It("should respect ctx cancellation", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			err := lt.WaitReady(ctx)
			Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
		})
	})

	When("WaitReady is called after SignalReady", func() {
		var err error

		BeforeEach(func() {
			lt.SignalReady()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err = lt.WaitReady(ctx)
		})

		It("should return immediately without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("SignalReady is called multiple times", func() {
		It("should not panic on repeated calls", func() {
			Expect(func() {
				lt.SignalReady()
				lt.SignalReady()
				lt.SignalReady()
			}).NotTo(Panic())
		})
	})
})
