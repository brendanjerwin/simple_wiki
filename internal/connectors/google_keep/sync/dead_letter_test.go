//revive:disable:dot-imports
package sync_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// deadLetterThresholdForTests mirrors the production threshold
// (deadLetterThreshold = 10 in connector.go). Defined locally so the
// test reads as data, not magic numbers.
const deadLetterThresholdForTests = 10

var _ = Describe("Connector dead-letter operations", func() {
	const (
		profile  = wikipage.PageIdentifier("profile_alice_example_com_aaaaaaaa")
		page     = "Groceries"
		listName = "weekly"
		listSrv  = "srv-list-1"
	)

	Describe("ListDeadLetters", func() {
		Describe("when the binding has items with mixed PushFailureCount values", func() {
			var (
				entries []keepsync.DeadLetterEntry
				err     error
			)

			BeforeEach(func() {
				c, store, _, _ := freshConnector(profile, page, listName, listSrv, nil)

				// Hand-craft a binding with three items: below, at,
				// and above the dead-letter threshold.
				bs := keepsync.NewSubscriptionStore(store)
				st, loadErr := bs.LoadState(profile)
				Expect(loadErr).ToNot(HaveOccurred())
				idMap := map[string]keepsync.ItemMapping{
					"uid-below": {
						ServerID:             "srv-A",
						LastObservedWikiText: "below threshold",
						PushFailureCount:     deadLetterThresholdForTests - 1,
						LastFailureCode:      "TRANSIENT",
					},
					"uid-at": {
						ServerID:             "srv-B",
						LastObservedWikiText: "at threshold",
						PushFailureCount:     deadLetterThresholdForTests,
						LastFailureCode:      "REJECTED",
						SyncedText:           "at threshold synced",
					},
					"uid-above": {
						ServerID:             "srv-C",
						LastObservedWikiText: "above threshold",
						PushFailureCount:     deadLetterThresholdForTests + 5,
						LastFailureCode:      "FATAL",
					},
				}
				st.Subscriptions[0].ItemIDMap = idMap
				Expect(bs.SaveState(profile, st)).To(Succeed())

				entries, err = c.ListDeadLetters(context.Background(), profile, page, listName)
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return only items at or above the threshold", func() {
				uids := make([]string, 0, len(entries))
				for _, e := range entries {
					uids = append(uids, e.ItemUID)
				}
				Expect(uids).To(ConsistOf("uid-at", "uid-above"))
			})

			It("should populate text from LastObservedWikiText", func() {
				byUID := map[string]keepsync.DeadLetterEntry{}
				for _, e := range entries {
					byUID[e.ItemUID] = e
				}
				Expect(byUID["uid-at"].Text).To(Equal("at threshold"))
				Expect(byUID["uid-above"].Text).To(Equal("above threshold"))
			})

			It("should populate the failure code from LastFailureCode", func() {
				byUID := map[string]keepsync.DeadLetterEntry{}
				for _, e := range entries {
					byUID[e.ItemUID] = e
				}
				Expect(byUID["uid-at"].LastFailureCode).To(Equal("REJECTED"))
				Expect(byUID["uid-above"].LastFailureCode).To(Equal("FATAL"))
			})

			It("should populate the failure count", func() {
				byUID := map[string]keepsync.DeadLetterEntry{}
				for _, e := range entries {
					byUID[e.ItemUID] = e
				}
				Expect(byUID["uid-at"].PushFailureCount).To(Equal(deadLetterThresholdForTests))
				Expect(byUID["uid-above"].PushFailureCount).To(Equal(deadLetterThresholdForTests + 5))
			})
		})

		Describe("when the binding has no dead-lettered items", func() {
			var (
				entries []keepsync.DeadLetterEntry
				err     error
			)

			BeforeEach(func() {
				c, _, _, _ := freshConnector(profile, page, listName, listSrv, map[string]string{
					"uid-A": "srv-A",
				})
				entries, err = c.ListDeadLetters(context.Background(), profile, page, listName)
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty slice", func() {
				Expect(entries).To(BeEmpty())
			})
		})

		Describe("when the binding does not exist", func() {
			var err error

			BeforeEach(func() {
				c, _, _, _ := freshConnector(profile, page, listName, listSrv, nil)
				_, err = c.ListDeadLetters(context.Background(), profile, "nonexistent-page", "nonexistent-list")
			})

			It("should return ErrSubscriptionNotFound", func() {
				Expect(errors.Is(err, keepsync.ErrSubscriptionNotFound)).To(BeTrue(), "expected ErrSubscriptionNotFound, got: %v", err)
			})
		})
	})

	Describe("ClearDeadLetter", func() {
		Describe("when called for a dead-lettered item", func() {
			var (
				err            error
				clearedItem    keepsync.ItemMapping
				preservedItem  keepsync.ItemMapping
				preservedFound bool
			)

			BeforeEach(func() {
				c, store, _, _ := freshConnector(profile, page, listName, listSrv, nil)
				bs := keepsync.NewSubscriptionStore(store)
				st, loadErr := bs.LoadState(profile)
				Expect(loadErr).ToNot(HaveOccurred())
				st.Subscriptions[0].ItemIDMap = map[string]keepsync.ItemMapping{
					"uid-cleared": {
						ServerID:         "srv-cleared",
						SyncedText:       "synced text - preserved",
						SyncedChecked:    true,
						SyncedSortValue:  "100",
						PushFailureCount: deadLetterThresholdForTests + 2,
						LastFailureCode:  "REJECTED",
						NextAttemptAt:    time.Now().Add(time.Hour),
					},
					"uid-other": {
						ServerID:         "srv-other",
						PushFailureCount: deadLetterThresholdForTests + 1,
						LastFailureCode:  "FATAL",
					},
				}
				Expect(bs.SaveState(profile, st)).To(Succeed())

				err = c.ClearDeadLetter(context.Background(), profile, page, listName, "uid-cleared")

				// Re-load to inspect the persisted state.
				postState, postErr := bs.LoadState(profile)
				Expect(postErr).ToNot(HaveOccurred())
				clearedItem = postState.Subscriptions[0].ItemIDMap["uid-cleared"]
				preservedItem, preservedFound = postState.Subscriptions[0].ItemIDMap["uid-other"]
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reset PushFailureCount to zero", func() {
				Expect(clearedItem.PushFailureCount).To(Equal(0))
			})

			It("should clear LastFailureCode", func() {
				Expect(clearedItem.LastFailureCode).To(Equal(""))
			})

			It("should clear NextAttemptAt", func() {
				Expect(clearedItem.NextAttemptAt.IsZero()).To(BeTrue())
			})

			It("should preserve SyncedText", func() {
				Expect(clearedItem.SyncedText).To(Equal("synced text - preserved"))
			})

			It("should preserve SyncedChecked", func() {
				Expect(clearedItem.SyncedChecked).To(BeTrue())
			})

			It("should preserve SyncedSortValue", func() {
				Expect(clearedItem.SyncedSortValue).To(Equal("100"))
			})

			It("should preserve ServerID", func() {
				Expect(clearedItem.ServerID).To(Equal("srv-cleared"))
			})

			It("should not touch other ItemBindings in the same binding", func() {
				Expect(preservedFound).To(BeTrue())
				Expect(preservedItem.PushFailureCount).To(Equal(deadLetterThresholdForTests + 1))
				Expect(preservedItem.LastFailureCode).To(Equal("FATAL"))
			})
		})

		Describe("when called for a non-existent item_uid", func() {
			var err error

			BeforeEach(func() {
				c, _, _, _ := freshConnector(profile, page, listName, listSrv, map[string]string{
					"uid-real": "srv-real",
				})
				err = c.ClearDeadLetter(context.Background(), profile, page, listName, "uid-bogus")
			})

			It("should return ErrDeadLetterItemNotFound", func() {
				Expect(errors.Is(err, keepsync.ErrDeadLetterItemNotFound)).To(BeTrue(), "expected ErrDeadLetterItemNotFound, got: %v", err)
			})
		})

		Describe("when called for a non-existent binding", func() {
			var err error

			BeforeEach(func() {
				c, _, _, _ := freshConnector(profile, page, listName, listSrv, nil)
				err = c.ClearDeadLetter(context.Background(), profile, "missing-page", "missing-list", "uid-x")
			})

			It("should return ErrSubscriptionNotFound", func() {
				Expect(errors.Is(err, keepsync.ErrSubscriptionNotFound)).To(BeTrue(), "expected ErrSubscriptionNotFound, got: %v", err)
			})
		})
	})
})
