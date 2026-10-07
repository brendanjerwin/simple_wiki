//revive:disable:dot-imports
package history_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/history"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockHistoryReader is a test implementation of history.HistoryReader.
type MockHistoryReader struct {
	versions map[string][]history.PageVersionMetadata
	contents map[string]string
}

// NewMockHistoryReader creates an empty mock history reader.
func NewMockHistoryReader() *MockHistoryReader {
	return &MockHistoryReader{
		versions: make(map[string][]history.PageVersionMetadata),
		contents: make(map[string]string),
	}
}

// AddVersion records a version for a page.
func (m *MockHistoryReader) AddVersion(identifier, versionID, content, author string, createdAt time.Time) {
	m.versions[identifier] = append(m.versions[identifier], history.PageVersionMetadata{
		VersionID:      versionID,
		PageIdentifier: identifier,
		Author:         author,
		CreatedAt:      createdAt,
	})
	m.contents[identifier+"/"+versionID] = content
}

// ListVersions returns a page's versions newest-first.
func (m *MockHistoryReader) ListVersions(identifier wikipage.PageIdentifier) ([]history.PageVersionMetadata, error) {
	versions := m.versions[string(identifier)]
	result := make([]history.PageVersionMetadata, len(versions))
	for i, v := range versions {
		result[len(versions)-1-i] = v
	}
	return result, nil
}

// ReadVersion returns the content of a specific version.
func (m *MockHistoryReader) ReadVersion(identifier wikipage.PageIdentifier, versionID string) (string, error) {
	return m.contents[string(identifier)+"/"+versionID], nil
}

var _ = Describe("Index", func() {
	var (
		idx    *history.Index
		reader *MockHistoryReader
	)

	BeforeEach(func() {
		reader = NewMockHistoryReader()
		var err error
		idx, err = history.NewIndex(reader)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should exist", func() {
		Expect(idx).NotTo(BeNil())
	})

	Describe("NewIndex", func() {
		It("should succeed", func() {
			i, err := history.NewIndex(reader)
			Expect(err).NotTo(HaveOccurred())
			Expect(i).NotTo(BeNil())
		})
	})

	Describe("AddPageToIndex", func() {
		BeforeEach(func() {
			reader.AddVersion("test-page", "v1", "The quick brown fox jumps over the lazy dog", "alice", time.Now())
			reader.AddVersion("test-page", "v2", "The lazy dog slept all day", "bob", time.Now())
			Expect(idx.AddPageToIndex("test-page")).To(Succeed())
		})

		It("should index versions and make them searchable by version_id", func() {
			ids, err := idx.QueryRawForTest("version_id:v1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(HaveLen(1))
		})

		It("should index version content and make it searchable by content term", func() {
			ids, err := idx.QueryRawForTest("content:fox")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(HaveLen(1))
		})
	})

	Describe("RemovePageFromIndex", func() {
		BeforeEach(func() {
			reader.AddVersion("test-page", "v1", "content one", "alice", time.Now())
			reader.AddVersion("test-page", "v2", "content two", "bob", time.Now())
			Expect(idx.AddPageToIndex("test-page")).To(Succeed())
			Expect(idx.RemovePageFromIndex("test-page")).To(Succeed())
		})

		It("should remove all versions for a page", func() {
			ids, err := idx.QueryRawForTest("page_identifier:test-page")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeEmpty())
		})
	})
})
