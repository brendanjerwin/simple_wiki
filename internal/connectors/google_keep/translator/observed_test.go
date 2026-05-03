//revive:disable:dot-imports
package translator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/translator"
)

var _ = Describe("LastObservedWikiFingerprints", func() {
	When("the checklist is nil", func() {
		It("should return an empty map", func() {
			result := translator.LastObservedWikiFingerprints(map[string]struct{}{"uid-A": {}}, nil)
			Expect(result).To(BeEmpty())
		})
	})

	When("a paired uid has a corresponding wiki item", func() {
		var result map[string]translator.Fingerprint

		BeforeEach(func() {
			checklist := &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: "uid-A", Text: "Apples", SortOrder: 1000},
				},
			}
			result = translator.LastObservedWikiFingerprints(map[string]struct{}{"uid-A": {}}, checklist)
		})

		It("should return a fingerprint for that uid", func() {
			Expect(result).To(HaveKey("uid-A"))
		})

		It("should populate the fingerprint with the wiki item's encoded text", func() {
			Expect(result["uid-A"].Text).To(Equal("Apples"))
		})

		It("should populate the SortValue from the wiki item's sort_order", func() {
			Expect(result["uid-A"].SortValue).To(Equal("1000"))
		})
	})

	When("a paired uid has no corresponding wiki item", func() {
		var result map[string]translator.Fingerprint

		BeforeEach(func() {
			checklist := &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: "uid-A", Text: "Apples"},
				},
			}
			result = translator.LastObservedWikiFingerprints(
				map[string]struct{}{"uid-A": {}, "uid-orphaned": {}}, checklist)
		})

		It("should include only the uid that has a wiki item", func() {
			Expect(result).To(HaveKey("uid-A"))
			Expect(result).ToNot(HaveKey("uid-orphaned"))
		})
	})

	When("a wiki item is unpaired (uid not in pairedUIDs)", func() {
		var result map[string]translator.Fingerprint

		BeforeEach(func() {
			checklist := &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: "uid-A", Text: "Apples"},
					{Uid: "uid-unpaired", Text: "Bananas"},
				},
			}
			result = translator.LastObservedWikiFingerprints(map[string]struct{}{"uid-A": {}}, checklist)
		})

		It("should not produce a fingerprint for the unpaired item", func() {
			Expect(result).ToNot(HaveKey("uid-unpaired"))
		})
	})

	When("a wiki item has an empty UID", func() {
		var result map[string]translator.Fingerprint

		BeforeEach(func() {
			checklist := &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: "", Text: "Apples"},
					{Uid: "uid-A", Text: "Bananas"},
				},
			}
			result = translator.LastObservedWikiFingerprints(map[string]struct{}{"uid-A": {}}, checklist)
		})

		It("should ignore the empty-uid item", func() {
			Expect(result).To(HaveKey("uid-A"))
			Expect(result).ToNot(HaveKey(""))
		})
	})

	When("the paired UID set is empty", func() {
		It("should return an empty map even with items in the checklist", func() {
			checklist := &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: "uid-A", Text: "Apples"}},
			}
			result := translator.LastObservedWikiFingerprints(map[string]struct{}{}, checklist)
			Expect(result).To(BeEmpty())
		})
	})
})
