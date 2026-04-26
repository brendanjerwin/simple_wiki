//revive:disable:dot-imports
package caldav_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

func TestBackend(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "caldav backend")
}

// fakeMutator is a hand-rolled MutatorBackend used by the backend tests.
// It stores a fixed map of pages -> checklists and returns clones so
// callers cannot mutate test fixtures by accident.
type fakeMutator struct {
	pages map[string][]*apiv1.Checklist
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

func (f *fakeMutator) UpsertFromCalDAV(_ context.Context, _, _, _ string, _ checklistmutator.UpsertFromCalDAVArgs, _, _ string, _ tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error) {
	return nil, nil, errors.New("fakeMutator.UpsertFromCalDAV not used in these tests")
}

func (f *fakeMutator) DeleteItem(_ context.Context, _, _, _ string, _ *time.Time, _ tailscale.IdentityValue) (*apiv1.Checklist, error) {
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
			cols      []caldav.CalendarCollection
			err       error
			updated1  time.Time
			updated2  time.Time
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
