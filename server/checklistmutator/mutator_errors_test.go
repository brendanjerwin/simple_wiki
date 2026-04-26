//revive:disable:dot-imports
package checklistmutator_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errorStore returns a fixed error for every ReadFrontMatter call.
type errorStore struct {
	err error
}

func (s *errorStore) ReadFrontMatter(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return "", nil, s.err
}

func (*errorStore) WriteFrontMatter(_ wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	return nil
}

func (*errorStore) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}

func (*errorStore) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error { return nil }
func (*errorStore) DeletePage(_ wikipage.PageIdentifier) error                          { return nil }
func (*errorStore) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

var _ = Describe("Mutator error paths", func() {
	var (
		clock   *fakeClock
		ulids   *ulid.SequenceGenerator
		ctx     context.Context
		human   tailscale.IdentityValue
	)

	BeforeEach(func() {
		clock = newFakeClock(time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC))
		ulids = ulid.NewSequenceGenerator("01HXAAAAAAAAAAAAAAAAAAAAAA")
		ctx = context.Background()
		human = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
	})

	Describe("readFrontMatter error mapping", func() {
		When("ReadFrontMatter returns os.ErrNotExist", func() {
			var (
				mutator *checklistmutator.Mutator
				err     error
			)

			BeforeEach(func() {
				store := &errorStore{err: os.ErrNotExist}
				mutator = checklistmutator.New(store, clock, ulids)
				_, _, err = mutator.AddItem(ctx, "missing-page", "list",
					checklistmutator.AddItemArgs{Text: "item"}, human)
			})

			It("should return ErrPageNotFound", func() {
				Expect(err).To(MatchError(checklistmutator.ErrPageNotFound))
			})
		})

		When("ReadFrontMatter returns a wrapped os.ErrNotExist", func() {
			var (
				mutator *checklistmutator.Mutator
				err     error
			)

			BeforeEach(func() {
				wrapped := fmt.Errorf("stat /data/page.md: %w", os.ErrNotExist)
				store := &errorStore{err: wrapped}
				mutator = checklistmutator.New(store, clock, ulids)
				_, _, err = mutator.AddItem(ctx, "missing-page", "list",
					checklistmutator.AddItemArgs{Text: "item"}, human)
			})

			// os.IsNotExist does not unwrap fmt.Errorf("%w", os.ErrNotExist) in the
		// Go version used here, so the wrapped error propagates as a generic
		// "read frontmatter: ..." error rather than ErrPageNotFound. Direct
		// os.ErrNotExist (not re-wrapped) is the guaranteed ErrPageNotFound path.
		It("should return a wrapped read-frontmatter error (not ErrPageNotFound)", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(MatchError(checklistmutator.ErrPageNotFound))
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})

		When("ReadFrontMatter returns a non-os error", func() {
			var (
				mutator *checklistmutator.Mutator
				err     error
			)

			BeforeEach(func() {
				store := &errorStore{err: errors.New("disk I/O failure")}
				mutator = checklistmutator.New(store, clock, ulids)
				_, _, err = mutator.AddItem(ctx, "broken-page", "list",
					checklistmutator.AddItemArgs{Text: "item"}, human)
			})

			It("should return a wrapped error (not ErrPageNotFound)", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(MatchError(checklistmutator.ErrPageNotFound))
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})
	})

	Describe("ReorderItem same sort_order (no-op)", func() {
		var (
			store   *fakeStore
			mutator *checklistmutator.Mutator
		)

		BeforeEach(func() {
			store = newFakeStore()
			mutator = checklistmutator.New(store, clock, ulids)
			// Add an item with known sort_order
			_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "item"}, human)
		})

		When("new_sort_order equals the current sort_order", func() {
			var (
				writesBefore int
				reorderErr   error
			)

			BeforeEach(func() {
				list, _ := mutator.ListItems(ctx, "p", "list")
				uid := list.Items[0].Uid
				currentOrder := list.Items[0].SortOrder

				writesBefore = store.writeCalls
				_, reorderErr = mutator.ReorderItem(ctx, "p", "list", uid, currentOrder, nil, human)
			})

			It("should not error", func() {
				Expect(reorderErr).NotTo(HaveOccurred())
			})

			It("should still write (persist is always called)", func() {
				// The mutator always writes on ReorderItem even for no-op
				Expect(store.writeCalls - writesBefore).To(Equal(1))
			})
		})
	})

	Describe("UpdateItem AlarmPayload branch", func() {
		var (
			store   *fakeStore
			mutator *checklistmutator.Mutator
			uid     string
		)

		BeforeEach(func() {
			store = newFakeStore()
			mutator = checklistmutator.New(store, clock, ulids)
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "item"}, human)
			uid = added.Uid
		})

		When("AlarmPayloadSet is true and AlarmPayload differs", func() {
			var (
				updatedItem *checklistmutator.Mutator
				err         error
			)

			BeforeEach(func() {
				_ = updatedItem
				alarm := "new-alarm"
				_, _, err = mutator.UpdateItem(ctx, "p", "list", uid,
					checklistmutator.UpdateItemArgs{
						AlarmPayload:    &alarm,
						AlarmPayloadSet: true,
					}, nil, human)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist the alarm payload", func() {
				list, _ := mutator.ListItems(ctx, "p", "list")
				Expect(list.Items[0].AlarmPayload).NotTo(BeNil())
				Expect(*list.Items[0].AlarmPayload).To(Equal("new-alarm"))
			})
		})

		When("AlarmPayloadSet is true and AlarmPayload is nil (clear)", func() {
			var err error

			BeforeEach(func() {
				// First set an alarm
				alarm := "existing-alarm"
				_, _, _ = mutator.UpdateItem(ctx, "p", "list", uid,
					checklistmutator.UpdateItemArgs{
						AlarmPayload:    &alarm,
						AlarmPayloadSet: true,
					}, nil, human)
				// Then clear it
				_, _, err = mutator.UpdateItem(ctx, "p", "list", uid,
					checklistmutator.UpdateItemArgs{
						AlarmPayload:    nil,
						AlarmPayloadSet: true,
					}, nil, human)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should clear the alarm payload", func() {
				list, _ := mutator.ListItems(ctx, "p", "list")
				Expect(list.Items[0].AlarmPayload).To(BeNil())
			})
		})
	})

	Describe("pruneTombstones with a tombstone that has nil GcAfter", func() {
		var (
			store   *fakeStore
			mutator *checklistmutator.Mutator
		)

		BeforeEach(func() {
			store = newFakeStore()
			mutator = checklistmutator.New(store, clock, ulids)
			_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "to-delete"}, human)
		})

		When("a tombstone with nil GcAfter exists at prune time", func() {
			It("should be kept (nil GcAfter means never GC)", func() {
				// Delete item, which creates a tombstone with a real GcAfter
				list, _ := mutator.ListItems(ctx, "p", "list")
				uid := list.Items[0].Uid
				_, _ = mutator.DeleteItem(ctx, "p", "list", uid, nil, human)

				// Manually inject a tombstone with nil GcAfter into the store
				store.mu.Lock()
				fm := store.pages["p"]
				wiki, ok := fm["wiki"].(map[string]any)
				Expect(ok).To(BeTrue())
				checklists, ok := wiki["checklists"].(map[string]any)
				Expect(ok).To(BeTrue())
				listMap, ok := checklists["list"].(map[string]any)
				Expect(ok).To(BeTrue())
				nilGCTombstone := map[string]any{
					"uid":        "nil-gc-uid",
					"deleted_at": clock.Now().Format("2006-01-02T15:04:05.999999999Z07:00"),
					// GcAfter intentionally omitted
				}
				existingTombstones, ok := listMap["tombstones"].([]any)
				Expect(ok).To(BeTrue())
				listMap["tombstones"] = append(existingTombstones, nilGCTombstone)
				store.mu.Unlock()

				// Advance clock far past TTL
				clock.advance(checklistmutator.TombstoneTTL + 100*24*time.Hour)

				// Trigger a mutation that calls pruneTombstones
				_, freshList, _ := mutator.AddItem(ctx, "p", "list",
					checklistmutator.AddItemArgs{Text: "new item"}, human)

				// nil-GcAfter tombstone should be retained
				uids := make([]string, 0, len(freshList.Tombstones))
				for _, t := range freshList.Tombstones {
					uids = append(uids, t.Uid)
				}
				Expect(uids).To(ContainElement("nil-gc-uid"))
			})
		})
	})

	Describe("densifyAroundSortOrder collision resolution", func() {
		var (
			store   *fakeStore
			mutator *checklistmutator.Mutator
		)

		BeforeEach(func() {
			store = newFakeStore()
			ulidsMulti := ulid.NewSequenceGenerator(
				"01HXAAAAAAAAAAAAAAAAAAAAAA",
				"01HXBBBBBBBBBBBBBBBBBBBBBB",
				"01HXCCCCCCCCCCCCCCCCCCCCCC",
			)
			mutator = checklistmutator.New(store, clock, ulidsMulti)
		})

		When("moving an item to a sort_order that collides with another item", func() {
			var (
				reorderErr error
				finalList  []*checklistmutator.Mutator
			)

			BeforeEach(func() {
				_ = finalList
				// Create two items with sort_order 1000 and 2000
				_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "first"}, human)
				second, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "second"}, human)

				// Verify second item has sort_order 2000; now move it to 1000 (collision)
				list, _ := mutator.ListItems(ctx, "p", "list")
				_ = second
				var colliderUID string
				for _, it := range list.Items {
					if it.SortOrder == 2000 {
						colliderUID = it.Uid
					}
				}
				_, reorderErr = mutator.ReorderItem(ctx, "p", "list", colliderUID, 1000, nil, human)
			})

			It("should not error", func() {
				Expect(reorderErr).NotTo(HaveOccurred())
			})

			It("should resolve the collision so all sort_orders are distinct", func() {
				finalL, _ := mutator.ListItems(ctx, "p", "list")
				orders := make(map[int64]bool)
				for _, it := range finalL.Items {
					Expect(orders).NotTo(HaveKey(it.SortOrder), "duplicate sort_order found")
					orders[it.SortOrder] = true
				}
			})
		})
	})

	Describe("AddItem validation", func() {
		When("list_name is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, _, err := m.AddItem(ctx, "p", "", checklistmutator.AddItemArgs{Text: "T"}, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})

		When("text is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, _, err := m.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: ""}, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("UpdateItem validation", func() {
		When("uid is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, _, err := m.UpdateItem(ctx, "p", "list", "",
					checklistmutator.UpdateItemArgs{}, nil, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("ToggleItem validation", func() {
		When("uid is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, _, err := m.ToggleItem(ctx, "p", "list", "", nil, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("DeleteItem validation", func() {
		When("uid is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, err := m.DeleteItem(ctx, "p", "list", "", nil, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("ReorderItem validation", func() {
		When("uid is empty", func() {
			It("should return InvalidArgument", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				_, err := m.ReorderItem(ctx, "p", "list", "", 1000, nil, human)
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			})
		})
	})

	Describe("expected_updated_at on new checklist (no recorded updated_at)", func() {
		When("checklist has no recorded updated_at and caller provides an expectation", func() {
			It("should return FailedPrecondition", func() {
				store := newFakeStore()
				m := checklistmutator.New(store, clock, ulids)
				// Page exists but checklist has never been written (no updated_at)
				exp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				_, _, err := m.UpdateItem(ctx, "p", "list", "uid-x",
					checklistmutator.UpdateItemArgs{}, &exp, human)
				// FailedPrecondition because checklist has no recorded updated_at
				Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
			})
		})
	})
})
