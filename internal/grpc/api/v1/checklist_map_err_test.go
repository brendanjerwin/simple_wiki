//revive:disable:dot-imports
package v1

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
)

var _ = Describe("mapChecklistMutatorErr", func() {
	When("err is nil", func() {
		It("should return nil", func() {
			Expect(mapChecklistMutatorErr(nil)).To(BeNil())
		})
	})

	When("err is ErrItemNotFound", func() {
		var result error

		BeforeEach(func() {
			result = mapChecklistMutatorErr(checklistmutator.ErrItemNotFound)
		})

		It("should return NotFound", func() {
			Expect(status.Code(result)).To(Equal(codes.NotFound))
		})
	})

	When("err wraps ErrItemNotFound", func() {
		var result error

		BeforeEach(func() {
			wrapped := fmt.Errorf("context: %w", checklistmutator.ErrItemNotFound)
			result = mapChecklistMutatorErr(wrapped)
		})

		It("should return NotFound", func() {
			Expect(status.Code(result)).To(Equal(codes.NotFound))
		})
	})

	When("err is ErrListNotFound", func() {
		var result error

		BeforeEach(func() {
			result = mapChecklistMutatorErr(checklistmutator.ErrListNotFound)
		})

		It("should return NotFound", func() {
			Expect(status.Code(result)).To(Equal(codes.NotFound))
		})
	})

	When("err is ErrPageNotFound", func() {
		var result error

		BeforeEach(func() {
			result = mapChecklistMutatorErr(checklistmutator.ErrPageNotFound)
		})

		It("should return NotFound", func() {
			Expect(status.Code(result)).To(Equal(codes.NotFound))
		})

		It("should say 'page not found'", func() {
			Expect(result.Error()).To(ContainSubstring("page not found"))
		})
	})

	When("err is already a gRPC status error", func() {
		var (
			original error
			result   error
		)

		BeforeEach(func() {
			original = status.Error(codes.FailedPrecondition, "optimistic lock failure")
			result = mapChecklistMutatorErr(original)
		})

		It("should pass the error through unchanged", func() {
			Expect(result).To(Equal(original))
		})

		It("should preserve the original status code", func() {
			Expect(status.Code(result)).To(Equal(codes.FailedPrecondition))
		})
	})

	When("err is a generic (non-status) error", func() {
		var result error

		BeforeEach(func() {
			result = mapChecklistMutatorErr(errors.New("disk I/O failure"))
		})

		It("should return Internal", func() {
			Expect(status.Code(result)).To(Equal(codes.Internal))
		})

		It("should wrap the error message", func() {
			Expect(result.Error()).To(ContainSubstring("checklist mutation"))
		})
	})
})
