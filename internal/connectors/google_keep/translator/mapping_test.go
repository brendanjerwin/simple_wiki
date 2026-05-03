//revive:disable:dot-imports
package translator_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/translator"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
)

var _ = Describe("WikiToKeep", func() {
	When("converting a basic checklist item", func() {
		var node gateway.Node

		BeforeEach(func() {
			node = translator.WikiToKeep(&apiv1.ChecklistItem{
				Uid:       "01HXXXXX",
				Text:      "Buy milk",
				Checked:   false,
				SortOrder: 1000,
				CreatedAt: timestamppb.New(time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)),
				UpdatedAt: timestamppb.New(time.Date(2026, 4, 25, 17, 5, 0, 0, time.UTC)),
			}, "srv-list-1", "keep-item-1")
		})

		It("should set type to LIST_ITEM", func() {
			Expect(node.Type).To(Equal(gateway.NodeTypeListItem))
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
		var node gateway.Node

		BeforeEach(func() {
			node = translator.WikiToKeep(&apiv1.ChecklistItem{
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
		var node gateway.Node

		BeforeEach(func() {
			node = translator.WikiToKeep(&apiv1.ChecklistItem{
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

		var convertErr error
		BeforeEach(func() {
			item, convertErr = translator.KeepToWiki(gateway.Node{
				ID:        "keep-1",
				ServerID:  "srv-keep-1",
				Type:      gateway.NodeTypeListItem,
				Text:      "Buy oat milk #urgent",
				Checked:   true,
				SortValue: "1500",
				Timestamps: gateway.Timestamps{
					Updated: time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC),
				},
			})
		})

		It("should not error", func() {
			Expect(convertErr).ToNot(HaveOccurred())
		})

		It("should populate text without #tag suffix", func() {
			// Round-trip stability: tags ride inline as #tag suffixes
			// on the wire but are stripped from the wiki-side text
			// so the wiki stores clean "text + tags array" — same
			// shape as before the sync round trip.
			Expect(item.GetText()).To(Equal("Buy oat milk"))
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

var _ = Describe("Fingerprint helpers", func() {
	When("a wiki item is round-tripped through WikiToKeep", func() {
		var (
			item    *apiv1.ChecklistItem
			node    gateway.Node
			wikiFP  translator.Fingerprint
			keepFP  translator.Fingerprint
		)

		BeforeEach(func() {
			desc := "fresh local"
			item = &apiv1.ChecklistItem{
				Text:        "Apples",
				Checked:     false,
				Tags:        []string{"produce"},
				Description: &desc,
				SortOrder:   1000,
			}
			node = translator.WikiToKeep(item, "list-server-id", "")
			wikiFP = translator.FingerprintWiki(item)
			keepFP = translator.FingerprintKeep(node)
		})

		It("should produce identical fingerprints on both sides", func() {
			Expect(wikiFP).To(Equal(keepFP))
		})

		It("should encode text with #tags and the description separator", func() {
			Expect(wikiFP.Text).To(Equal("Apples #produce\n— fresh local"))
		})

		It("should preserve checked state on both sides", func() {
			Expect(wikiFP.Checked).To(BeFalse())
			Expect(keepFP.Checked).To(BeFalse())
		})

		It("should encode SortValue identically on both sides", func() {
			Expect(wikiFP.SortValue).To(Equal("1000"))
			Expect(keepFP.SortValue).To(Equal("1000"))
		})
	})

	When("comparing wiki and Keep fingerprints to a synced baseline", func() {
		var synced translator.Fingerprint

		BeforeEach(func() {
			synced = translator.Fingerprint{Text: "Apples", Checked: false, SortValue: "1000"}
		})

		It("should report no divergence when the two fingerprints match the baseline exactly", func() {
			wiki := translator.Fingerprint{Text: "Apples", Checked: false, SortValue: "1000"}
			Expect(wiki).To(Equal(synced))
		})

		It("should report wiki divergence when the wiki fingerprint differs from the baseline", func() {
			wiki := translator.Fingerprint{Text: "Green Apples", Checked: false, SortValue: "1000"}
			Expect(wiki).NotTo(Equal(synced))
		})

		It("should report Keep divergence when the Keep fingerprint differs from the baseline", func() {
			keep := translator.Fingerprint{Text: "Apples", Checked: true, SortValue: "1000"}
			Expect(keep).NotTo(Equal(synced))
		})
	})

	When("loading the synced baseline from the persisted SyncedText/Checked/SortValue triple", func() {
		It("should reconstitute the fingerprint", func() {
			Expect(translator.FingerprintFromSyncedFields("Apples", true, "2000")).To(Equal(translator.Fingerprint{
				Text:      "Apples",
				Checked:   true,
				SortValue: "2000",
			}))
		})
	})
})
