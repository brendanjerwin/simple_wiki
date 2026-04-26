//revive:disable:dot-imports
package checklistmutator_test

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestMutator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "checklistmutator")
}

// fakeClock returns a fixed time on every Now() call. Tests advance the
// clock between mutations to assert per-step timestamps.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeStore implements wikipage.PageReaderMutator backed by an in-memory
// map. It records the count of read/write calls so tests can assert
// "single round-trip per mutation."
type fakeStore struct {
	mu          sync.Mutex
	pages       map[string]wikipage.FrontMatter
	readCalls   int
	writeCalls  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{pages: make(map[string]wikipage.FrontMatter)}
}

func (s *fakeStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readCalls++
	fm, ok := s.pages[string(id)]
	if !ok {
		fm = wikipage.FrontMatter{}
	}
	return id, deepCopyFM(fm), nil
}

func (s *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeCalls++
	s.pages[string(id)] = deepCopyFM(fm)
	return nil
}

func (*fakeStore) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}

func (*fakeStore) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error { return nil }
func (*fakeStore) DeletePage(_ wikipage.PageIdentifier) error                          { return nil }
func (*fakeStore) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// deepCopyFM is enough for our tests — TOML-shaped maps with strings,
// numbers, booleans, and nested maps/slices.
func deepCopyFM(in wikipage.FrontMatter) wikipage.FrontMatter {
	if in == nil {
		return nil
	}
	out := make(wikipage.FrontMatter, len(in))
	for k, v := range in {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v2 := range x {
			out[k] = deepCopyValue(v2)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v2 := range x {
			out[i] = deepCopyValue(v2)
		}
		return out
	default:
		return v
	}
}

var _ = Describe("Mutator", func() {
	var (
		store    *fakeStore
		clock    *fakeClock
		ulids    *ulid.SequenceGenerator
		mutator  *checklistmutator.Mutator
		ctx      context.Context
		human    tailscale.IdentityValue
		agent    tailscale.IdentityValue
	)

	BeforeEach(func() {
		store = newFakeStore()
		clock = newFakeClock(time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC))
		ulids = ulid.NewSequenceGenerator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
			"01HXCCCCCCCCCCCCCCCCCCCCCC",
			"01HXDDDDDDDDDDDDDDDDDDDDDD",
		)
		mutator = checklistmutator.New(store, clock, ulids)
		ctx = context.Background()
		human = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		agent = tailscale.NewAgentIdentity("scheduler@example.com", "Scheduler", "scheduler-bot")
	})

	Describe("AddItem", func() {
		When("adding to an empty page (no existing checklist)", func() {
			var (
				item        *apiv1.ChecklistItem
				err         error
				readsBefore int
				writesBefore int
			)

			BeforeEach(func() {
				readsBefore = store.readCalls
				writesBefore = store.writeCalls
				item, _, err = mutator.AddItem(ctx, "shopping", "groceries", checklistmutator.AddItemArgs{Text: "Buy milk"}, human)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should assign the next ULID from the sequence generator", func() {
				Expect(item.Uid).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
			})

			It("should perform exactly one ReadFrontMatter call", func() {
				Expect(store.readCalls - readsBefore).To(Equal(1))
			})

			It("should perform exactly one WriteFrontMatter call", func() {
				Expect(store.writeCalls - writesBefore).To(Equal(1))
			})
		})

		When("the caller is a human", func() {
			var added *apiv1.ChecklistItem

			BeforeEach(func() {
				added, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "T"}, human)
			})

			It("should record automated=false", func() {
				Expect(added.Automated).To(BeFalse())
			})
		})

		When("the caller is an agent", func() {
			var added *apiv1.ChecklistItem

			BeforeEach(func() {
				added, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "T"}, agent)
			})

			It("should record automated=true", func() {
				Expect(added.Automated).To(BeTrue())
			})
		})

		When("an item is added with default sort order", func() {
			It("should append at the next 1000-spaced slot", func() {
				_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "first"}, human)
				added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "second"}, human)
				Expect(added.SortOrder).To(Equal(int64(2000)))
			})
		})
	})

	Describe("ToggleItem", func() {
		var initialUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "T"}, human)
			initialUID = added.Uid
		})

		When("checked transitions false to true", func() {
			var item *apiv1.ChecklistItem

			BeforeEach(func() {
				clock.advance(time.Minute)
				item, _, _ = mutator.ToggleItem(ctx, "p", "list", initialUID, nil, human)
			})

			It("should populate completed_at", func() {
				Expect(item.CompletedAt).NotTo(BeNil())
			})

			It("should populate completed_by from identity.Name()", func() {
				Expect(item.CompletedBy).NotTo(BeNil())
				Expect(*item.CompletedBy).To(Equal("alice@example.com"))
			})
		})

		When("checked transitions true to false", func() {
			var item *apiv1.ChecklistItem

			BeforeEach(func() {
				_, _, _ = mutator.ToggleItem(ctx, "p", "list", initialUID, nil, human)
				clock.advance(time.Minute)
				item, _, _ = mutator.ToggleItem(ctx, "p", "list", initialUID, nil, human)
			})

			It("should clear completed_at", func() {
				Expect(item.CompletedAt).To(BeNil())
			})

			It("should clear completed_by", func() {
				Expect(item.CompletedBy).To(BeNil())
			})
		})
	})

	Describe("UpdateItem", func() {
		var initialUID string
		var initialUpdatedAt time.Time

		BeforeEach(func() {
			added, list, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "Original"}, human)
			initialUID = added.Uid
			initialUpdatedAt = list.UpdatedAt.AsTime()
		})

		When("the request changes the text", func() {
			var (
				updatedAtAfter time.Time
				err            error
			)

			BeforeEach(func() {
				clock.advance(time.Minute)
				newText := "Updated"
				updated, _, updateErr := mutator.UpdateItem(ctx, "p", "list", initialUID, checklistmutator.UpdateItemArgs{Text: &newText}, nil, human)
				err = updateErr
				if updated != nil {
					updatedAtAfter = updated.UpdatedAt.AsTime()
				}
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should advance updated_at past the initial value", func() {
				Expect(updatedAtAfter).To(BeTemporally(">", initialUpdatedAt))
			})
		})

		When("expected_updated_at is stale", func() {
			var err error

			BeforeEach(func() {
				clock.advance(time.Minute)
				_, _, _ = mutator.ToggleItem(ctx, "p", "list", initialUID, nil, human)
				clock.advance(time.Minute)
				stale := initialUpdatedAt
				newText := "Updated"
				_, _, err = mutator.UpdateItem(ctx, "p", "list", initialUID, checklistmutator.UpdateItemArgs{Text: &newText}, &stale, human)
			})

			It("should return FailedPrecondition", func() {
				Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
			})
		})

		When("the uid does not exist", func() {
			var err error

			BeforeEach(func() {
				newText := "X"
				_, _, err = mutator.UpdateItem(ctx, "p", "list", "does-not-exist", checklistmutator.UpdateItemArgs{Text: &newText}, nil, human)
			})

			It("should return ErrItemNotFound", func() {
				Expect(err).To(MatchError(checklistmutator.ErrItemNotFound))
			})
		})
	})

	Describe("DeleteItem", func() {
		var initialUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "Doomed"}, human)
			initialUID = added.Uid
			clock.advance(time.Minute)
		})

		When("the item is deleted", func() {
			var (
				list      *apiv1.Checklist
				deleteErr error
			)

			BeforeEach(func() {
				list, deleteErr = mutator.DeleteItem(ctx, "p", "list", initialUID, nil, human)
			})

			It("should not error", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})

			It("should produce one tombstone for the deleted uid", func() {
				Expect(list.Tombstones).To(HaveLen(1))
			})

			It("should stamp the post-bump sync_token on the tombstone", func() {
				Expect(list.Tombstones).To(HaveLen(1))
				Expect(list.Tombstones[0].SyncToken).To(Equal(list.SyncToken))
			})
		})
	})

	Describe("sync_token", func() {
		var initialUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "T"}, human)
			initialUID = added.Uid
		})

		When("a single mutation occurs", func() {
			It("should advance sync_token by exactly 1", func() {
				_, listBefore, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "intermediate"}, human)
				before := listBefore.SyncToken
				_, _, _ = mutator.ToggleItem(ctx, "p", "list", initialUID, nil, human)
				_, listAfter, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "after"}, human)
				// before → +1 (Toggle) → +1 (AddItem) = before + 2
				Expect(listAfter.SyncToken).To(Equal(before + 2))
			})
		})
	})

	Describe("tombstone GC", func() {
		var deletedUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "X"}, human)
			deletedUID = added.Uid
			_, _ = mutator.DeleteItem(ctx, "p", "list", deletedUID, nil, human)
		})

		When("the next mutation runs after the tombstone TTL", func() {
			It("should drop the expired tombstone", func() {
				clock.advance(checklistmutator.TombstoneTTL + time.Hour)
				_, list, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "fresh"}, human)
				Expect(list.Tombstones).To(BeEmpty())
			})
		})

		When("the next mutation runs before the tombstone TTL", func() {
			It("should keep the tombstone", func() {
				clock.advance(time.Hour)
				_, list, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "fresh"}, human)
				Expect(list.Tombstones).To(HaveLen(1))
			})
		})
	})

	Describe("legacy item promotion on first mutation", func() {
		When("an item without a uid was injected via raw MergeFrontmatter", func() {
			var allItems []*apiv1.ChecklistItem

			BeforeEach(func() {
				// Simulate a raw frontmatter write: an item with no uid.
				store.pages["legacy_page"] = wikipage.FrontMatter{
					"checklists": map[string]any{
						"shopping": map[string]any{
							"items": []any{
								map[string]any{"text": "no uid here", "checked": false},
							},
						},
					},
				}
				// Trigger a mutation: AddItem appends, but should also
				// promote the existing item to a real ULID.
				_, list, _ := mutator.AddItem(ctx, "legacy_page", "shopping",
					checklistmutator.AddItemArgs{Text: "added via service"}, human)
				allItems = list.Items
			})

			It("should persist a real ULID for the legacy item, not the empty string", func() {
				Expect(allItems).To(HaveLen(2))
				for _, it := range allItems {
					Expect(it.Uid).NotTo(BeEmpty(), "every item must have a uid after a mutation")
				}
			})

			It("should keep the original text", func() {
				texts := []string{allItems[0].Text, allItems[1].Text}
				Expect(texts).To(ContainElement("no uid here"))
			})
		})
	})

	Describe("concurrent mutations on the same page", func() {
		It("should serialize without losing updates", func() {
			_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "seed"}, human)

			const concurrency = 20
			var wg sync.WaitGroup
			wg.Add(concurrency)
			for i := 0; i < concurrency; i++ {
				go func() {
					defer wg.Done()
					_, _, _ = mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "concurrent"}, human)
				}()
			}
			wg.Wait()

			list, err := mutator.ListItems(ctx, "p", "list")
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(HaveLen(concurrency + 1))
		})
	})
})
