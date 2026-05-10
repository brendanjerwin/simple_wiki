//revive:disable:dot-imports
package translator_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/translator"
)

var _ = Describe("NormalizeForSeedMatch", func() {
	When("the text has no description suffix and no tags", func() {
		It("should lowercase and trim whitespace", func() {
			Expect(translator.NormalizeForSeedMatch("  Buy Apples  ")).To(Equal("buy apples"))
		})
	})

	When("the text has a description suffix", func() {
		It("should drop the description and keep only the head line", func() {
			Expect(translator.NormalizeForSeedMatch("Buy Apples\n— from the farmers market")).To(Equal("buy apples"))
		})
	})

	When("the text has inline #tag markers", func() {
		It("should drop the #tag tokens", func() {
			Expect(translator.NormalizeForSeedMatch("Buy Apples #produce #urgent")).To(Equal("buy apples"))
		})
	})

	When("the text has both #tags and a description suffix", func() {
		It("should drop both", func() {
			Expect(translator.NormalizeForSeedMatch("Buy Apples #produce\n— from the farmers market")).To(Equal("buy apples"))
		})
	})

	When("the text is empty", func() {
		It("should return an empty string", func() {
			Expect(translator.NormalizeForSeedMatch("")).To(Equal(""))
		})
	})

	When("the text is only #tags", func() {
		It("should return an empty string", func() {
			Expect(translator.NormalizeForSeedMatch("#produce #urgent")).To(Equal(""))
		})
	})
})

var _ = Describe("MatchWikiItemsToKeepNodes", func() {
	const listSrv = "list-server-id"

	When("the listServerID is empty", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			matches = translator.MatchWikiItemsToKeepNodes(
				[]gateway.Node{
					{Type: gateway.NodeTypeListItem, ServerID: "n1", ParentID: listSrv, Text: "Apples"},
				},
				"",
				[]translator.SeedWikiItem{{UID: "uid-A", Text: "Apples"}},
			)
		})

		It("should return an empty match map", func() {
			Expect(matches).To(BeEmpty())
		})
	})

	When("a wiki item matches a Keep LIST_ITEM under the bound list", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{
					Type:           gateway.NodeTypeListItem,
					ID:             "client-id-A",
					ServerID:       "srv-A",
					ParentID:       listSrv,
					ParentServerID: listSrv,
					BaseVersion:    "v1",
					Text:           "Apples",
				},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-A", Text: "Apples"},
			})
		})

		It("should populate ServerID on the match", func() {
			Expect(matches["uid-A"].ServerID).To(Equal("srv-A"))
		})

		It("should populate BaseVersion on the match", func() {
			Expect(matches["uid-A"].BaseVersion).To(Equal("v1"))
		})

		It("should populate ClientID on the match", func() {
			Expect(matches["uid-A"].ClientID).To(Equal("client-id-A"))
		})
	})

	When("a Keep node belongs to a different list", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{
					Type:           gateway.NodeTypeListItem,
					ServerID:       "srv-other",
					ParentID:       "other-list",
					ParentServerID: "other-list",
					Text:           "Apples",
				},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-A", Text: "Apples"},
			})
		})

		It("should not match it", func() {
			Expect(matches).ToNot(HaveKey("uid-A"))
		})
	})

	When("a Keep node is trashed", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			trashed := time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)
			nodes := []gateway.Node{
				{
					Type:           gateway.NodeTypeListItem,
					ServerID:       "srv-trashed",
					ParentID:       listSrv,
					ParentServerID: listSrv,
					Text:           "Apples",
					Timestamps:     gateway.Timestamps{Trashed: trashed},
				},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-A", Text: "Apples"},
			})
		})

		It("should not match it", func() {
			Expect(matches).ToNot(HaveKey("uid-A"))
		})
	})

	When("two Keep nodes share normalized text", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeListItem, ServerID: "srv-first", ParentID: listSrv, Text: "Apples"},
				{Type: gateway.NodeTypeListItem, ServerID: "srv-second", ParentID: listSrv, Text: "Apples"},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-A", Text: "Apples"},
			})
		})

		It("should pair the wiki item with the first match", func() {
			Expect(matches["uid-A"].ServerID).To(Equal("srv-first"))
		})
	})

	When("a wiki item has an empty UID", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeListItem, ServerID: "srv-A", ParentID: listSrv, Text: "Apples"},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "", Text: "Apples"},
			})
		})

		It("should skip it", func() {
			Expect(matches).To(BeEmpty())
		})
	})

	When("a wiki item normalizes to an empty key (only #tags)", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeListItem, ServerID: "srv-A", ParentID: listSrv, Text: "Apples"},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-empty", Text: "#tag-only"},
			})
		})

		It("should not match it", func() {
			Expect(matches).ToNot(HaveKey("uid-empty"))
		})
	})

	When("a wiki item's text uses inline #tags but matches a plain Keep item by base", func() {
		var matches map[string]translator.SeedMatch

		BeforeEach(func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeListItem, ServerID: "srv-A", ParentID: listSrv, Text: "Apples"},
			}
			matches = translator.MatchWikiItemsToKeepNodes(nodes, listSrv, []translator.SeedWikiItem{
				{UID: "uid-A", Text: "Apples #produce"},
			})
		})

		It("should pair them — base text matches after normalization", func() {
			Expect(matches["uid-A"].ServerID).To(Equal("srv-A"))
		})
	})
})

var _ = Describe("FindListClientID", func() {
	When("the LIST node is present", func() {
		It("should return its client-side id", func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeList, ID: "list-client-id", ServerID: "list-srv"},
				{Type: gateway.NodeTypeListItem, ID: "item-client", ServerID: "item-srv", ParentID: "list-srv"},
			}
			Expect(translator.FindListClientID(nodes, "list-srv")).To(Equal("list-client-id"))
		})
	})

	When("there's no matching LIST node", func() {
		It("should return an empty string", func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeListItem, ID: "item-client", ParentID: "list-srv"},
			}
			Expect(translator.FindListClientID(nodes, "list-srv")).To(Equal(""))
		})
	})

	When("a different LIST node is present", func() {
		It("should return an empty string", func() {
			nodes := []gateway.Node{
				{Type: gateway.NodeTypeList, ID: "other-id", ServerID: "other-srv"},
			}
			Expect(translator.FindListClientID(nodes, "list-srv")).To(Equal(""))
		})
	})
})

var _ = Describe("IndexLabelsByName", func() {
	When("the input has a mix of live, tombstoned, and incomplete labels", func() {
		var index map[string]string

		BeforeEach(func() {
			deletedAt := time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)
			labels := []gateway.LabelEntry{
				{MainID: "mid-1", Name: "Household"},
				{MainID: "mid-2", Name: "Urgent", Deleted: deletedAt},
				{MainID: "", Name: "BogusEmptyMID"},
				{MainID: "mid-3", Name: ""},
				{MainID: "mid-4", Name: "Garage"},
			}
			index = translator.IndexLabelsByName(labels)
		})

		It("should include live labels", func() {
			Expect(index).To(HaveKeyWithValue("Household", "mid-1"))
			Expect(index).To(HaveKeyWithValue("Garage", "mid-4"))
		})

		It("should skip tombstoned labels", func() {
			Expect(index).ToNot(HaveKey("Urgent"))
		})

		It("should skip labels with empty MainID", func() {
			Expect(index).ToNot(HaveKey("BogusEmptyMID"))
		})

		It("should skip labels with empty Name", func() {
			for k := range index {
				Expect(k).ToNot(BeEmpty())
			}
		})
	})

	When("preserving Keep's canonical capitalization", func() {
		It("should not lowercase the name keys", func() {
			labels := []gateway.LabelEntry{
				{MainID: "mid-1", Name: "Household"},
			}
			index := translator.IndexLabelsByName(labels)
			Expect(index).To(HaveKey("Household"))
			Expect(index).ToNot(HaveKey("household"))
		})
	})
})
