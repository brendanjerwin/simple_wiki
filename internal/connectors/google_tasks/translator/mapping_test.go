//revive:disable:dot-imports
package translator_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
)

var _ = Describe("TaskToChecklistItem", func() {
	When("converting a basic needsAction task", func() {
		var (
			item *apiv1.ChecklistItem
			err  error
		)

		BeforeEach(func() {
			item, err = translator.TaskToChecklistItem(translator.Task{
				ID:       "task-1",
				Title:    "Buy milk",
				Status:   translator.TaskStatusNeedsAction,
				Position: "00000000000000001000",
				Updated:  time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC),
			})
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set text from the title", func() {
			Expect(item.GetText()).To(Equal("Buy milk"))
		})

		It("should set checked to false", func() {
			Expect(item.GetChecked()).To(BeFalse())
		})

		It("should populate sort_order from position", func() {
			Expect(item.GetSortOrder()).To(Equal(int64(1000)))
		})

		It("should not set the uid", func() {
			Expect(item.GetUid()).To(Equal(""))
		})

		It("should propagate updated_at", func() {
			Expect(item.GetUpdatedAt().AsTime().UTC()).To(Equal(time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)))
		})
	})

	When("the task is completed", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title:     "Done already",
				Status:    translator.TaskStatusCompleted,
				Completed: time.Date(2026, 4, 25, 18, 30, 0, 0, time.UTC),
			})
		})

		It("should set checked to true", func() {
			Expect(item.GetChecked()).To(BeTrue())
		})

		It("should populate completed_at", func() {
			Expect(item.GetCompletedAt().AsTime().UTC()).To(Equal(time.Date(2026, 4, 25, 18, 30, 0, 0, time.UTC)))
		})
	})

	When("the title contains #tags", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title: "Buy oat milk #urgent #kirsten",
			})
		})

		It("should strip the #tag tokens from text", func() {
			Expect(item.GetText()).To(Equal("Buy oat milk"))
		})

		It("should extract urgent into tags", func() {
			Expect(item.GetTags()).To(ContainElement("urgent"))
		})

		It("should extract kirsten into tags", func() {
			Expect(item.GetTags()).To(ContainElement("kirsten"))
		})
	})

	When("notes carry a trailing wiki uid marker", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			notes := "fresh local" + translator.WikiUIDMarker("01HABC")
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title: "Apples",
				Notes: notes,
			})
		})

		It("should populate description with the marker stripped", func() {
			Expect(item.GetDescription()).To(Equal("fresh local"))
		})

		It("should not surface the uid in description", func() {
			Expect(item.GetDescription()).NotTo(ContainSubstring("wiki:uid"))
		})
	})

	When("notes have no marker", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title: "Apples",
				Notes: "context only",
			})
		})

		It("should populate description verbatim", func() {
			Expect(item.GetDescription()).To(Equal("context only"))
		})
	})

	When("notes are empty", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title: "Apples",
				Notes: "",
			})
		})

		It("should leave description unset", func() {
			Expect(item.Description).To(BeNil())
		})
	})

	When("the task has a due date", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item, _ = translator.TaskToChecklistItem(translator.Task{
				Title: "Pay rent",
				Due:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			})
		})

		It("should propagate the due timestamp", func() {
			Expect(item.GetDue().AsTime().UTC()).To(Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)))
		})
	})
})

var _ = Describe("ChecklistItemToTaskFields", func() {
	When("the item is nil", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			fields = translator.ChecklistItemToTaskFields(nil)
		})

		It("should return zero-value TaskFields", func() {
			Expect(fields).To(Equal(translator.TaskFields{}))
		})
	})

	When("converting a basic unchecked item", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Uid:     "01HABC",
				Text:    "Buy milk",
				Checked: false,
			})
		})

		It("should encode title from text", func() {
			Expect(fields.Title).To(Equal("Buy milk"))
		})

		It("should set status to needsAction", func() {
			Expect(fields.Status).To(Equal(translator.TaskStatusNeedsAction))
		})

		It("should NOT leak the wiki uid into the user-visible notes field", func() {
			// The wiki↔Tasks binding lives on the Subscription's
			// ItemIDMap, not in the user-visible "Details" field of
			// the Tasks UI. See feedback_no_implementation_in_user_fields.
			Expect(fields.Notes).NotTo(ContainSubstring("wiki:uid"))
		})
	})

	When("the item has tags", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Uid:  "01HDEF",
				Text: "Buy milk",
				Tags: []string{"urgent"},
			})
		})

		It("should encode tags as #tag suffixes in title", func() {
			Expect(fields.Title).To(Equal("Buy milk #urgent"))
		})
	})

	When("the item has a description", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			desc := "fresh local"
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Uid:         "01HXYZ",
				Text:        "Apples",
				Description: &desc,
			})
		})

		It("should pass the description through verbatim", func() {
			Expect(fields.Notes).To(Equal("fresh local"))
		})

		It("should NOT append a wiki uid marker", func() {
			Expect(fields.Notes).NotTo(ContainSubstring("wiki:uid"))
		})
	})

	When("the item is checked", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			completed := time.Date(2026, 4, 25, 19, 0, 0, 0, time.UTC)
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Uid:         "01HABC",
				Text:        "Done",
				Checked:     true,
				CompletedAt: timestamppb.New(completed),
			})
		})

		It("should set status to completed", func() {
			Expect(fields.Status).To(Equal(translator.TaskStatusCompleted))
		})

		It("should propagate completed_at to Completed", func() {
			Expect(fields.Completed.UTC()).To(Equal(time.Date(2026, 4, 25, 19, 0, 0, 0, time.UTC)))
		})
	})

	When("the item has a due date", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Uid:  "01HABC",
				Text: "Pay rent",
				Due:  timestamppb.New(time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)),
			})
		})

		It("should propagate due", func() {
			Expect(fields.Due.UTC()).To(Equal(time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)))
		})
	})

	When("the item has no uid", func() {
		var fields translator.TaskFields

		BeforeEach(func() {
			fields = translator.ChecklistItemToTaskFields(&apiv1.ChecklistItem{
				Text: "Brand new",
			})
		})

		It("should not include a wiki uid marker", func() {
			Expect(fields.Notes).NotTo(ContainSubstring("wiki:uid"))
		})

		It("should produce empty notes when there is no description", func() {
			Expect(fields.Notes).To(Equal(""))
		})
	})
})

var _ = Describe("Round trip ChecklistItem → TaskFields → Task → ChecklistItem", func() {
	When("an item with text, tags, description, checked, due, sort_order is round-tripped", func() {
		var (
			original *apiv1.ChecklistItem
			out      *apiv1.ChecklistItem
		)

		BeforeEach(func() {
			desc := "fresh local"
			completed := time.Date(2026, 4, 25, 19, 0, 0, 0, time.UTC)
			due := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

			original = &apiv1.ChecklistItem{
				Uid:         "01HABC",
				Text:        "Buy oat milk",
				Tags:        []string{"urgent", "kirsten"},
				Description: &desc,
				Checked:     true,
				Due:         timestamppb.New(due),
				CompletedAt: timestamppb.New(completed),
				SortOrder:   1000,
			}

			fields := translator.ChecklistItemToTaskFields(original)

			// Imagine the gateway pushed `fields`, Google echoed back
			// a Task with the same content. Position is server-issued
			// from sort_order; we mirror that here.
			task := translator.Task{
				ID:        "tasks-server-id",
				Title:     fields.Title,
				Notes:     fields.Notes,
				Status:    fields.Status,
				Due:       fields.Due,
				Completed: fields.Completed,
				Position:  translator.SortOrderToPosition(original.GetSortOrder()),
			}

			var err error
			out, err = translator.TaskToChecklistItem(task)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve text", func() {
			Expect(out.GetText()).To(Equal(original.GetText()))
		})

		It("should preserve the urgent tag", func() {
			Expect(out.GetTags()).To(ContainElement("urgent"))
		})

		It("should preserve the kirsten tag", func() {
			Expect(out.GetTags()).To(ContainElement("kirsten"))
		})

		It("should preserve description", func() {
			Expect(out.GetDescription()).To(Equal(original.GetDescription()))
		})

		It("should preserve checked", func() {
			Expect(out.GetChecked()).To(Equal(original.GetChecked()))
		})

		It("should preserve due", func() {
			Expect(out.GetDue().AsTime().UTC()).To(Equal(original.GetDue().AsTime().UTC()))
		})

		It("should preserve completed_at", func() {
			Expect(out.GetCompletedAt().AsTime().UTC()).To(Equal(original.GetCompletedAt().AsTime().UTC()))
		})

		It("should preserve sort_order", func() {
			Expect(out.GetSortOrder()).To(Equal(original.GetSortOrder()))
		})

		It("should not surface the wiki uid marker in the round-tripped description", func() {
			Expect(out.GetDescription()).NotTo(ContainSubstring("wiki:uid"))
		})
	})
})
