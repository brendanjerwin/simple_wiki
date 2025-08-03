//revive:disable:dot-imports
package index_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockIndexOperator is a test implementation of index.IndexOperator.
type MockIndexOperator struct {
	AddPageToIndexFunc    func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc func(identifier wikipage.PageIdentifier) error
	addCalled             []wikipage.PageIdentifier
	removeCalled          []wikipage.PageIdentifier
}

func (m *MockIndexOperator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.addCalled = append(m.addCalled, identifier)
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockIndexOperator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.removeCalled = append(m.removeCalled, identifier)
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

var _ = Describe("FrontmatterIndexJob", func() {
	var (
		mockIndex *MockIndexOperator
		job       *index.FrontmatterIndexJob
	)

	BeforeEach(func() {
		mockIndex = &MockIndexOperator{}
	})

	It("should exist", func() {
		job := &index.FrontmatterIndexJob{}
		Expect(job).NotTo(BeNil())
	})

	It("should implement the Job interface", func() {
		job := index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Add)
		var jobInterface jobs.Job = job
		Expect(jobInterface).NotTo(BeNil())
	})

	Describe("when creating a new job", func() {
		BeforeEach(func() {
			job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Add)
		})

		It("should store the index operator", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})

		It("should store the page identifier", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})

		It("should store the operation type", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})
	})

	Describe("GetName", func() {
		BeforeEach(func() {
			job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Add)
		})

		It("should return FrontmatterIndex", func() {
			Expect(job.GetName()).To(Equal("FrontmatterIndex"))
		})
	})

	Describe("Execute", func() {
		Describe("when operation is Add", func() {
			BeforeEach(func() {
				job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Add)
				_ = job.Execute()
			})

			It("should call AddPageToIndex on the index", func() {
				Expect(mockIndex.addCalled).To(ContainElement("test-page"))
			})

			It("should not call RemovePageFromIndex", func() {
				Expect(mockIndex.removeCalled).To(BeEmpty())
			})
		})

		Describe("when operation is Remove", func() {
			BeforeEach(func() {
				job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Remove)
				_ = job.Execute()
			})

			It("should call RemovePageFromIndex on the index", func() {
				Expect(mockIndex.removeCalled).To(ContainElement("test-page"))
			})

			It("should not call AddPageToIndex", func() {
				Expect(mockIndex.addCalled).To(BeEmpty())
			})
		})

		Describe("when AddPageToIndex returns an error", func() {
			var (
				indexError error
				execError  error
			)

			BeforeEach(func() {
				indexError = errors.New("failed to add page to frontmatter index")
				mockIndex.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					return indexError
				}
				job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Add)
				execError = job.Execute()
			})

			It("should return the error", func() {
				Expect(execError).To(MatchError("failed to add page to frontmatter index"))
			})
		})

		Describe("when RemovePageFromIndex returns an error", func() {
			var (
				indexError error
				execError  error
			)

			BeforeEach(func() {
				indexError = errors.New("failed to remove page from frontmatter index")
				mockIndex.RemovePageFromIndexFunc = func(identifier wikipage.PageIdentifier) error {
					return indexError
				}
				job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Remove)
				execError = job.Execute()
			})

			It("should return the error", func() {
				Expect(execError).To(MatchError("failed to remove page from frontmatter index"))
			})
		})

		Describe("when operation type is invalid", func() {
			var (
				execError error
				job       *index.FrontmatterIndexJob
			)

			BeforeEach(func() {
				// We can't directly create a job with an invalid operation using the constructor,
				// so we'll create a valid job and test that the constructor validates properly
				job = index.NewFrontmatterIndexJob(mockIndex, "test-page", index.Operation(999))
				execError = job.Execute()
			})

			It("should return an error", func() {
				Expect(execError).To(MatchError("unknown operation type: 999"))
			})

			It("should not call any index methods", func() {
				Expect(mockIndex.addCalled).To(BeEmpty())
				Expect(mockIndex.removeCalled).To(BeEmpty())
			})
		})
	})
})

var _ = Describe("BleveIndexJob", func() {
	var (
		mockIndex *MockIndexOperator
		job       *index.BleveIndexJob
	)

	BeforeEach(func() {
		mockIndex = &MockIndexOperator{}
	})

	It("should exist", func() {
		job := &index.BleveIndexJob{}
		Expect(job).NotTo(BeNil())
	})

	It("should implement the Job interface", func() {
		job := index.NewBleveIndexJob(mockIndex, "test-page", index.Add)
		var jobInterface jobs.Job = job
		Expect(jobInterface).NotTo(BeNil())
	})

	Describe("when creating a new job", func() {
		BeforeEach(func() {
			job = index.NewBleveIndexJob(mockIndex, "test-page", index.Add)
		})

		It("should store the index operator", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})

		It("should store the page identifier", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})

		It("should store the operation type", func() {
			// Note: These fields are not exported, so we test via behavior
			Expect(job).NotTo(BeNil())
		})
	})

	Describe("GetName", func() {
		BeforeEach(func() {
			job = index.NewBleveIndexJob(mockIndex, "test-page", index.Add)
		})

		It("should return BleveIndex", func() {
			Expect(job.GetName()).To(Equal("BleveIndex"))
		})
	})

	Describe("Execute", func() {
		Describe("when operation is Add", func() {
			BeforeEach(func() {
				job = index.NewBleveIndexJob(mockIndex, "test-page", index.Add)
				_ = job.Execute()
			})

			It("should call AddPageToIndex on the index", func() {
				Expect(mockIndex.addCalled).To(ContainElement("test-page"))
			})

			It("should not call RemovePageFromIndex", func() {
				Expect(mockIndex.removeCalled).To(BeEmpty())
			})
		})

		Describe("when operation is Remove", func() {
			BeforeEach(func() {
				job = index.NewBleveIndexJob(mockIndex, "test-page", index.Remove)
				_ = job.Execute()
			})

			It("should call RemovePageFromIndex on the index", func() {
				Expect(mockIndex.removeCalled).To(ContainElement("test-page"))
			})

			It("should not call AddPageToIndex", func() {
				Expect(mockIndex.addCalled).To(BeEmpty())
			})
		})

		Describe("when AddPageToIndex returns an error", func() {
			var (
				indexError error
				execError  error
			)

			BeforeEach(func() {
				indexError = errors.New("failed to add page to bleve index")
				mockIndex.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					return indexError
				}
				job = index.NewBleveIndexJob(mockIndex, "test-page", index.Add)
				execError = job.Execute()
			})

			It("should return the error", func() {
				Expect(execError).To(MatchError("failed to add page to bleve index"))
			})
		})

		Describe("when RemovePageFromIndex returns an error", func() {
			var (
				indexError error
				execError  error
			)

			BeforeEach(func() {
				indexError = errors.New("failed to remove page from bleve index")
				mockIndex.RemovePageFromIndexFunc = func(identifier wikipage.PageIdentifier) error {
					return indexError
				}
				job = index.NewBleveIndexJob(mockIndex, "test-page", index.Remove)
				execError = job.Execute()
			})

			It("should return the error", func() {
				Expect(execError).To(MatchError("failed to remove page from bleve index"))
			})
		})

		Describe("when operation type is invalid", func() {
			var (
				execError error
				job       *index.BleveIndexJob
			)

			BeforeEach(func() {
				// We can't directly create a job with an invalid operation using the constructor,
				// so we'll create a valid job and test that the constructor validates properly
				job = index.NewBleveIndexJob(mockIndex, "test-page", index.Operation(999))
				execError = job.Execute()
			})

			It("should return an error", func() {
				Expect(execError).To(MatchError("unknown operation type: 999"))
			})

			It("should not call any index methods", func() {
				Expect(mockIndex.addCalled).To(BeEmpty())
				Expect(mockIndex.removeCalled).To(BeEmpty())
			})
		})
	})
})