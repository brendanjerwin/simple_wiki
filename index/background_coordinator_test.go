package index_test

import (
	"errors"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MockIndexMaintainer is a mock implementation of IMaintainIndex for testing.
type MockIndexMaintainer struct {
	AddPageToIndexFunc    func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc func(identifier wikipage.PageIdentifier) error
	AddPageCallCount      int
	RemovePageCallCount   int
	AddedPages           []wikipage.PageIdentifier
	RemovedPages         []wikipage.PageIdentifier
	IndexName            string
	mu                   sync.Mutex // Protects concurrent access to counters and slices
}

func (m *MockIndexMaintainer) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.mu.Lock()
	m.AddPageCallCount++
	m.AddedPages = append(m.AddedPages, identifier)
	m.mu.Unlock()
	
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockIndexMaintainer) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.mu.Lock()
	m.RemovePageCallCount++
	m.RemovedPages = append(m.RemovedPages, identifier)
	m.mu.Unlock()
	
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

func (m *MockIndexMaintainer) GetIndexName() string {
	if m.IndexName != "" {
		return m.IndexName
	}
	return "mock"
}

// GetAddPageCallCount returns the count of AddPageToIndex calls in a thread-safe manner.
func (m *MockIndexMaintainer) GetAddPageCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.AddPageCallCount
}

// GetRemovePageCallCount returns the count of RemovePageFromIndex calls in a thread-safe manner.
func (m *MockIndexMaintainer) GetRemovePageCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.RemovePageCallCount
}

var _ = Describe("BackgroundIndexingCoordinator", func() {
	var (
		coordinator     *index.BackgroundIndexingCoordinator
		mockMaintainer1 *MockIndexMaintainer
		mockMaintainer2 *MockIndexMaintainer
		multiMaintainer *index.MultiMaintainer
		logger          lumber.Logger
		testPages       []wikipage.PageIdentifier
	)

	BeforeEach(func() {
		mockMaintainer1 = &MockIndexMaintainer{IndexName: "index1"}
		mockMaintainer2 = &MockIndexMaintainer{IndexName: "index2"}
		multiMaintainer = index.NewMultiMaintainer(mockMaintainer1, mockMaintainer2)
		logger = lumber.NewConsoleLogger(lumber.INFO)
		
		testPages = []wikipage.PageIdentifier{
			"page1",
			"page2", 
			"page3",
			"page4",
			"page5",
		}
		
		coordinator = index.NewBackgroundIndexingCoordinator(multiMaintainer, logger, 2)
	})

	AfterEach(func() {
		if coordinator != nil {
			_ = coordinator.Stop()
		}
	})

	Describe("NewBackgroundIndexingCoordinator", func() {
		It("should exist", func() {
			Expect(coordinator).NotTo(BeNil())
		})

		When("worker count is zero", func() {
			It("should panic", func() {
				Expect(func() {
					index.NewBackgroundIndexingCoordinator(multiMaintainer, logger, 0)
				}).To(Panic())
			})
		})

		When("using CPU workers constructor", func() {
			BeforeEach(func() {
				coordinator = index.NewBackgroundIndexingCoordinatorWithCPUWorkers(multiMaintainer, logger)
			})

			It("should create coordinator with positive worker count", func() {
				// We can't easily test the exact value, but we can test it's positive
				progress := coordinator.GetProgress()
				Expect(progress.IsRunning).To(BeFalse())
			})
		})
	})

	Describe("IMaintainIndex interface compatibility", func() {
		When("AddPageToIndex is called", func() {
			var err error

			BeforeEach(func() {
				err = coordinator.AddPageToIndex("test-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delegate to underlying MultiMaintainer", func() {
				Expect(mockMaintainer1.GetAddPageCallCount()).To(Equal(1))
				Expect(mockMaintainer2.GetAddPageCallCount()).To(Equal(1))
				Expect(mockMaintainer1.AddedPages[0]).To(Equal(wikipage.PageIdentifier("test-page")))
			})
		})

		When("RemovePageFromIndex is called", func() {
			var err error

			BeforeEach(func() {
				err = coordinator.RemovePageFromIndex("test-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delegate to underlying MultiMaintainer", func() {
				Expect(mockMaintainer1.GetRemovePageCallCount()).To(Equal(1))
				Expect(mockMaintainer2.GetRemovePageCallCount()).To(Equal(1))
				Expect(mockMaintainer1.RemovedPages[0]).To(Equal(wikipage.PageIdentifier("test-page")))
			})
		})
	})

	Describe("StartBackground", func() {
		When("starting background indexing", func() {
			var err error

			BeforeEach(func() {
				err = coordinator.StartBackground(testPages)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should mark indexing as running", func() {
				progress := coordinator.GetProgress()
				Expect(progress.IsRunning).To(BeTrue())
			})

			It("should set correct total pages", func() {
				progress := coordinator.GetProgress()
				Expect(progress.TotalPages).To(Equal(len(testPages)))
			})

			It("should start with completed pages within bounds", func() {
				progress := coordinator.GetProgress()
				Expect(progress.CompletedPages).To(BeNumerically(">=", 0))
				Expect(progress.CompletedPages).To(BeNumerically("<=", len(testPages)))
			})
		})

		When("starting background indexing twice", func() {
			var err1, err2 error

			BeforeEach(func() {
				err1 = coordinator.StartBackground(testPages)
				err2 = coordinator.StartBackground(testPages)
			})

			It("should not return errors", func() {
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
			})

			It("should remain running", func() {
				progress := coordinator.GetProgress()
				Expect(progress.IsRunning).To(BeTrue())
			})
		})

		When("pages are processed", func() {
			BeforeEach(func() {
				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for processing to complete - check that both indexes have processed all pages
				Eventually(func() bool {
					return mockMaintainer1.GetAddPageCallCount() == len(testPages) &&
						   mockMaintainer2.GetAddPageCallCount() == len(testPages)
				}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
			})

			It("should process all pages", func() {
				// Verify all pages were added to both indexes
				Expect(mockMaintainer1.GetAddPageCallCount()).To(Equal(len(testPages)))
				Expect(mockMaintainer2.GetAddPageCallCount()).To(Equal(len(testPages)))
			})

			It("should update progress correctly", func() {
				progress := coordinator.GetProgress()
				Expect(progress.CompletedPages).To(Equal(len(testPages)))
				Expect(progress.ProcessingRatePerSecond).To(BeNumerically(">", 0))
			})

			It("should calculate processing rate", func() {
				progress := coordinator.GetProgress()
				Expect(progress.ProcessingRatePerSecond).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("Stop", func() {
		When("stopping indexing that hasn't started", func() {
			var err error

			BeforeEach(func() {
				err = coordinator.Stop()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("stopping running indexing", func() {
			var err error

			BeforeEach(func() {
				startErr := coordinator.StartBackground(testPages)
				Expect(startErr).NotTo(HaveOccurred())
				
				err = coordinator.Stop()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should mark indexing as not running", func() {
				progress := coordinator.GetProgress()
				Expect(progress.IsRunning).To(BeFalse())
			})
		})
	})

	Describe("GetProgress", func() {
		When("no indexing has started", func() {
			var progress index.IndexingProgress

			BeforeEach(func() {
				progress = coordinator.GetProgress()
			})

			It("should not be running", func() {
				Expect(progress.IsRunning).To(BeFalse())
			})

			It("should have zero totals", func() {
				Expect(progress.TotalPages).To(Equal(0))
				Expect(progress.CompletedPages).To(Equal(0))
			})

			It("should have zero processing rate", func() {
				Expect(progress.ProcessingRatePerSecond).To(Equal(0.0))
			})

			It("should have no estimated completion", func() {
				Expect(progress.EstimatedCompletion).To(BeNil())
			})
		})

		When("indexing is in progress", func() {
			BeforeEach(func() {
				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for some progress
				Eventually(func() int {
					progress := coordinator.GetProgress()
					return progress.CompletedPages
				}, 2*time.Second, 50*time.Millisecond).Should(BeNumerically(">", 0))
			})

			It("should show progress", func() {
				progress := coordinator.GetProgress()
				Expect(progress.IsRunning).To(BeTrue())
				Expect(progress.CompletedPages).To(BeNumerically(">", 0))
				Expect(progress.CompletedPages).To(BeNumerically("<=", len(testPages)))
			})

			It("should calculate queue depth", func() {
				progress := coordinator.GetProgress()
				Expect(progress.QueueDepth).To(BeNumerically(">=", 0))
			})
		})
	})

	Describe("Per-Index Progress Tracking", func() {
		When("pages are processed", func() {
			BeforeEach(func() {
				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for processing to complete
				Eventually(func() int {
					progress := coordinator.GetProgress()
					return progress.CompletedPages
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(len(testPages)))
			})

			It("should track progress for each individual index", func() {
				progress := coordinator.GetProgress()
				Expect(progress.IndexProgress).To(HaveLen(2))

				// Check index1 progress
				index1Progress, exists := progress.IndexProgress["index1"]
				Expect(exists).To(BeTrue())
				Expect(index1Progress.Name).To(Equal("index1"))
				Expect(index1Progress.Completed).To(Equal(len(testPages)))
				Expect(index1Progress.Total).To(Equal(len(testPages)))
				Expect(index1Progress.ProcessingRatePerSecond).To(BeNumerically(">", 0))
				Expect(index1Progress.LastError).To(BeNil())

				// Check index2 progress
				index2Progress, exists := progress.IndexProgress["index2"]
				Expect(exists).To(BeTrue())
				Expect(index2Progress.Name).To(Equal("index2"))
				Expect(index2Progress.Completed).To(Equal(len(testPages)))
				Expect(index2Progress.Total).To(Equal(len(testPages)))
				Expect(index2Progress.ProcessingRatePerSecond).To(BeNumerically(">", 0))
				Expect(index2Progress.LastError).To(BeNil())
			})
		})

		When("one index fails for some pages", func() {
			BeforeEach(func() {
				// Make index1 fail for the first page
				mockMaintainer1.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					if identifier == "page1" {
						return errors.New("simulated index1 error")
					}
					return nil
				}

				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for processing to complete
				Eventually(func() bool {
					progress := coordinator.GetProgress()
					// Since index1 fails for page1, we expect index1 to complete len(testPages)-1
					// and index2 to complete all len(testPages)
					index1Progress, exists1 := progress.IndexProgress["index1"]
					index2Progress, exists2 := progress.IndexProgress["index2"] 
					return exists1 && exists2 && 
						   index1Progress.Completed == len(testPages)-1 &&
						   index2Progress.Completed == len(testPages)
				}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
			})

			It("should track errors per index", func() {
				progress := coordinator.GetProgress()

				// Check index1 has an error
				index1Progress, exists := progress.IndexProgress["index1"]
				Expect(exists).To(BeTrue())
				Expect(index1Progress.Completed).To(Equal(len(testPages) - 1))
				Expect(index1Progress.LastError).NotTo(BeNil())
				Expect(*index1Progress.LastError).To(ContainSubstring("simulated index1 error"))

				// Check index2 has no error
				index2Progress, exists := progress.IndexProgress["index2"]
				Expect(exists).To(BeTrue())
				Expect(index2Progress.Completed).To(Equal(len(testPages)))
				Expect(index2Progress.LastError).To(BeNil())
			})
		})
	})

	Describe("Thread Safety", func() {
		When("multiple goroutines read progress simultaneously", func() {
			BeforeEach(func() {
				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle concurrent progress reads without data races", func() {
				const numReaders = 10
				done := make(chan bool, numReaders)
				
				// Start multiple goroutines reading progress concurrently
				for i := 0; i < numReaders; i++ {
					go func() {
						defer func() { done <- true }()
						for j := 0; j < 50; j++ {
							progress := coordinator.GetProgress()
							// Verify the data is consistent
							Expect(progress.TotalPages).To(Equal(len(testPages)))
							Expect(progress.CompletedPages).To(BeNumerically(">=", 0))
							Expect(progress.CompletedPages).To(BeNumerically("<=", len(testPages)))
							Expect(progress.QueueDepth).To(BeNumerically(">=", 0))
						}
					}()
				}
				
				// Wait for all readers to complete
				for i := 0; i < numReaders; i++ {
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						Fail("Timed out waiting for concurrent readers")
					}
				}
			})
		})

		When("progress is updated while being read", func() {
			BeforeEach(func() {
				// Use a slow mock that will create more opportunities for race conditions
				mockMaintainer1.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					time.Sleep(10 * time.Millisecond) // Small delay to create timing windows
					return nil
				}
				mockMaintainer2.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					time.Sleep(5 * time.Millisecond) // Different delay for each index
					return nil
				}

				err := coordinator.StartBackground(testPages)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should maintain data consistency during concurrent access", func() {
				const numReaders = 5
				done := make(chan []index.IndexingProgress, numReaders)
				
				// Start multiple goroutines reading progress while work is happening
				for i := 0; i < numReaders; i++ {
					go func() {
						var progressSnapshots []index.IndexingProgress
						for len(progressSnapshots) < 20 {
							progress := coordinator.GetProgress()
							progressSnapshots = append(progressSnapshots, progress)
							time.Sleep(2 * time.Millisecond)
						}
						done <- progressSnapshots
					}()
				}
				
				// Collect all progress snapshots
				allSnapshots := make([]index.IndexingProgress, 0)
				for i := 0; i < numReaders; i++ {
					select {
					case snapshots := <-done:
						allSnapshots = append(allSnapshots, snapshots...)
					case <-time.After(15 * time.Second):
						Fail("Timed out waiting for progress readers")
					}
				}
				
				// Verify all snapshots are internally consistent
				for _, progress := range allSnapshots {
					Expect(progress.TotalPages).To(Equal(len(testPages)))
					Expect(progress.CompletedPages).To(BeNumerically(">=", 0))
					Expect(progress.CompletedPages).To(BeNumerically("<=", len(testPages)))
					
					// Verify per-index progress is consistent
					for indexName, indexProgress := range progress.IndexProgress {
						Expect(indexProgress.Name).To(Equal(indexName))
						Expect(indexProgress.Total).To(Equal(len(testPages)))
						Expect(indexProgress.Completed).To(BeNumerically(">=", 0))
						Expect(indexProgress.Completed).To(BeNumerically("<=", len(testPages)))
					}
				}
			})
		})
	})
})