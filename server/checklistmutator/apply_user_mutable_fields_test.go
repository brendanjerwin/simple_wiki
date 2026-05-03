//revive:disable:dot-imports
// Internal tests for applyUserMutableFields. Lives in package
// checklistmutator (not _test) so it can exercise the unexported helper.
package checklistmutator

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

var _ = Describe("applyUserMutableFields", func() {
	var (
		item    *apiv1.ChecklistItem
		args    UpdateItemArgs
		changed bool
	)

	BeforeEach(func() {
		desc := "old desc"
		alarm := "old alarm"
		item = &apiv1.ChecklistItem{
			Uid:          "u",
			Text:         "old text",
			Tags:         []string{"a"},
			Description:  &desc,
			Due:          timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			AlarmPayload: &alarm,
			SortOrder:    1000,
		}
		args = UpdateItemArgs{}
	})

	When("args has no fields set", func() {
		BeforeEach(func() {
			changed = applyUserMutableFields(item, args)
		})

		It("should report no change", func() {
			Expect(changed).To(BeFalse())
		})

		It("should leave text unchanged", func() {
			Expect(item.Text).To(Equal("old text"))
		})
	})

	When("args.Text differs from current text", func() {
		BeforeEach(func() {
			newText := "new text"
			args.Text = &newText
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should update text", func() {
			Expect(item.Text).To(Equal("new text"))
		})
	})

	When("args.Text matches current text", func() {
		BeforeEach(func() {
			same := "old text"
			args.Text = &same
			changed = applyUserMutableFields(item, args)
		})

		It("should report no change", func() {
			Expect(changed).To(BeFalse())
		})
	})

	When("TagsSet is true and tags differ", func() {
		BeforeEach(func() {
			args.TagsSet = true
			args.Tags = []string{"b", "c"}
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should replace tags", func() {
			Expect(item.Tags).To(Equal([]string{"b", "c"}))
		})
	})

	When("TagsSet is false even with non-nil Tags", func() {
		BeforeEach(func() {
			args.Tags = []string{"ignored"}
			changed = applyUserMutableFields(item, args)
		})

		It("should report no change", func() {
			Expect(changed).To(BeFalse())
		})

		It("should leave tags unchanged", func() {
			Expect(item.Tags).To(Equal([]string{"a"}))
		})
	})

	When("DescriptionSet is true and value differs", func() {
		BeforeEach(func() {
			newDesc := "new desc"
			args.DescriptionSet = true
			args.Description = &newDesc
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should update description", func() {
			Expect(*item.Description).To(Equal("new desc"))
		})
	})

	When("DescriptionSet is true and Description is nil (clear)", func() {
		BeforeEach(func() {
			args.DescriptionSet = true
			args.Description = nil
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should clear description", func() {
			Expect(item.Description).To(BeNil())
		})
	})

	When("DueSet is true with new due", func() {
		BeforeEach(func() {
			newDue := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
			args.DueSet = true
			args.Due = &newDue
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should update due", func() {
			Expect(item.Due.AsTime()).To(Equal(time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)))
		})
	})

	When("DueSet is true with nil Due (clear)", func() {
		BeforeEach(func() {
			args.DueSet = true
			args.Due = nil
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should clear due", func() {
			Expect(item.Due).To(BeNil())
		})
	})

	When("AlarmPayloadSet is true with new value", func() {
		BeforeEach(func() {
			newAlarm := "new alarm"
			args.AlarmPayloadSet = true
			args.AlarmPayload = &newAlarm
			changed = applyUserMutableFields(item, args)
		})

		It("should report a change", func() {
			Expect(changed).To(BeTrue())
		})

		It("should update alarm payload", func() {
			Expect(*item.AlarmPayload).To(Equal("new alarm"))
		})
	})
})
