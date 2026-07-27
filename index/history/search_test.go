//revive:disable:dot-imports
package history_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/history"
)

var _ = Describe("Search", func() {
	var (
		idx    *history.Index
		reader *MockHistoryReader
	)

	BeforeEach(func() {
		reader = NewMockHistoryReader()
		var err error
		idx, err = history.NewIndex(reader)
		Expect(err).NotTo(HaveOccurred())

		now := time.Now()
		reader.AddVersion("page-a", "a1", "The quick brown fox jumps over the lazy dog", "alice", now.Add(-2*time.Hour))
		reader.AddVersion("page-a", "a2", "The lazy dog slept all day", "bob", now.Add(-1*time.Hour))
		reader.AddVersion("page-b", "b1", "A fox in the garden", "bob", now.Add(-30*time.Minute))

		Expect(idx.AddPageToIndex("page-a")).To(Succeed())
		Expect(idx.AddPageToIndex("page-b")).To(Succeed())
	})

	Describe("SearchPageHistory", func() {
		It("returns matching versions for a page", func() {
			results, err := idx.SearchPageHistory("page-a", "fox")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Page).To(Equal("page-a"))
			Expect(results[0].Version.VersionID).To(Equal("a1"))
			Expect(results[0].Snippet).NotTo(BeEmpty())
		})
	})

	Describe("SearchHistory", func() {
		It("returns matching versions across all pages with query text", func() {
			results, err := idx.SearchHistory(history.HistorySearchFilter{Query: "fox"})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))

			ids := []string{results[0].Version.VersionID, results[1].Version.VersionID}
			Expect(ids).To(ContainElements("a1", "b1"))
		})

		It("filters by page_filter", func() {
			results, err := idx.SearchHistory(history.HistorySearchFilter{
				Query:          "fox",
				PageFilter: "page-a",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Page).To(Equal("page-a"))
		})

		It("filters by author_filter", func() {
			results, err := idx.SearchHistory(history.HistorySearchFilter{
				Query:        "fox",
				AuthorFilter: "alice",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Version.Author).To(Equal("alice"))
		})

		It("filters by time range", func() {
			from := time.Now().Add(-45 * time.Minute)
			to := time.Now()
			results, err := idx.SearchHistory(history.HistorySearchFilter{
				Query: "fox",
				From:  from,
				To:    to,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Version.VersionID).To(Equal("b1"))
		})

		It("returns HistorySearchResult fields", func() {
			results, err := idx.SearchHistory(history.HistorySearchFilter{
				Query:          "fox",
				PageFilter: "page-a",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))

			result := results[0]
			Expect(result.Page).To(Equal("page-a"))
			Expect(result.Version.VersionID).To(Equal("a1"))
			Expect(result.Version.Author).To(Equal("alice"))
			Expect(result.Snippet).NotTo(BeEmpty())
		})
	})
})
