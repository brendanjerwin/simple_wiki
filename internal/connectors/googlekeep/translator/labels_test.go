//revive:disable:dot-imports
package translator_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/translator"
)

var _ = Describe("MergeKeepLabels", func() {
	var now time.Time

	BeforeEach(func() {
		now = time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)
	})

	When("the wiki tag list is empty", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(nil, map[string]string{}, nil, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return no label CRUD entries", func() {
			Expect(labelPush).To(BeEmpty())
		})

		It("should return no label IDs to assign", func() {
			Expect(listLabelIDs).To(BeEmpty())
		})
	})

	When("a tag has a persisted MainID under canonical Keep capitalization", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			persisted := map[string]string{"Household": "stable-mid-1"}
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"household"}, persisted, nil, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not mint a fresh label CRUD entry for the lowercase tag", func() {
			Expect(labelPush).To(BeEmpty())
		})

		It("should resolve the lowercase tag to the persisted MainID", func() {
			Expect(listLabelIDs).To(ConsistOf("stable-mid-1"))
		})
	})

	When("a tag has no persisted MainID and no pull-supplied label", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"fresh"}, map[string]string{}, nil, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should mint exactly one label CRUD entry", func() {
			Expect(labelPush).To(HaveLen(1))
		})

		It("should set the new label's name to the input tag", func() {
			Expect(labelPush[0].Name).To(Equal("fresh"))
		})

		It("should reference the freshly-minted MainID on the LIST", func() {
			Expect(listLabelIDs).To(HaveLen(1))
			Expect(listLabelIDs[0]).To(Equal(labelPush[0].MainID))
		})

		It("should stamp Created and Updated to now", func() {
			Expect(labelPush[0].Created).To(Equal(now))
			Expect(labelPush[0].Updated).To(Equal(now))
		})
	})

	When("an existing pull-supplied label matches the tag name", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			pullLabels := []gateway.LabelEntry{
				{MainID: "pull-mid-2", Name: "Urgent"},
			}
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"urgent"}, map[string]string{}, pullLabels, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not mint a fresh CRUD entry — the pull supplied one", func() {
			Expect(labelPush).To(BeEmpty())
		})

		It("should reference the pull-supplied MainID on the LIST", func() {
			Expect(listLabelIDs).To(ConsistOf("pull-mid-2"))
		})
	})

	When("a pull-supplied label is tombstoned", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			deletedAt := now.Add(-1 * time.Hour)
			pullLabels := []gateway.LabelEntry{
				{MainID: "old-mid", Name: "ZombieLabel", Deleted: deletedAt},
			}
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"zombielabel"}, map[string]string{}, pullLabels, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should mint a fresh entry rather than reviving the tombstone", func() {
			Expect(labelPush).To(HaveLen(1))
			Expect(labelPush[0].MainID).ToNot(Equal("old-mid"))
		})

		It("should reference the freshly-minted MainID on the LIST", func() {
			Expect(listLabelIDs).To(HaveLen(1))
			Expect(listLabelIDs[0]).To(Equal(labelPush[0].MainID))
		})
	})

	When("the input list contains duplicate tags", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"fresh", "fresh"}, map[string]string{}, nil, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should mint only one CRUD entry — the second tag reuses the first's MainID", func() {
			Expect(labelPush).To(HaveLen(1))
		})

		It("should reference the same MainID twice in listLabelIDs", func() {
			Expect(listLabelIDs).To(HaveLen(2))
			Expect(listLabelIDs[0]).To(Equal(listLabelIDs[1]))
		})
	})

	When("persistedLabelIDs has empty MainID values for some names", func() {
		var (
			labelPush    []gateway.LabelEntry
			listLabelIDs []string
			err          error
		)

		BeforeEach(func() {
			persisted := map[string]string{
				"household": "",          // empty MainID — should not be used
				"urgent":    "good-mid",  // good
			}
			labelPush, listLabelIDs, err = translator.MergeKeepLabels(
				[]string{"household", "urgent"}, persisted, nil, now)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should mint a fresh entry for the empty-MainID tag", func() {
			Expect(labelPush).To(HaveLen(1))
			Expect(labelPush[0].Name).To(Equal("household"))
		})

		It("should resolve the populated tag from persisted", func() {
			Expect(listLabelIDs).To(ContainElement("good-mid"))
		})
	})
})

var _ = Describe("GenerateLabelMainID", func() {
	When("called once", func() {
		var (
			id  string
			err error
		)

		BeforeEach(func() {
			id, err = translator.GenerateLabelMainID(time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC), 0)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should produce a non-empty id", func() {
			Expect(id).ToNot(BeEmpty())
		})

		It("should match the gkeepapi node-id shape ms-hex.16-hex", func() {
			parts := strings.Split(id, ".")
			Expect(parts).To(HaveLen(2))
			Expect(len(parts[1])).To(Equal(16))
		})
	})

	When("called twice in the same instant with different idx values", func() {
		var (
			now time.Time
			a   string
			b   string
		)

		BeforeEach(func() {
			now = time.Date(2026, 4, 25, 17, 0, 0, 0, time.UTC)
			a, _ = translator.GenerateLabelMainID(now, 0)
			b, _ = translator.GenerateLabelMainID(now, 1)
		})

		It("should produce distinct ids", func() {
			Expect(a).ToNot(Equal(b))
		})
	})
})
