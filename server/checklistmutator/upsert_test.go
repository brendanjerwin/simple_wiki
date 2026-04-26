//revive:disable:dot-imports
package checklistmutator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("UpsertFromCalDAV", func() {
	var (
		store   *fakeStore
		clock   *fakeClock
		ulids   *ulid.SequenceGenerator
		mutator *checklistmutator.Mutator
		ctx     context.Context
		alice   tailscale.IdentityValue
		bot     tailscale.IdentityValue
	)

	BeforeEach(func() {
		store = newFakeStore()
		clock = newFakeClock(time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC))
		ulids = ulid.NewSequenceGenerator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
			"01HXCCCCCCCCCCCCCCCCCCCCCC",
		)
		mutator = checklistmutator.New(store, clock, ulids)
		ctx = context.Background()
		alice = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		bot = tailscale.NewAgentIdentity("scheduler@example.com", "Scheduler", "scheduler-bot")
	})

	When("uid is unknown (create path)", func() {
		const newUID = "01HXNEWNEWNEWNEWNEWNEWNEWN"
		var (
			item      *apiv1.ChecklistItem
			checklist *apiv1.Checklist
			err       error
		)

		BeforeEach(func() {
			args := checklistmutator.UpsertFromCalDAVArgs{
				Text: "Buy milk",
				Tags: []string{"urgent"},
			}
			item, checklist, err = mutator.UpsertFromCalDAV(ctx, "p", "list", newUID, args, "", "", alice)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the client-provided uid", func() {
			Expect(item.Uid).To(Equal(newUID))
		})

		It("should populate text from args", func() {
			Expect(item.Text).To(Equal("Buy milk"))
		})

		It("should populate tags from args", func() {
			Expect(item.Tags).To(Equal([]string{"urgent"}))
		})

		It("should stamp created_at to clock.Now()", func() {
			Expect(item.CreatedAt.AsTime()).To(Equal(clock.Now()))
		})

		It("should stamp updated_at to clock.Now()", func() {
			Expect(item.UpdatedAt.AsTime()).To(Equal(clock.Now()))
		})

		It("should advance sync_token by exactly 1", func() {
			Expect(checklist.SyncToken).To(Equal(int64(1)))
		})

		It("should record automated based on identity.IsAgent()", func() {
			Expect(item.Automated).To(BeFalse())
		})
	})

	When("uid is unknown and the caller is an agent", func() {
		It("should record automated=true on the new item", func() {
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "Auto"}
			item, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", "01HXAUTOAUTOAUTOAUTOAUTOAU", args, "", "", bot)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.Automated).To(BeTrue())
		})
	})

	When("uid is unknown and args.Created is provided", func() {
		It("should honor args.Created for created_at", func() {
			created := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "Backdated", Created: &created}
			item, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", "01HXOFFLINEOFFLINEOFFLINEX", args, "", "", alice)
			Expect(err).NotTo(HaveOccurred())
			Expect(item.CreatedAt.AsTime()).To(Equal(created))
		})
	})

	When("uid is known (update path)", func() {
		var existingUID string
		var initialUpdatedAtETag string

		BeforeEach(func() {
			added, list, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "Original"}, alice)
			existingUID = added.Uid
			initialUpdatedAtETag = list.UpdatedAt.AsTime().Format(time.RFC3339Nano)
			clock.advance(time.Minute)
		})

		When("text and tags both change", func() {
			var (
				item      *apiv1.ChecklistItem
				checklist *apiv1.Checklist
				err       error
			)

			BeforeEach(func() {
				args := checklistmutator.UpsertFromCalDAVArgs{Text: "Updated", Tags: []string{"x", "y"}}
				item, checklist, err = mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "", "", alice)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should apply text", func() {
				Expect(item.Text).To(Equal("Updated"))
			})

			It("should apply tags", func() {
				Expect(item.Tags).To(Equal([]string{"x", "y"}))
			})

			It("should bump sync_token by exactly 1 (one upsert call = one bump)", func() {
				// AddItem in the BeforeEach already bumped to 1; this upsert bumps to 2.
				Expect(checklist.SyncToken).To(Equal(int64(2)))
			})

			It("should advance updated_at to clock.Now()", func() {
				Expect(item.UpdatedAt.AsTime()).To(Equal(clock.Now()))
			})
		})

		When("If-Match matches the current ETag", func() {
			It("should succeed", func() {
				// Item ETag = item.UpdatedAt. Initial item updated_at == list.updated_at after AddItem.
				etag := initialUpdatedAtETag
				newText := "OK"
				args := checklistmutator.UpsertFromCalDAVArgs{Text: newText}
				_, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, etag, "", alice)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("If-Match does not match the current ETag", func() {
			It("should return FailedPrecondition", func() {
				args := checklistmutator.UpsertFromCalDAVArgs{Text: "stale"}
				_, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "1999-01-01T00:00:00Z", "", alice)
				Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
			})
		})

		When("If-None-Match: * is sent for an existing item", func() {
			It("should return FailedPrecondition", func() {
				args := checklistmutator.UpsertFromCalDAVArgs{Text: "duplicate"}
				_, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "", "*", alice)
				Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
			})
		})
	})

	When("If-Match is set but the item does not exist", func() {
		It("should return FailedPrecondition", func() {
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "ghost"}
			_, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", "01HXMISSINGMISSINGMISSINGM", args, "1999-01-01T00:00:00Z", "", alice)
			Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
		})
	})

	When("checked transitions false to true", func() {
		var existingUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "Pending"}, alice)
			existingUID = added.Uid
			clock.advance(time.Minute)
		})

		When("args.CompletedAt is nil", func() {
			var item *apiv1.ChecklistItem

			BeforeEach(func() {
				args := checklistmutator.UpsertFromCalDAVArgs{Text: "Pending", Checked: true}
				item, _, _ = mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "", "", alice)
			})

			It("should stamp completed_at = clock.Now()", func() {
				Expect(item.CompletedAt.AsTime()).To(Equal(clock.Now()))
			})

			It("should set completed_by from identity.Name()", func() {
				Expect(item.CompletedBy).NotTo(BeNil())
				Expect(*item.CompletedBy).To(Equal("alice@example.com"))
			})

			It("should set checked=true", func() {
				Expect(item.Checked).To(BeTrue())
			})
		})

		When("args.CompletedAt is provided", func() {
			It("should honor the explicit timestamp", func() {
				completedAt := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
				args := checklistmutator.UpsertFromCalDAVArgs{Text: "Pending", Checked: true, CompletedAt: &completedAt}
				item, _, _ := mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "", "", alice)
				Expect(item.CompletedAt.AsTime()).To(Equal(completedAt))
			})
		})
	})

	When("checked transitions true to false", func() {
		var existingUID string

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "Done"}, alice)
			existingUID = added.Uid
			_, _, _ = mutator.ToggleItem(ctx, "p", "list", existingUID, nil, alice)
			clock.advance(time.Minute)
		})

		var resultItem *apiv1.ChecklistItem

		BeforeEach(func() {
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "Done", Checked: false}
			resultItem, _, _ = mutator.UpsertFromCalDAV(ctx, "p", "list", existingUID, args, "", "", alice)
		})

		It("should clear completed_at", func() {
			Expect(resultItem.CompletedAt).To(BeNil())
		})

		It("should clear completed_by", func() {
			Expect(resultItem.CompletedBy).To(BeNil())
		})

		It("should set checked to false", func() {
			Expect(resultItem.Checked).To(BeFalse())
		})
	})

	When("uid is empty", func() {
		It("should return InvalidArgument", func() {
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "x"}
			_, _, err := mutator.UpsertFromCalDAV(ctx, "p", "list", "", args, "", "", alice)
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})
	})

	When("an expired tombstone exists at upsert time", func() {
		var (
			deletedUID string
			checklist  *apiv1.Checklist
		)

		BeforeEach(func() {
			added, _, _ := mutator.AddItem(ctx, "p", "list", checklistmutator.AddItemArgs{Text: "doomed"}, alice)
			deletedUID = added.Uid
			_, _ = mutator.DeleteItem(ctx, "p", "list", deletedUID, nil, alice)
			clock.advance(checklistmutator.TombstoneTTL + time.Hour)
			args := checklistmutator.UpsertFromCalDAVArgs{Text: "fresh"}
			_, checklist, _ = mutator.UpsertFromCalDAV(ctx, "p", "list", "01HXFRESHFRESHFRESHFRESHFR", args, "", "", alice)
		})

		It("should prune the expired tombstone", func() {
			Expect(checklist.Tombstones).To(BeEmpty())
		})
	})
})
