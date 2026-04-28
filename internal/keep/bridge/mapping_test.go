//revive:disable:dot-imports
package bridge_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

var _ = Describe("WikiToKeep", func() {
	When("converting a basic checklist item", func() {
		var node protocol.Node

		BeforeEach(func() {
			node = bridge.WikiToKeep(&apiv1.ChecklistItem{
				Uid:       "01HXXXXX",
				Text:      "Buy milk",
				Checked:   false,
				SortOrder: 1000,
				CreatedAt: timestamppb.New(time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)),
				UpdatedAt: timestamppb.New(time.Date(2026, 4, 25, 17, 5, 0, 0, time.UTC)),
			}, "srv-list-1", "keep-item-1")
		})

		It("should set type to LIST_ITEM", func() {
			Expect(node.Type).To(Equal(protocol.NodeTypeListItem))
		})

		It("should set parentId to the Keep list serverId", func() {
			Expect(node.ParentID).To(Equal("srv-list-1"))
		})

		It("should pass through the existing keep item id", func() {
			Expect(node.ID).To(Equal("keep-item-1"))
		})

		It("should set text from the wiki text", func() {
			Expect(node.Text).To(Equal("Buy milk"))
		})

		It("should encode sort_order as decimal string", func() {
			Expect(node.SortValue).To(Equal("1000"))
		})

		It("should set updated timestamp", func() {
			Expect(node.Timestamps.Updated.UTC()).To(Equal(time.Date(2026, 4, 25, 17, 5, 0, 0, time.UTC)))
		})
	})

	When("the item has tags not already in text", func() {
		var node protocol.Node

		BeforeEach(func() {
			node = bridge.WikiToKeep(&apiv1.ChecklistItem{
				Text: "Buy milk",
				Tags: []string{"urgent", "kirsten"},
			}, "srv-1", "")
		})

		It("should append #tags to the text", func() {
			Expect(node.Text).To(ContainSubstring("#urgent"))
			Expect(node.Text).To(ContainSubstring("#kirsten"))
		})

		It("should preserve the original text", func() {
			Expect(node.Text).To(HavePrefix("Buy milk"))
		})
	})

	When("a tag is already inline in the text", func() {
		var node protocol.Node

		BeforeEach(func() {
			node = bridge.WikiToKeep(&apiv1.ChecklistItem{
				Text: "Buy milk #urgent",
				Tags: []string{"urgent"},
			}, "srv-1", "")
		})

		It("should not duplicate the inline tag", func() {
			// Single occurrence of #urgent in the encoded text.
			matches := 0
			for i := 0; i+len("#urgent") <= len(node.Text); i++ {
				if node.Text[i:i+len("#urgent")] == "#urgent" {
					matches++
				}
			}
			Expect(matches).To(Equal(1))
		})
	})
})

var _ = Describe("KeepToWiki", func() {
	When("converting a LIST_ITEM with #tags inline", func() {
		var item *apiv1.ChecklistItem

		BeforeEach(func() {
			item = bridge.KeepToWiki(protocol.Node{
				ID:        "keep-1",
				ServerID:  "srv-keep-1",
				Type:      protocol.NodeTypeListItem,
				Text:      "Buy oat milk #urgent",
				Checked:   true,
				SortValue: "1500",
				Timestamps: protocol.Timestamps{
					Updated: time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC),
				},
			})
		})

		It("should populate text", func() {
			Expect(item.GetText()).To(Equal("Buy oat milk #urgent"))
		})

		It("should propagate checked", func() {
			Expect(item.GetChecked()).To(BeTrue())
		})

		It("should extract tags from the text", func() {
			Expect(item.GetTags()).To(ContainElement("urgent"))
		})

		It("should parse sort_order from sortValue", func() {
			Expect(item.GetSortOrder()).To(Equal(int64(1500)))
		})
	})
})
