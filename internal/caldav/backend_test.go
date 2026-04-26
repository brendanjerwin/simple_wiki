//revive:disable:dot-imports
package caldav_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/internal/caldav/icalcodec"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

func TestBackend(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "caldav backend")
}

// fakeMutator is a hand-rolled MutatorBackend used by the backend tests.
// It stores a fixed map of pages -> checklists and returns clones so
// callers cannot mutate test fixtures by accident. Tests that exercise
// UpsertFromCalDAV / DeleteItem inject upsertFn / deleteFn to capture
// arguments and return canned responses.
type fakeMutator struct {
	pages    map[string][]*apiv1.Checklist
	upsertFn func(ctx context.Context, page, listName, uid string, args checklistmutator.UpsertFromCalDAVArgs, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error)
	deleteFn func(ctx context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, identity tailscale.IdentityValue) (*apiv1.Checklist, error)
}

func (f *fakeMutator) GetChecklists(_ context.Context, page string) ([]*apiv1.Checklist, error) {
	cls, ok := f.pages[page]
	if !ok {
		return nil, nil
	}
	return cls, nil
}

func (f *fakeMutator) ListItems(_ context.Context, page, listName string) (*apiv1.Checklist, error) {
	cls, ok := f.pages[page]
	if !ok {
		// Mirror real Mutator behavior: an unknown page yields an empty
		// checklist with the requested name (no UpdatedAt, no items).
		return &apiv1.Checklist{Name: listName}, nil
	}
	for _, c := range cls {
		if c.Name == listName {
			return c, nil
		}
	}
	return &apiv1.Checklist{Name: listName}, nil
}

func (f *fakeMutator) UpsertFromCalDAV(ctx context.Context, page, listName, uid string, args checklistmutator.UpsertFromCalDAVArgs, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, page, listName, uid, args, ifMatch, ifNoneMatch, identity)
	}
	return nil, nil, errors.New("fakeMutator.UpsertFromCalDAV not used in these tests")
}

func (f *fakeMutator) DeleteItem(ctx context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, identity tailscale.IdentityValue) (*apiv1.Checklist, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, page, listName, uid, expectedUpdatedAt, identity)
	}
	return nil, errors.New("fakeMutator.DeleteItem not used in these tests")
}

// fixedNow returns a deterministic clock for DTSTAMP / nowFn injection.
func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

var _ = Describe("defaultBackend.ListCollections", func() {
	var (
		ctx     context.Context
		now     time.Time
		fake    *fakeMutator
		backend caldav.CalendarBackend
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
	})

	When("the page has no checklists", func() {
		var (
			cols []caldav.CalendarCollection
			err  error
		)

		BeforeEach(func() {
			cols, err = backend.ListCollections(ctx, "no-lists-page")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return no collections", func() {
			Expect(cols).To(BeEmpty())
		})
	})

	When("the page has two checklists", func() {
		var (
			cols       []caldav.CalendarCollection
			err        error
			updated1   time.Time
			updated2   time.Time
			checklist1 *apiv1.Checklist
			checklist2 *apiv1.Checklist
		)

		BeforeEach(func() {
			updated1 = now.Add(-1 * time.Hour)
			updated2 = now.Add(-2 * time.Hour)
			checklist1 = &apiv1.Checklist{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(updated1),
				SyncToken: 7,
			}
			checklist2 = &apiv1.Checklist{
				Name:      "next-week",
				UpdatedAt: timestamppb.New(updated2),
				SyncToken: 3,
			}
			fake.pages["shopping"] = []*apiv1.Checklist{checklist1, checklist2}
			cols, err = backend.ListCollections(ctx, "shopping")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return one collection per checklist", func() {
			Expect(cols).To(HaveLen(2))
		})

		It("should set Page from the input page name", func() {
			Expect(cols[0].Page).To(Equal("shopping"))
			Expect(cols[1].Page).To(Equal("shopping"))
		})

		It("should set ListName from the checklist Name", func() {
			Expect(cols[0].ListName).To(Equal("this-week"))
			Expect(cols[1].ListName).To(Equal("next-week"))
		})

		It("should set DisplayName from the checklist Name", func() {
			Expect(cols[0].DisplayName).To(Equal("this-week"))
			Expect(cols[1].DisplayName).To(Equal("next-week"))
		})

		It("should set UpdatedAt from the checklist UpdatedAt", func() {
			Expect(cols[0].UpdatedAt).To(Equal(updated1))
			Expect(cols[1].UpdatedAt).To(Equal(updated2))
		})

		It("should set SyncToken to the URI form of the sync_token counter", func() {
			Expect(cols[0].SyncToken).To(Equal("http://simple-wiki.local/ns/sync/7"))
			Expect(cols[1].SyncToken).To(Equal("http://simple-wiki.local/ns/sync/3"))
		})

		It("should set CTag to the quoted RFC3339Nano of the checklist UpdatedAt", func() {
			Expect(cols[0].CTag).To(Equal(`"` + updated1.Format(time.RFC3339Nano) + `"`))
			Expect(cols[1].CTag).To(Equal(`"` + updated2.Format(time.RFC3339Nano) + `"`))
		})
	})
})

var _ = Describe("defaultBackend.GetCollection", func() {
	var (
		ctx     context.Context
		now     time.Time
		fake    *fakeMutator
		backend caldav.CalendarBackend
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
	})

	When("the named (page, list) exists", func() {
		var (
			col       caldav.CalendarCollection
			err       error
			updatedAt time.Time
		)

		BeforeEach(func() {
			updatedAt = now.Add(-15 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(updatedAt),
				SyncToken: 11,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(updatedAt),
				}},
			}}
			col, err = backend.GetCollection(ctx, "shopping", "this-week")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set Page from the input page name", func() {
			Expect(col.Page).To(Equal("shopping"))
		})

		It("should set ListName from the checklist name", func() {
			Expect(col.ListName).To(Equal("this-week"))
		})

		It("should set DisplayName from the checklist name", func() {
			Expect(col.DisplayName).To(Equal("this-week"))
		})

		It("should set UpdatedAt from the checklist UpdatedAt", func() {
			Expect(col.UpdatedAt).To(Equal(updatedAt))
		})

		It("should set SyncToken to the URI form of the sync_token counter", func() {
			Expect(col.SyncToken).To(Equal("http://simple-wiki.local/ns/sync/11"))
		})

		It("should set CTag to the quoted RFC3339Nano of the checklist UpdatedAt", func() {
			Expect(col.CTag).To(Equal(`"` + updatedAt.Format(time.RFC3339Nano) + `"`))
		})
	})

	When("the named list does not exist on the page", func() {
		var (
			col caldav.CalendarCollection
			err error
		)

		BeforeEach(func() {
			// Page exists but only has "other-list", not "this-week".
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "other-list",
				UpdatedAt: timestamppb.New(now),
			}}
			col, err = backend.GetCollection(ctx, "shopping", "this-week")
		})

		It("should return ErrCollectionNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrCollectionNotFound))
		})

		It("should return a zero-value CalendarCollection", func() {
			Expect(col).To(Equal(caldav.CalendarCollection{}))
		})
	})

	When("the page itself does not exist", func() {
		var (
			col caldav.CalendarCollection
			err error
		)

		BeforeEach(func() {
			col, err = backend.GetCollection(ctx, "missing-page", "this-week")
		})

		It("should return ErrCollectionNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrCollectionNotFound))
		})

		It("should return a zero-value CalendarCollection", func() {
			Expect(col).To(Equal(caldav.CalendarCollection{}))
		})
	})
})

var _ = Describe("defaultBackend.ListItems", func() {
	var (
		ctx     context.Context
		now     time.Time
		fake    *fakeMutator
		backend caldav.CalendarBackend
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
	})

	When("the collection has two live items and one tombstoned uid", func() {
		var (
			col       caldav.CalendarCollection
			items     []caldav.CalendarItem
			err       error
			updatedAt time.Time
			item1     *apiv1.ChecklistItem
			item2     *apiv1.ChecklistItem
		)

		BeforeEach(func() {
			updatedAt = now.Add(-1 * time.Hour)
			itemUpdated := now.Add(-30 * time.Minute)
			item1 = &apiv1.ChecklistItem{
				Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
				Text:      "Buy milk",
				SortOrder: 1000,
				CreatedAt: timestamppb.New(itemUpdated.Add(-time.Hour)),
				UpdatedAt: timestamppb.New(itemUpdated),
			}
			item2 = &apiv1.ChecklistItem{
				Uid:       "01HXBBBBBBBBBBBBBBBBBBBBBB",
				Text:      "Buy bread",
				SortOrder: 2000,
				CreatedAt: timestamppb.New(itemUpdated.Add(-time.Hour)),
				UpdatedAt: timestamppb.New(itemUpdated),
			}
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(updatedAt),
				SyncToken: 5,
				Items:     []*apiv1.ChecklistItem{item1, item2},
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 3,
				}},
			}}
			col, items, err = backend.ListItems(ctx, "shopping", "this-week")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return one entry per live item", func() {
			Expect(items).To(HaveLen(2))
		})

		It("should not include tombstoned uids", func() {
			for _, it := range items {
				Expect(it.UID).NotTo(Equal("01HXCCCCCCCCCCCCCCCCCCCCCC"))
			}
		})

		It("should set each item's UID from the source item", func() {
			Expect(items[0].UID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
			Expect(items[1].UID).To(Equal("01HXBBBBBBBBBBBBBBBBBBBBBB"))
		})

		It("should set each item's ETag from the per-item ETag derivation", func() {
			Expect(items[0].ETag).To(Equal(`W/"` + item1.UpdatedAt.AsTime().Format(time.RFC3339Nano) + `"`))
			Expect(items[1].ETag).To(Equal(`W/"` + item2.UpdatedAt.AsTime().Format(time.RFC3339Nano) + `"`))
		})

		It("should set each item's UpdatedAt from the source item", func() {
			Expect(items[0].UpdatedAt).To(Equal(item1.UpdatedAt.AsTime()))
			Expect(items[1].UpdatedAt).To(Equal(item2.UpdatedAt.AsTime()))
		})

		It("should set each item's CreatedAt from the source item", func() {
			Expect(items[0].CreatedAt).To(Equal(item1.CreatedAt.AsTime()))
			Expect(items[1].CreatedAt).To(Equal(item2.CreatedAt.AsTime()))
		})

		It("should produce non-empty ICalBytes for each item", func() {
			Expect(items[0].ICalBytes).NotTo(BeEmpty())
			Expect(items[1].ICalBytes).NotTo(BeEmpty())
		})

		It("should produce ICalBytes containing a VTODO component", func() {
			Expect(string(items[0].ICalBytes)).To(ContainSubstring("BEGIN:VTODO"))
			Expect(string(items[1].ICalBytes)).To(ContainSubstring("BEGIN:VTODO"))
		})

		It("should return collection metadata with Page set from input", func() {
			Expect(col.Page).To(Equal("shopping"))
		})

		It("should return collection metadata with ListName set from input", func() {
			Expect(col.ListName).To(Equal("this-week"))
		})

		It("should return collection metadata with UpdatedAt from the checklist", func() {
			Expect(col.UpdatedAt).To(Equal(updatedAt))
		})

		It("should return collection metadata with the URI sync-token", func() {
			Expect(col.SyncToken).To(Equal("http://simple-wiki.local/ns/sync/5"))
		})

		It("should return collection metadata with the quoted-time CTag", func() {
			Expect(col.CTag).To(Equal(`"` + updatedAt.Format(time.RFC3339Nano) + `"`))
		})
	})

	When("the named list does not exist on the page", func() {
		var (
			col   caldav.CalendarCollection
			items []caldav.CalendarItem
			err   error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "other-list",
				UpdatedAt: timestamppb.New(now),
			}}
			col, items, err = backend.ListItems(ctx, "shopping", "this-week")
		})

		It("should return ErrCollectionNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrCollectionNotFound))
		})

		It("should return zero collection metadata", func() {
			Expect(col).To(Equal(caldav.CalendarCollection{}))
		})

		It("should return no items", func() {
			Expect(items).To(BeEmpty())
		})
	})
})

var _ = Describe("defaultBackend.GetItem", func() {
	var (
		ctx     context.Context
		now     time.Time
		fake    *fakeMutator
		backend caldav.CalendarBackend
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
	})

	When("the requested uid names a live item", func() {
		var (
			item   caldav.CalendarItem
			err    error
			source *apiv1.ChecklistItem
		)

		BeforeEach(func() {
			itemUpdated := now.Add(-30 * time.Minute)
			source = &apiv1.ChecklistItem{
				Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
				Text:      "Buy milk",
				SortOrder: 1000,
				CreatedAt: timestamppb.New(itemUpdated.Add(-time.Hour)),
				UpdatedAt: timestamppb.New(itemUpdated),
			}
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items:     []*apiv1.ChecklistItem{source},
			}}
			item, err = backend.GetItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set UID from the source item", func() {
			Expect(item.UID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should derive ETag from the source item's UpdatedAt", func() {
			Expect(item.ETag).To(Equal(`W/"` + source.UpdatedAt.AsTime().Format(time.RFC3339Nano) + `"`))
		})

		It("should set UpdatedAt from the source item", func() {
			Expect(item.UpdatedAt).To(Equal(source.UpdatedAt.AsTime()))
		})

		It("should set CreatedAt from the source item", func() {
			Expect(item.CreatedAt).To(Equal(source.CreatedAt.AsTime()))
		})

		It("should produce non-empty ICalBytes", func() {
			Expect(item.ICalBytes).NotTo(BeEmpty())
		})

		It("should produce ICalBytes containing a VTODO component", func() {
			Expect(string(item.ICalBytes)).To(ContainSubstring("BEGIN:VTODO"))
		})
	})

	When("the requested uid is in the tombstone list", func() {
		var (
			item caldav.CalendarItem
			err  error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 3,
				}},
			}}
			item, err = backend.GetItem(ctx, "shopping", "this-week", "01HXCCCCCCCCCCCCCCCCCCCCCC")
		})

		It("should return ErrItemDeleted", func() {
			Expect(err).To(MatchError(caldav.ErrItemDeleted))
		})

		It("should return a zero-value CalendarItem", func() {
			Expect(item).To(Equal(caldav.CalendarItem{}))
		})
	})

	When("the requested uid is unknown", func() {
		var (
			item caldav.CalendarItem
			err  error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(now),
				}},
			}}
			item, err = backend.GetItem(ctx, "shopping", "this-week", "01HXZZZZZZZZZZZZZZZZZZZZZZ")
		})

		It("should return ErrItemNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrItemNotFound))
		})

		It("should return a zero-value CalendarItem", func() {
			Expect(item).To(Equal(caldav.CalendarItem{}))
		})
	})

	When("the named list does not exist on the page", func() {
		var (
			item caldav.CalendarItem
			err  error
		)

		BeforeEach(func() {
			item, err = backend.GetItem(ctx, "missing", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA")
		})

		It("should return ErrItemNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrItemNotFound))
		})

		It("should return a zero-value CalendarItem", func() {
			Expect(item).To(Equal(caldav.CalendarItem{}))
		})
	})
})

var _ = Describe("defaultBackend.DeleteItem", func() {
	var (
		ctx      context.Context
		now      time.Time
		fake     *fakeMutator
		backend  caldav.CalendarBackend
		identity tailscale.IdentityValue
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
		identity = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
	})

	When("ifMatch is empty and the item exists", func() {
		var (
			err                       error
			capturedExpectedUpdatedAt *time.Time
			capturedPage              string
			capturedListName          string
			capturedUID               string
			capturedIdentity          tailscale.IdentityValue
			itemUpdated               time.Time
		)

		BeforeEach(func() {
			itemUpdated = now.Add(-30 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(itemUpdated),
				}},
			}}
			fake.deleteFn = func(_ context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, id tailscale.IdentityValue) (*apiv1.Checklist, error) {
				capturedPage = page
				capturedListName = listName
				capturedUID = uid
				capturedExpectedUpdatedAt = expectedUpdatedAt
				capturedIdentity = id
				return &apiv1.Checklist{Name: listName}, nil
			}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", "", identity)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call mutator.DeleteItem with nil expectedUpdatedAt", func() {
			Expect(capturedExpectedUpdatedAt).To(BeNil())
		})

		It("should pass the page through to the mutator", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass the list name through to the mutator", func() {
			Expect(capturedListName).To(Equal("this-week"))
		})

		It("should pass the uid through to the mutator", func() {
			Expect(capturedUID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should pass the identity through to the mutator", func() {
			Expect(capturedIdentity).To(Equal(identity))
		})
	})

	When("ifMatch matches the current item ETag", func() {
		var (
			err                       error
			capturedExpectedUpdatedAt *time.Time
			itemUpdated               time.Time
			ifMatch                   string
		)

		BeforeEach(func() {
			itemUpdated = now.Add(-30 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(itemUpdated),
				}},
			}}
			ifMatch = `W/"` + itemUpdated.Format(time.RFC3339Nano) + `"`
			fake.deleteFn = func(_ context.Context, _, _, _ string, expectedUpdatedAt *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
				capturedExpectedUpdatedAt = expectedUpdatedAt
				return &apiv1.Checklist{Name: "this-week"}, nil
			}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", ifMatch, identity)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call mutator.DeleteItem with non-nil expectedUpdatedAt", func() {
			Expect(capturedExpectedUpdatedAt).NotTo(BeNil())
		})

		It("should pass the item's UpdatedAt as expectedUpdatedAt", func() {
			Expect(*capturedExpectedUpdatedAt).To(Equal(itemUpdated))
		})
	})

	When("ifMatch does not match the current item ETag", func() {
		var (
			err              error
			deleteFnCalled   bool
		)

		BeforeEach(func() {
			itemUpdated := now.Add(-30 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(itemUpdated),
				}},
			}}
			fake.deleteFn = func(_ context.Context, _, _, _ string, _ *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
				deleteFnCalled = true
				return &apiv1.Checklist{Name: "this-week"}, nil
			}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", `W/"stale-etag"`, identity)
		})

		It("should return ErrPreconditionFailed", func() {
			Expect(err).To(MatchError(caldav.ErrPreconditionFailed))
		})

		It("should not call mutator.DeleteItem", func() {
			Expect(deleteFnCalled).To(BeFalse())
		})
	})

	When("the requested uid is unknown", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(now),
				}},
			}}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXZZZZZZZZZZZZZZZZZZZZZZ", `W/"some-etag"`, identity)
		})

		It("should return ErrItemNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrItemNotFound))
		})
	})

	When("the requested uid is in the tombstone list", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 3,
				}},
			}}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXCCCCCCCCCCCCCCCCCCCCCC", `W/"any-etag"`, identity)
		})

		It("should return ErrItemDeleted", func() {
			Expect(err).To(MatchError(caldav.ErrItemDeleted))
		})
	})

	When("the mutator returns FailedPrecondition", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(now),
				}},
			}}
			fake.deleteFn = func(_ context.Context, _, _, _ string, _ *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
				return nil, status.Error(codes.FailedPrecondition, "stale")
			}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", "", identity)
		})

		It("should return ErrPreconditionFailed", func() {
			Expect(err).To(MatchError(caldav.ErrPreconditionFailed))
		})
	})

	When("the mutator returns ErrItemNotFound", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(now),
				}},
			}}
			fake.deleteFn = func(_ context.Context, _, _, _ string, _ *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
				return nil, checklistmutator.ErrItemNotFound
			}
			err = backend.DeleteItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", "", identity)
		})

		It("should return ErrItemNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrItemNotFound))
		})
	})
})

// buildVTODOBody assembles a minimal-but-valid VCALENDAR/VTODO body
// from the given UID and SUMMARY for use by the PutItem tests.
func buildVTODOBody(uid, summary string) []byte {
	return []byte(
		"BEGIN:VCALENDAR\r\n" +
			"VERSION:2.0\r\n" +
			"PRODID:-//test//EN\r\n" +
			"BEGIN:VTODO\r\n" +
			"UID:" + uid + "\r\n" +
			"SUMMARY:" + summary + "\r\n" +
			"END:VTODO\r\n" +
			"END:VCALENDAR\r\n",
	)
}

var _ = Describe("defaultBackend.PutItem", func() {
	var (
		ctx      context.Context
		now      time.Time
		fake     *fakeMutator
		backend  caldav.CalendarBackend
		identity tailscale.IdentityValue
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
		identity = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
	})

	When("the uid does not exist and the body is valid", func() {
		var (
			newETag         string
			created         bool
			err             error
			capturedArgs    checklistmutator.UpsertFromCalDAVArgs
			capturedPage    string
			capturedList    string
			capturedUID     string
			capturedIfMatch string
			capturedINM     string
			itemUpdated     time.Time
		)

		BeforeEach(func() {
			itemUpdated = now
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
			}}
			fake.upsertFn = func(_ context.Context, page, listName, uid string, args checklistmutator.UpsertFromCalDAVArgs, ifMatch, ifNoneMatch string, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
				capturedPage = page
				capturedList = listName
				capturedUID = uid
				capturedArgs = args
				capturedIfMatch = ifMatch
				capturedINM = ifNoneMatch
				return &apiv1.ChecklistItem{
					Uid:       uid,
					Text:      args.Text,
					UpdatedAt: timestamppb.New(itemUpdated),
				}, nil, nil
			}
			body := buildVTODOBody("01HXAAAAAAAAAAAAAAAAAAAAAA", "Buy milk")
			newETag, created, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report created=true", func() {
			Expect(created).To(BeTrue())
		})

		It("should return the per-item ETag", func() {
			Expect(newETag).To(Equal(`W/"` + itemUpdated.Format(time.RFC3339Nano) + `"`))
		})

		It("should pass page through to the mutator", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass list name through to the mutator", func() {
			Expect(capturedList).To(Equal("this-week"))
		})

		It("should pass uid through to the mutator", func() {
			Expect(capturedUID).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should populate args.Text from SUMMARY", func() {
			Expect(capturedArgs.Text).To(Equal("Buy milk"))
		})

		It("should pass ifMatch through to the mutator", func() {
			Expect(capturedIfMatch).To(Equal(""))
		})

		It("should pass ifNoneMatch through to the mutator", func() {
			Expect(capturedINM).To(Equal(""))
		})
	})

	When("the uid already exists and the body is valid", func() {
		var (
			newETag     string
			created     bool
			err         error
			itemUpdated time.Time
		)

		BeforeEach(func() {
			itemUpdated = now
			existing := &apiv1.ChecklistItem{
				Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
				Text:      "Buy milk (old)",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
			}
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items:     []*apiv1.ChecklistItem{existing},
			}}
			fake.upsertFn = func(_ context.Context, _, _, uid string, args checklistmutator.UpsertFromCalDAVArgs, _, _ string, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
				return &apiv1.ChecklistItem{
					Uid:       uid,
					Text:      args.Text,
					UpdatedAt: timestamppb.New(itemUpdated),
				}, nil, nil
			}
			body := buildVTODOBody("01HXAAAAAAAAAAAAAAAAAAAAAA", "Buy milk (new)")
			newETag, created, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report created=false", func() {
			Expect(created).To(BeFalse())
		})

		It("should return the per-item ETag from the upsert result", func() {
			Expect(newETag).To(Equal(`W/"` + itemUpdated.Format(time.RFC3339Nano) + `"`))
		})
	})

	When("the body fails to parse as iCalendar", func() {
		var err error

		BeforeEach(func() {
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", []byte("NOT ICAL"), "", "", identity)
		})

		It("should return ErrInvalidBody", func() {
			Expect(err).To(MatchError(caldav.ErrInvalidBody))
		})
	})

	When("the body has no VTODO component", func() {
		var err error

		BeforeEach(func() {
			body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\nEND:VCALENDAR\r\n")
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should return ErrInvalidBody", func() {
			Expect(err).To(MatchError(caldav.ErrInvalidBody))
		})
	})

	When("the body's VTODO is missing UID", func() {
		var err error

		BeforeEach(func() {
			body := []byte(
				"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
					"BEGIN:VTODO\r\nSUMMARY:no uid\r\nEND:VTODO\r\nEND:VCALENDAR\r\n",
			)
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should return ErrInvalidBody", func() {
			Expect(err).To(MatchError(caldav.ErrInvalidBody))
		})
	})

	When("the body's DESCRIPTION exceeds the size cap", func() {
		var err error

		BeforeEach(func() {
			huge := strings.Repeat("a", icalcodec.DescriptionMaxBytes+1)
			body := []byte(
				"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\n" +
					"BEGIN:VTODO\r\nUID:01HXAAAAAAAAAAAAAAAAAAAAAA\r\nSUMMARY:big\r\n" +
					"DESCRIPTION:" + huge + "\r\n" +
					"END:VTODO\r\nEND:VCALENDAR\r\n",
			)
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should return ErrDescriptionTooLarge", func() {
			Expect(err).To(MatchError(caldav.ErrDescriptionTooLarge))
		})
	})

	When("the body's UID does not match the path uid", func() {
		var err error

		BeforeEach(func() {
			body := buildVTODOBody("01HXBBBBBBBBBBBBBBBBBBBBBB", "Buy milk")
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "", identity)
		})

		It("should return ErrUIDMismatch", func() {
			Expect(err).To(MatchError(caldav.ErrUIDMismatch))
		})
	})

	When("the mutator returns FailedPrecondition", func() {
		var err error

		BeforeEach(func() {
			fake.upsertFn = func(_ context.Context, _, _, _ string, _ checklistmutator.UpsertFromCalDAVArgs, _, _ string, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
				return nil, nil, status.Error(codes.FailedPrecondition, "stale")
			}
			body := buildVTODOBody("01HXAAAAAAAAAAAAAAAAAAAAAA", "Buy milk")
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, `W/"stale"`, "", identity)
		})

		It("should return ErrPreconditionFailed", func() {
			Expect(err).To(MatchError(caldav.ErrPreconditionFailed))
		})
	})

	When("ifNoneMatch is * and the item exists and the mutator returns FailedPrecondition", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 5,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				}},
			}}
			fake.upsertFn = func(_ context.Context, _, _, _ string, _ checklistmutator.UpsertFromCalDAVArgs, _, _ string, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
				return nil, nil, status.Error(codes.FailedPrecondition, "exists")
			}
			body := buildVTODOBody("01HXAAAAAAAAAAAAAAAAAAAAAA", "Buy milk")
			_, _, err = backend.PutItem(ctx, "shopping", "this-week", "01HXAAAAAAAAAAAAAAAAAAAAAA", body, "", "*", identity)
		})

		It("should return ErrPreconditionFailed", func() {
			Expect(err).To(MatchError(caldav.ErrPreconditionFailed))
		})
	})
})

var _ = Describe("defaultBackend.SyncCollection", func() {
	var (
		ctx     context.Context
		now     time.Time
		fake    *fakeMutator
		backend caldav.CalendarBackend
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC)
		fake = &fakeMutator{pages: map[string][]*apiv1.Checklist{}}
		backend = caldav.NewBackend(fake, "https://wiki.example.com", fixedNow(now))
	})

	When("the client supplies an empty token (initial sync)", func() {
		var (
			newToken    string
			changed     []caldav.CalendarItem
			deletedUIDs []string
			err         error
			item1       *apiv1.ChecklistItem
			item2       *apiv1.ChecklistItem
		)

		BeforeEach(func() {
			itemUpdated := now.Add(-30 * time.Minute)
			item1 = &apiv1.ChecklistItem{
				Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
				Text:      "Buy milk",
				UpdatedAt: timestamppb.New(itemUpdated),
			}
			item2 = &apiv1.ChecklistItem{
				Uid:       "01HXBBBBBBBBBBBBBBBBBBBBBB",
				Text:      "Buy bread",
				UpdatedAt: timestamppb.New(itemUpdated),
			}
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 7,
				Items:     []*apiv1.ChecklistItem{item1, item2},
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 3,
				}},
			}}
			newToken, changed, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the current collection sync-token", func() {
			Expect(newToken).To(Equal("http://simple-wiki.local/ns/sync/7"))
		})

		It("should emit every live item as changed", func() {
			Expect(changed).To(HaveLen(2))
		})

		It("should set each changed item's UID from the source", func() {
			uids := []string{changed[0].UID, changed[1].UID}
			Expect(uids).To(ConsistOf("01HXAAAAAAAAAAAAAAAAAAAAAA", "01HXBBBBBBBBBBBBBBBBBBBBBB"))
		})

		It("should produce VTODO bytes for each changed item", func() {
			Expect(string(changed[0].ICalBytes)).To(ContainSubstring("BEGIN:VTODO"))
			Expect(string(changed[1].ICalBytes)).To(ContainSubstring("BEGIN:VTODO"))
		})

		It("should not emit any deleted uids", func() {
			Expect(deletedUIDs).To(BeEmpty())
		})
	})

	When("the client's token equals the current collection sync-token", func() {
		var (
			newToken    string
			changed     []caldav.CalendarItem
			deletedUIDs []string
			err         error
		)

		BeforeEach(func() {
			itemUpdated := now.Add(-30 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 12,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(itemUpdated),
				}},
			}}
			newToken, changed, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "http://simple-wiki.local/ns/sync/12")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the same collection sync-token", func() {
			Expect(newToken).To(Equal("http://simple-wiki.local/ns/sync/12"))
		})

		It("should emit no changed items", func() {
			Expect(changed).To(BeEmpty())
		})

		It("should emit no deleted uids", func() {
			Expect(deletedUIDs).To(BeEmpty())
		})
	})

	When("a tombstone has SyncToken greater than the client's token", func() {
		var (
			newToken    string
			changed     []caldav.CalendarItem
			deletedUIDs []string
			err         error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 9,
				Items:     []*apiv1.ChecklistItem{},
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-time.Minute)),
					SyncToken: 9,
				}},
			}}
			newToken, changed, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "http://simple-wiki.local/ns/sync/5")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should advance the sync-token to the collection's current value", func() {
			Expect(newToken).To(Equal("http://simple-wiki.local/ns/sync/9"))
		})

		It("should emit the tombstoned uid as deleted", func() {
			Expect(deletedUIDs).To(ConsistOf("01HXCCCCCCCCCCCCCCCCCCCCCC"))
		})

		It("should emit no changed items", func() {
			Expect(changed).To(BeEmpty())
		})
	})

	When("the client is far behind the current sync-token", func() {
		var (
			changed     []caldav.CalendarItem
			deletedUIDs []string
			err         error
		)

		BeforeEach(func() {
			itemUpdated := now.Add(-30 * time.Minute)
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 50,
				Items: []*apiv1.ChecklistItem{{
					Uid:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
					Text:      "Buy milk",
					UpdatedAt: timestamppb.New(itemUpdated),
				}, {
					Uid:       "01HXBBBBBBBBBBBBBBBBBBBBBB",
					Text:      "Buy bread",
					UpdatedAt: timestamppb.New(itemUpdated),
				}},
			}}
			_, changed, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "http://simple-wiki.local/ns/sync/2")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should over-emit by returning every live item", func() {
			Expect(changed).To(HaveLen(2))
		})

		It("should emit no deleted uids when no tombstones are newer than the client's token", func() {
			Expect(deletedUIDs).To(BeEmpty())
		})
	})

	When("a legacy tombstone has SyncToken zero and the client's token parses to zero", func() {
		var (
			deletedUIDs []string
			err         error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 4,
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 0,
				}},
			}}
			_, _, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should emit the legacy tombstone uid as deleted", func() {
			Expect(deletedUIDs).To(ConsistOf("01HXCCCCCCCCCCCCCCCCCCCCCC"))
		})
	})

	When("a legacy tombstone has SyncToken zero and the client's token is non-zero", func() {
		var (
			deletedUIDs []string
			err         error
		)

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 4,
				Tombstones: []*apiv1.Tombstone{{
					Uid:       "01HXCCCCCCCCCCCCCCCCCCCCCC",
					DeletedAt: timestamppb.New(now.Add(-2 * time.Hour)),
					SyncToken: 0,
				}},
			}}
			_, _, deletedUIDs, err = backend.SyncCollection(ctx, "shopping", "this-week", "http://simple-wiki.local/ns/sync/2")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should suppress the legacy tombstone", func() {
			Expect(deletedUIDs).To(BeEmpty())
		})
	})

	When("the named (page, list) does not exist", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = backend.SyncCollection(ctx, "missing-page", "this-week", "")
		})

		It("should return ErrCollectionNotFound", func() {
			Expect(err).To(MatchError(caldav.ErrCollectionNotFound))
		})
	})

	When("the client supplies a malformed sync-token", func() {
		var err error

		BeforeEach(func() {
			fake.pages["shopping"] = []*apiv1.Checklist{{
				Name:      "this-week",
				UpdatedAt: timestamppb.New(now.Add(-time.Hour)),
				SyncToken: 7,
			}}
			_, _, _, err = backend.SyncCollection(ctx, "shopping", "this-week", "not-a-uri-at-all")
		})

		It("should return ErrInvalidSyncToken", func() {
			Expect(err).To(MatchError(caldav.ErrInvalidSyncToken))
		})
	})
})
