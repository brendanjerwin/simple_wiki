//revive:disable:dot-imports
//revive:disable:add-constant
package checklistmutator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
)

// newSyncMutator is a convenience constructor used in sync_helpers behavior
// tests. It wires up the same fakeStore / fakeClock helpers that the rest
// of the checklistmutator_test package uses, so the tests stay small and
// focus on the behaviour under test rather than boilerplate.
func newSyncMutator(ulidSeeds ...string) (*checklistmutator.Mutator, *fakeStore) {
	store := newFakeStore()
	clock := newFakeClock(time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC))
	generator := ulid.NewSequenceGenerator(ulidSeeds...)
	return checklistmutator.New(store, clock, generator), store
}

var _ = Describe("parseSortHint (covered via AddItemForSync)", func() {
	var (
		mutator *checklistmutator.Mutator
		ctx     context.Context
	)

	BeforeEach(func() {
		mutator, _ = newSyncMutator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
			"01HXCCCCCCCCCCCCCCCCCCCCCC",
		)
		ctx = context.Background()
	})

	When("a valid positive integer sort hint is provided", func() {
		var (
			uid string
			err error
		)

		BeforeEach(func() {
			uid, err = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "3000", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-empty uid", func() {
			Expect(uid).NotTo(BeEmpty())
		})

		It("should apply the sort hint as the item's SortOrder", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetSortOrder()).To(Equal(int64(3000)))
		})
	})

	When("a large integer sort hint is provided", func() {
		var (
			uid string
			err error
		)

		BeforeEach(func() {
			uid, err = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "99000", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-empty uid", func() {
			Expect(uid).NotTo(BeEmpty())
		})

		It("should apply the large sort hint as the item's SortOrder", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetSortOrder()).To(Equal(int64(99000)))
		})
	})

	When("a non-numeric (float-style) sort hint is provided", func() {
		var uid string

		BeforeEach(func() {
			var err error
			uid, err = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "3.14", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append at the default sort order slot (hint treated as no-hint)", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			// Default first-item slot is 1000 (1-indexed * 1000)
			Expect(list.GetItems()[0].GetSortOrder()).To(Equal(int64(1000)))
		})

		It("should return a non-empty uid", func() {
			Expect(uid).NotTo(BeEmpty())
		})
	})

	When("an alphabetic sort hint is provided", func() {
		BeforeEach(func() {
			_, err := mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "abc", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fall back to the default sort order", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetSortOrder()).To(Equal(int64(1000)))
		})
	})

	When("an empty sort hint is provided", func() {
		BeforeEach(func() {
			_, err := mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append at the default sort order slot", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetSortOrder()).To(Equal(int64(1000)))
		})
	})
})

var _ = Describe("AddItemForSync", func() {
	var (
		mutator *checklistmutator.Mutator
		ctx     context.Context
	)

	BeforeEach(func() {
		mutator, _ = newSyncMutator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
			"01HXCCCCCCCCCCCCCCCCCCCCCC",
		)
		ctx = context.Background()
	})

	When("adding an unchecked item with tags and description", func() {
		var (
			uid string
			err error
		)

		BeforeEach(func() {
			uid, err = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "buy milk", false, []string{"#food"}, "a note", "", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the item uid", func() {
			Expect(uid).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should persist the item as unchecked", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetChecked()).To(BeFalse())
		})

		It("should persist the item text", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()[0].GetText()).To(Equal("buy milk"))
		})
	})

	When("adding a pre-checked item", func() {
		var (
			uid string
			err error
		)

		BeforeEach(func() {
			uid, err = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "buy milk", true, nil, "", "", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a uid", func() {
			Expect(uid).NotTo(BeEmpty())
		})

		It("should persist the item as checked", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			Expect(list.GetItems()).To(HaveLen(1))
			Expect(list.GetItems()[0].GetChecked()).To(BeTrue())
		})
	})

	When("the owner email is empty (system fallback)", func() {
		var err error

		BeforeEach(func() {
			_, err = mutator.AddItemForSync(ctx, "p", "list", "", "text", false, nil, "", "", nil)
		})

		It("should not error (system identity is used as fallback)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("UpdateItemForSync", func() {
	var (
		mutator *checklistmutator.Mutator
		ctx     context.Context
		uid     string
	)

	BeforeEach(func() {
		mutator, _ = newSyncMutator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
		)
		ctx = context.Background()
		var addErr error
		uid, addErr = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "original text", false, nil, "", "", nil)
		Expect(addErr).NotTo(HaveOccurred())
	})

	When("updating an item's text and tags", func() {
		var err error

		BeforeEach(func() {
			err = mutator.UpdateItemForSync(ctx, "p", "list", "alice@example.com", uid, "updated text", false, []string{"#newtag"}, "", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should persist the updated text", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			var found bool
			for _, it := range list.GetItems() {
				if it.GetUid() == uid {
					Expect(it.GetText()).To(Equal("updated text"))
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	When("toggling an item from unchecked to checked via UpdateItemForSync", func() {
		var err error

		BeforeEach(func() {
			err = mutator.UpdateItemForSync(ctx, "p", "list", "alice@example.com", uid, "original text", true, nil, "", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should persist checked=true", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			var found bool
			for _, it := range list.GetItems() {
				if it.GetUid() == uid {
					Expect(it.GetChecked()).To(BeTrue())
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	When("updating an item with a description", func() {
		var err error

		BeforeEach(func() {
			err = mutator.UpdateItemForSync(ctx, "p", "list", "alice@example.com", uid, "text", false, nil, "my description", nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("DeleteItemForSync", func() {
	var (
		mutator *checklistmutator.Mutator
		ctx     context.Context
		uid     string
	)

	BeforeEach(func() {
		mutator, _ = newSyncMutator("01HXAAAAAAAAAAAAAAAAAAAAAA")
		ctx = context.Background()
		var addErr error
		uid, addErr = mutator.AddItemForSync(ctx, "p", "list", "alice@example.com", "text", false, nil, "", "", nil)
		Expect(addErr).NotTo(HaveOccurred())
	})

	When("deleting an existing item", func() {
		var err error

		BeforeEach(func() {
			err = mutator.DeleteItemForSync(ctx, "p", "list", "alice@example.com", uid)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the item from the list", func() {
			list, listErr := mutator.ListItems(ctx, "p", "list")
			Expect(listErr).NotTo(HaveOccurred())
			for _, it := range list.GetItems() {
				Expect(it.GetUid()).NotTo(Equal(uid))
			}
		})
	})
})
