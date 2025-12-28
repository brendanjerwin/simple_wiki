package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockMigrationApplicator tracks migration calls for testing
type MockMigrationApplicator struct {
	ApplyMigrationsFunc func(content []byte) ([]byte, error)
	callCount           int
	lastContentReceived []byte
}

func (m *MockMigrationApplicator) ApplyMigrations(content []byte) ([]byte, error) {
	m.callCount++
	m.lastContentReceived = make([]byte, len(content))
	copy(m.lastContentReceived, content)
	
	if m.ApplyMigrationsFunc != nil {
		return m.ApplyMigrationsFunc(content)
	}
	return content, nil
}

func (m *MockMigrationApplicator) CallCount() int {
	return m.callCount
}

func (m *MockMigrationApplicator) LastContentReceived() []byte {
	return m.lastContentReceived
}

var _ = Describe("Migration Coordination in ReadOrInit", func() {
	var (
		testDataDir             string
		site                    *server.Site
		mockMigrationApplicator *MockMigrationApplicator
	)

	BeforeEach(func() {
		var err error
		testDataDir, err = os.MkdirTemp("", "wiki_migration_coordination_test_*")
		Expect(err).NotTo(HaveOccurred())

		// Use a mock migration applicator to test coordination, not specific migrations
		mockMigrationApplicator = &MockMigrationApplicator{}
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		site = &server.Site{
			PathToData:          testDataDir,
			MigrationApplicator: mockMigrationApplicator,
			Logger:              logger,
		}
	})

	AfterEach(func() {
		if testDataDir != "" {
			os.RemoveAll(testDataDir)
		}
	})

	Context("when opening existing file via ReadOrInit", func() {
		var (
			identifier   string
			fileContent  string
			filePath     string
			openedPage   *wikipage.Page
			err          error
			req          *http.Request
		)

		BeforeEach(func() {
			identifier = "test_page"
			fileContent = `+++
identifier = 'test_page'
title = 'Test Page'
+++
# Test content that needs migration`

			// Create the file exactly as it would exist on disk
			mungedIdentifier, mungeErr := wikiidentifiers.MungeIdentifier(identifier)
			Expect(mungeErr).NotTo(HaveOccurred())
			encodedFilename := base32tools.EncodeToBase32(mungedIdentifier) + ".md"
			filePath = filepath.Join(testDataDir, encodedFilename)

			err = os.WriteFile(filePath, []byte(fileContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create a mock HTTP request
			req = &http.Request{
				URL: &url.URL{
					Path:     "/test_page/edit",
					RawQuery: "",
				},
			}
		})

		Context("when calling ReadOrInit", func() {
			BeforeEach(func() {
				openedPage, err = site.ReadOrInitPageForTesting(identifier, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a page", func() {
				Expect(openedPage).NotTo(BeNil())
			})

			It("should have loaded the page from disk", func() {
				Expect(openedPage.WasLoadedFromDisk).To(BeTrue(), "Page should be loaded from disk")
				Expect(openedPage.IsNew()).To(BeFalse(), "Page should not be new")
			})

			It("should have called the migration applicator", func() {
				Expect(mockMigrationApplicator.CallCount()).To(BeNumerically(">=", 1), "Migration applicator should have been called at least once")
			})

			It("should have passed file content to migration applicator", func() {
				Expect(mockMigrationApplicator.LastContentReceived()).NotTo(BeEmpty(), "Migration applicator should have received content")
				Expect(string(mockMigrationApplicator.LastContentReceived())).To(ContainSubstring("Test content that needs migration"))
			})
		})
	})

	Context("when file doesn't exist (new page)", func() {
		var (
			identifier string
			openedPage *wikipage.Page
			err        error
			req        *http.Request
		)

		BeforeEach(func() {
			identifier = "nonexistent_page"
			
			// Create a mock HTTP request
			req = &http.Request{
				URL: &url.URL{
					Path:     "/nonexistent_page/edit",
					RawQuery: "",
				},
			}

			openedPage, err = site.ReadOrInitPageForTesting(identifier, req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a new page", func() {
			Expect(openedPage).NotTo(BeNil())
			Expect(openedPage.IsNew()).To(BeTrue(), "Page should be new")
			Expect(openedPage.WasLoadedFromDisk).To(BeFalse(), "Page should not be loaded from disk")
		})

		It("should not call migration applicator for new pages", func() {
			Expect(mockMigrationApplicator.CallCount()).To(Equal(0), "Migration applicator should not be called for new pages")
		})
	})

	Context("when migration applicator returns modified content", func() {
		var (
			identifier   string
			fileContent  string
			filePath     string
			openedPage   *wikipage.Page
			err          error
			req          *http.Request
		)

		BeforeEach(func() {
			identifier = "migration_test_page"
			fileContent = `+++
identifier = 'migration_test_page'
+++
# Original content`

			// Set up mock to simulate migration changes
			mockMigrationApplicator.ApplyMigrationsFunc = func(content []byte) ([]byte, error) {
				// Simulate a migration that changes content
				modified := string(content) + "\n# Migration applied"
				return []byte(modified), nil
			}

			// Create the file
			mungedIdentifier, mungeErr := wikiidentifiers.MungeIdentifier(identifier)
			Expect(mungeErr).NotTo(HaveOccurred())
			encodedFilename := base32tools.EncodeToBase32(mungedIdentifier) + ".md"
			filePath = filepath.Join(testDataDir, encodedFilename)

			err = os.WriteFile(filePath, []byte(fileContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create a mock HTTP request
			req = &http.Request{
				URL: &url.URL{
					Path:     "/migration_test_page/edit",
					RawQuery: "",
				},
			}

			openedPage, err = site.ReadOrInitPageForTesting(identifier, req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should have called the migration applicator", func() {
			Expect(mockMigrationApplicator.CallCount()).To(Equal(1))
		})

		It("should use the migrated content", func() {
			content := openedPage.Text
			Expect(content).To(ContainSubstring("# Migration applied"), "Page should contain migrated content")
		})
	})
})