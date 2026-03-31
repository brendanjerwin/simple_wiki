//revive:disable:dot-imports
package server

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// captureLogger captures log messages for test assertions.
type captureLogger struct {
	infoMessages  []string
	warnMessages  []string
	errorMessages []string
}

func (l *captureLogger) Info(format string, args ...any) {
	l.infoMessages = append(l.infoMessages, fmt.Sprintf(format, args...))
}

func (l *captureLogger) Warn(format string, args ...any) {
	l.warnMessages = append(l.warnMessages, fmt.Sprintf(format, args...))
}

func (l *captureLogger) Error(format string, args ...any) {
	l.errorMessages = append(l.errorMessages, fmt.Sprintf(format, args...))
}

var _ = Describe("PageInventoryNormalizationJob", func() {
	var (
		deps   *mockPageReaderMutator
		logger *captureLogger
		job    *PageInventoryNormalizationJob
	)

	BeforeEach(func() {
		deps = newMockPageReaderMutator()
		logger = &captureLogger{}
	})

	Describe("GetName", func() {
		BeforeEach(func() {
			job = NewPageInventoryNormalizationJob("some_page", deps, logger)
		})

		It("should return the correct job name", func() {
			Expect(job.GetName()).To(Equal(PageInventoryNormalizationJobName))
		})
	})

	Describe("Execute", func() {
		When("the page exists with no inventory items", func() {
			var err error

			BeforeEach(func() {
				deps.setPage("simple_page", map[string]any{
					"identifier": "simple_page",
				}, "")
				job = NewPageInventoryNormalizationJob("simple_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not log any info messages", func() {
				Expect(logger.infoMessages).To(BeEmpty())
			})

			It("should not log any warning messages", func() {
				Expect(logger.warnMessages).To(BeEmpty())
			})
		})

		When("reading frontmatter fails with a non-NotExist error", func() {
			var err error

			BeforeEach(func() {
				deps.readFrontMatterErr = errors.New("storage failure")
				job = NewPageInventoryNormalizationJob("some_page", deps, logger)
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the read error", func() {
				Expect(err.Error()).To(ContainSubstring("storage failure"))
			})
		})

		When("the page has inventory items that need to be created", func() {
			var err error

			BeforeEach(func() {
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"new_item_1"},
					},
				}, "")
				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should log an info message about created pages", func() {
				Expect(logger.infoMessages).NotTo(BeEmpty())
			})

			It("should include the page ID in the info message", func() {
				Expect(logger.infoMessages[0]).To(ContainSubstring("container_page"))
			})
		})

		When("creating item pages fails due to a write error", func() {
			var err error

			BeforeEach(func() {
				// Pre-set is_container=true so ensureIsContainerField skips its WriteFrontMatter call,
				// allowing us to test only the CreateItemPage failure path.
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"is_container": true,
						"items":        []any{"failing_item"},
					},
				}, "")
				deps.writeFrontMatterErr = errors.New("disk full")

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should log warning messages about failed page creation", func() {
				Expect(logger.warnMessages).NotTo(BeEmpty())
			})

			It("should include the container page ID in the warning", func() {
				Expect(logger.warnMessages[0]).To(ContainSubstring("container_page"))
			})
		})
	})
})
