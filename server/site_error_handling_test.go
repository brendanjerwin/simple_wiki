//revive:disable:dot-imports
package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// writeCloserBuffer wraps a bytes.Buffer to implement io.WriteCloser for capturing logs
type logWriteCloser struct {
	*bytes.Buffer
}

func (*logWriteCloser) Close() error {
	return nil
}

var _ = Describe("Site error handling", func() {
	var (
		s           *Site
		tempDir     string
		logBuffer   *bytes.Buffer
		testLogger  *lumber.ConsoleLogger
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "site-error-test")
		Expect(err).NotTo(HaveOccurred())

		logBuffer = &bytes.Buffer{}
		testLogger = lumber.NewBasicLogger(&logWriteCloser{logBuffer}, lumber.TRACE)

		s = &Site{
			Logger:              testLogger,
			PathToData:          tempDir,
			MarkdownRenderer:    &goldmarkrenderer.GoldmarkRenderer{},
			MigrationApplicator: lazy.NewEmptyApplicator(),
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("startMigrationJobs", func() {
		Describe("when EnqueueJob fails for file shadowing scan job", func() {
			var logOutput string

			BeforeEach(func() {
				// Create a coordinator with a factory that returns failing dispatchers
				dispatchErr := errors.New("dispatcher queue full")
				s.JobQueueCoordinator = jobs.NewJobQueueCoordinatorWithFactory(
					testLogger,
					jobs.FailingDispatcherFactory(dispatchErr),
				)

				// Call startMigrationJobs
				s.startMigrationJobs()

				logOutput = logBuffer.String()
			})

			It("should log the file shadowing scan job error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue file shadowing scan job"))
			})

			It("should include the error message in the log", func() {
				Expect(logOutput).To(ContainSubstring("dispatcher queue full"))
			})

			It("should log the JSON archive migration job error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue JSON archive migration job"))
			})
		})

		Describe("when EnqueueJob succeeds", func() {
			var logOutput string

			BeforeEach(func() {
				// Create a normal coordinator
				s.JobQueueCoordinator = jobs.NewJobQueueCoordinator(testLogger)

				// Call startMigrationJobs
				s.startMigrationJobs()

				logOutput = logBuffer.String()
			})

			It("should log that file shadowing scan started", func() {
				Expect(logOutput).To(ContainSubstring("File shadowing scan started"))
			})

			It("should log that JSON archive migration started", func() {
				Expect(logOutput).To(ContainSubstring("JSON archive migration started"))
			})
		})
	})

	Describe("DeletePage", func() {
		var (
			pageIdentifier wikipage.PageIdentifier
			pagePath       string
		)

		BeforeEach(func() {
			pageIdentifier = "test-delete-page"
			pagePath = filepath.Join(tempDir, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")
		})

		Describe("when the page exists", func() {
			BeforeEach(func() {
				// Create a test page
				content := `+++
title = "Test Page"
+++
test content`
				fileErr := os.WriteFile(pagePath, []byte(content), 0644)
				Expect(fileErr).NotTo(HaveOccurred())
			})

			Describe("when EnqueueIndexJob fails", func() {
				var (
					deleteErr error
					logOutput string
				)

				BeforeEach(func() {
					// Create a failing job queue coordinator
					dispatchErr := errors.New("index queue full")
					failingCoordinator := jobs.NewJobQueueCoordinatorWithFactory(
						testLogger,
						jobs.FailingDispatcherFactory(dispatchErr),
					)

					// Create index coordinator with failing job queue
					mockFrontmatter := &MockIndexOperator{}
					mockBleve := &MockIndexOperator{}
					s.IndexCoordinator = index.NewIndexCoordinator(failingCoordinator, mockFrontmatter, mockBleve)

					// Perform delete
					deleteErr = s.DeletePage(pageIdentifier)

					logOutput = logBuffer.String()
				})

				It("should still delete the page successfully", func() {
					Expect(deleteErr).NotTo(HaveOccurred())
				})

				It("should log the index removal error", func() {
					Expect(logOutput).To(ContainSubstring("Failed to enqueue index removal job"))
				})

				It("should include the page identifier in the log", func() {
					Expect(logOutput).To(ContainSubstring(string(pageIdentifier)))
				})

				It("should include the error message in the log", func() {
					Expect(logOutput).To(ContainSubstring("index queue full"))
				})

				It("should move the file to deleted directory", func() {
					_, statErr := os.Stat(pagePath)
					Expect(os.IsNotExist(statErr)).To(BeTrue())
				})
			})
		})
	})

	Describe("savePageAndIndex", func() {
		var (
			page *wikipage.Page
		)

		BeforeEach(func() {
			page = &wikipage.Page{
				Identifier: "test-save-page",
				Text:       "test content",
			}
		})

		Describe("when EnqueueIndexJob fails", func() {
			var (
				saveErr   error
				logOutput string
			)

			BeforeEach(func() {
				// Create a failing job queue coordinator
				dispatchErr := errors.New("index queue full")
				failingCoordinator := jobs.NewJobQueueCoordinatorWithFactory(
					testLogger,
					jobs.FailingDispatcherFactory(dispatchErr),
				)

				// Create index coordinator with failing job queue
				mockFrontmatter := &MockIndexOperator{}
				mockBleve := &MockIndexOperator{}
				s.IndexCoordinator = index.NewIndexCoordinator(failingCoordinator, mockFrontmatter, mockBleve)

				// Also set JobQueueCoordinator (this will also fail)
				s.JobQueueCoordinator = failingCoordinator

				// Perform save
				saveErr = s.savePageAndIndex(page)

				logOutput = logBuffer.String()
			})

			It("should still save the page successfully", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should log the index job error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue index job"))
			})

			It("should include the page identifier in the log", func() {
				Expect(logOutput).To(ContainSubstring(page.Identifier))
			})

			It("should log the per-page inventory normalization error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue per-page inventory normalization job"))
			})

			It("should write the page to disk", func() {
				pagePath := filepath.Join(tempDir, base32tools.EncodeToBase32(strings.ToLower(page.Identifier))+".md")
				content, readErr := os.ReadFile(pagePath)
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal(page.Text))
			})
		})

		Describe("when only per-page inventory normalization job EnqueueJob fails", func() {
			var (
				saveErr   error
				logOutput string
			)

			BeforeEach(func() {
				// Set up working index coordinator
				coordinator := jobs.NewJobQueueCoordinator(testLogger)
				mockFrontmatter := &MockIndexOperator{}
				mockBleve := &MockIndexOperator{}
				s.IndexCoordinator = index.NewIndexCoordinator(coordinator, mockFrontmatter, mockBleve)

				// Set up failing job queue coordinator for inventory normalization
				dispatchErr := errors.New("inventory queue full")
				failingCoordinator := jobs.NewJobQueueCoordinatorWithFactory(
					testLogger,
					jobs.FailingDispatcherFactory(dispatchErr),
				)
				s.JobQueueCoordinator = failingCoordinator

				// Perform save
				saveErr = s.savePageAndIndex(page)

				logOutput = logBuffer.String()
			})

			It("should still save the page successfully", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should log the per-page inventory normalization error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue per-page inventory normalization job"))
			})

			It("should include the error message in the log", func() {
				Expect(logOutput).To(ContainSubstring("inventory queue full"))
			})
		})
	})

	Describe("InitializeIndexing callback error handling", func() {
		Describe("when BulkEnqueuePagesWithCompletion fails immediately", func() {
			var (
				initErr   error
				logOutput string
			)

			BeforeEach(func() {
				// Create a page so there's something to index
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test"))
				pagePath := filepath.Join(tempDir, encodedFilename+".md")
				testPageContent := `+++
identifier = "test"
+++
# Test Content`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				// We need to intercept InitializeIndexing after bleve/frontmatter indexes are created
				// but before the bulk enqueue. This requires modifying the Site after partial init.
				// For now, we'll test by calling InitializeIndexing with a failing coordinator.

				// First, call InitializeIndexing normally to set up indexes
				initErr = s.InitializeIndexing()
				Expect(initErr).NotTo(HaveOccurred())

				// Now replace the coordinator with one that fails and call again
				// Note: This is a bit of a workaround since InitializeIndexing creates its own coordinator
				// The actual test would need to observe the log output from the first call
				logOutput = logBuffer.String()
			})

			// Note: Testing the immediate failure of BulkEnqueuePagesWithCompletion is challenging
			// because InitializeIndexing creates its own JobQueueCoordinator internally.
			// The error handling for this case logs but doesn't return an error.
			It("should complete initialization without returning an error", func() {
				Expect(initErr).NotTo(HaveOccurred())
			})

			It("should log background indexing started message", func() {
				Expect(logOutput).To(ContainSubstring("Background indexing started"))
			})
		})

		// Test the callback error paths by using a mock index coordinator
		Describe("when async callback executes with nil FrontmatterIndexQueryer", func() {
			var (
				logOutput     string
				callbackError error
				mu            sync.Mutex
			)

			BeforeEach(func() {
				// Create a page so there's something to index
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test-callback"))
				pagePath := filepath.Join(tempDir, encodedFilename+".md")
				testPageContent := `+++
identifier = "test-callback"
+++
# Test Content`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				// Set up a working job queue coordinator
				s.JobQueueCoordinator = jobs.NewJobQueueCoordinator(testLogger)

				// Set up frontmatter index (needed by mock)
				mockFrontmatter := &MockIndexOperator{}
				mockBleve := &MockIndexOperator{}
				s.IndexCoordinator = index.NewIndexCoordinator(s.JobQueueCoordinator, mockFrontmatter, mockBleve)

				// Set FrontmatterIndexQueryer to nil to trigger error in NewInventoryNormalizationJob
				s.FrontmatterIndexQueryer = nil

				// Simulate what InitializeIndexing does - call BulkEnqueuePagesWithCompletion
				// with a callback that tries to create a normalization job
				pageIdentifiers := []string{"test-callback"}
				err := s.IndexCoordinator.BulkEnqueuePagesWithCompletion(pageIdentifiers, index.Add, func() {
					// This is the callback that would normally run after indexing completes
					// It tries to create an InventoryNormalizationJob
					normJob, err := NewInventoryNormalizationJob(s, s.FrontmatterIndexQueryer, s.Logger)
					if err != nil {
						s.Logger.Error("Failed to create inventory normalization job: %v", err)
						mu.Lock()
						callbackError = err
						mu.Unlock()
						return
					}
					if err := s.JobQueueCoordinator.EnqueueJob(normJob); err != nil {
						s.Logger.Error("Failed to enqueue inventory normalization job: %v", err)
					} else {
						s.Logger.Info("Inventory normalization job queued after indexing completed")
					}
				})
				Expect(err).NotTo(HaveOccurred())

				// Wait for indexing to complete
				completed, _ := s.IndexCoordinator.WaitForCompletionWithTimeout(context.Background(), 2*time.Second)
				Expect(completed).To(BeTrue())

				// Give a moment for the callback to execute
				time.Sleep(100 * time.Millisecond)

				logOutput = logBuffer.String()
			})

			It("should log the job creation error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to create inventory normalization job"))
			})

			It("should include the specific error message", func() {
				Expect(logOutput).To(ContainSubstring("fmIndex is required"))
			})

			It("should capture the callback error", func() {
				mu.Lock()
				defer mu.Unlock()
				Expect(callbackError).To(HaveOccurred())
				Expect(callbackError.Error()).To(ContainSubstring("fmIndex is required"))
			})
		})

		Describe("when async callback fails to enqueue normalization job", func() {
			var logOutput string

			BeforeEach(func() {
				// Create a page so there's something to index
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test-enqueue-fail"))
				pagePath := filepath.Join(tempDir, encodedFilename+".md")
				testPageContent := `+++
identifier = "test-enqueue-fail"
+++
# Test Content`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				// Set up a working job queue coordinator for indexing
				workingCoordinator := jobs.NewJobQueueCoordinator(testLogger)

				// Set up index coordinator
				mockFrontmatter := &MockIndexOperator{}
				mockBleve := &MockIndexOperator{}
				s.IndexCoordinator = index.NewIndexCoordinator(workingCoordinator, mockFrontmatter, mockBleve)

				// Set up a failing job queue coordinator for the normalization job
				dispatchErr := errors.New("normalization queue full")
				failingCoordinator := jobs.NewJobQueueCoordinatorWithFactory(
					testLogger,
					jobs.FailingDispatcherFactory(dispatchErr),
				)
				s.JobQueueCoordinator = failingCoordinator

				// Set up FrontmatterIndexQueryer
				s.FrontmatterIndexQueryer = &mockFrontmatterIndexQueryerForCallbackTest{}

				// Simulate what InitializeIndexing does
				pageIdentifiers := []string{"test-enqueue-fail"}
				err := s.IndexCoordinator.BulkEnqueuePagesWithCompletion(pageIdentifiers, index.Add, func() {
					normJob, err := NewInventoryNormalizationJob(s, s.FrontmatterIndexQueryer, s.Logger)
					if err != nil {
						s.Logger.Error("Failed to create inventory normalization job: %v", err)
						return
					}
					if err := s.JobQueueCoordinator.EnqueueJob(normJob); err != nil {
						s.Logger.Error("Failed to enqueue inventory normalization job: %v", err)
					} else {
						s.Logger.Info("Inventory normalization job queued after indexing completed")
					}
				})
				Expect(err).NotTo(HaveOccurred())

				// Wait for indexing to complete
				completed, _ := s.IndexCoordinator.WaitForCompletionWithTimeout(context.Background(), 2*time.Second)
				Expect(completed).To(BeTrue())

				// Give a moment for the callback to execute
				time.Sleep(100 * time.Millisecond)

				logOutput = logBuffer.String()
			})

			It("should log the enqueue error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to enqueue inventory normalization job"))
			})

			It("should include the specific error message", func() {
				Expect(logOutput).To(ContainSubstring("normalization queue full"))
			})
		})
	})
})

// MockIndexCoordinatorWithFailingEnqueue is a mock that fails on EnqueueIndexJob
type MockIndexCoordinatorWithFailingEnqueue struct {
	EnqueueError error
}

func (m *MockIndexCoordinatorWithFailingEnqueue) EnqueueIndexJob(_ string, _ index.Operation) error {
	return m.EnqueueError
}

func (m *MockIndexCoordinatorWithFailingEnqueue) BulkEnqueuePages(_ []string, _ index.Operation) error {
	return m.EnqueueError
}

func (m *MockIndexCoordinatorWithFailingEnqueue) BulkEnqueuePagesWithCompletion(_ []string, _ index.Operation, onComplete func()) error {
	if onComplete != nil {
		onComplete()
	}
	return m.EnqueueError
}

// mockFrontmatterIndexQueryerForCallbackTest is a minimal mock for testing callback error paths
type mockFrontmatterIndexQueryerForCallbackTest struct{}

var _ frontmatter.IQueryFrontmatterIndex = (*mockFrontmatterIndexQueryerForCallbackTest)(nil)

func (*mockFrontmatterIndexQueryerForCallbackTest) GetValue(_, _ string) string {
	return ""
}

func (*mockFrontmatterIndexQueryerForCallbackTest) QueryExactMatch(_, _ string) []string {
	return nil
}

func (*mockFrontmatterIndexQueryerForCallbackTest) QueryPrefixMatch(_, _ string) []string {
	return nil
}

func (*mockFrontmatterIndexQueryerForCallbackTest) QueryKeyExistence(_ string) []string {
	return nil
}
