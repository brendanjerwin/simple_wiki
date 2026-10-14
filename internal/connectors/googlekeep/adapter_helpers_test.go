//revive:disable:dot-imports
package googlekeep_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
)

var _ = Describe("flattenRawIDMap", func() {
	When("raw is a map[string]string", func() {
		It("should return it unchanged", func() {
			input := map[string]string{"uid-1": "ref-1", "uid-2": "ref-2"}
			Expect(googlekeep.FlattenRawIDMapForTest(input)).To(Equal(input))
		})
	})

	When("raw is a map[string]any with string values", func() {
		It("should convert to map[string]string", func() {
			input := map[string]any{
				"uid-1": "ref-1",
				"uid-2": "ref-2",
			}
			result := googlekeep.FlattenRawIDMapForTest(input)
			Expect(result).To(Equal(map[string]string{"uid-1": "ref-1", "uid-2": "ref-2"}))
		})
	})

	When("raw is a map[string]any with a non-string value", func() {
		It("should skip entries whose value is not a string", func() {
			input := map[string]any{
				"uid-1": "ref-1",
				"uid-2": 42,
			}
			result := googlekeep.FlattenRawIDMapForTest(input)
			Expect(result).To(HaveKeyWithValue("uid-1", "ref-1"))
			Expect(result).ToNot(HaveKey("uid-2"))
		})
	})

	When("raw is an unsupported type", func() {
		It("should return nil", func() {
			Expect(googlekeep.FlattenRawIDMapForTest("not-a-map")).To(BeNil())
		})
	})

	When("raw is nil", func() {
		It("should return nil", func() {
			Expect(googlekeep.FlattenRawIDMapForTest(nil)).To(BeNil())
		})
	})
})

var _ = Describe("isLiveListItemForParent", func() {
	const parentID = "parent-srv-1"

	When("the node is not a LIST_ITEM", func() {
		It("should return false", func() {
			n := gateway.Node{Type: gateway.NodeTypeList, ServerID: "srv-1", ParentID: parentID}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeFalse())
		})
	})

	When("the node's ParentID and ParentServerID both differ from parentID", func() {
		It("should return false", func() {
			n := gateway.Node{
				Type:           gateway.NodeTypeListItem,
				ServerID:       "srv-1",
				ParentID:       "other",
				ParentServerID: "other",
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeFalse())
		})
	})

	When("the node is trashed", func() {
		It("should return false", func() {
			n := gateway.Node{
				Type:       gateway.NodeTypeListItem,
				ServerID:   "srv-1",
				ParentID:   parentID,
				Timestamps: gateway.Timestamps{Trashed: time.Now()},
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeFalse())
		})
	})

	When("the node is deleted (Deleted timestamp set)", func() {
		It("should return false", func() {
			n := gateway.Node{
				Type:       gateway.NodeTypeListItem,
				ServerID:   "srv-1",
				ParentID:   parentID,
				Timestamps: gateway.Timestamps{Deleted: time.Now()},
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeFalse())
		})
	})

	When("the node has an empty ServerID", func() {
		It("should return false", func() {
			n := gateway.Node{
				Type:     gateway.NodeTypeListItem,
				ServerID: "",
				ParentID: parentID,
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeFalse())
		})
	})

	When("the node is a live LIST_ITEM under the correct parent via ParentServerID", func() {
		It("should return true", func() {
			n := gateway.Node{
				Type:           gateway.NodeTypeListItem,
				ServerID:       "srv-1",
				ParentServerID: parentID,
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeTrue())
		})
	})

	When("the node is a live LIST_ITEM under the correct parent via ParentID", func() {
		It("should return true", func() {
			n := gateway.Node{
				Type:     gateway.NodeTypeListItem,
				ServerID: "srv-1",
				ParentID: parentID,
			}
			Expect(googlekeep.IsLiveListItemForParentForTest(n, parentID)).To(BeTrue())
		})
	})
})
