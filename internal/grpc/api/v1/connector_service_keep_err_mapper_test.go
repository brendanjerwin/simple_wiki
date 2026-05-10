//revive:disable:dot-imports
package v1

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	keepgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
)

// These tests pin mapKeepConnectorErr's branch coverage for the new
// 403-classification sentinels (ErrServiceDisabled, ErrPermissionDenied)
// added alongside the body-aware classifier in the Keep gateway. They
// mirror the Tasks coverage in connector_service_err_mapper_test.go —
// the bug they exist to prevent is the 2026-05-02 incident where
// "Tasks API not enabled" (HTTP 403, accessNotConfigured) bubbled to
// the UI as "rate_limited" because every 4xx collapsed to a single
// sentinel. Same anti-pattern, same fix, per-package.
var _ = Describe("mapKeepConnectorErr 403-class coverage", func() {
	When("err is gateway.ErrServiceDisabled (Keep API not enabled)", func() {
		var result error

		BeforeEach(func() {
			// The gateway's classifier already wraps the activation URL
			// into the error message via fmt.Errorf("%w: ... %s ...").
			// Mirror that shape here so the FailedPrecondition message
			// surfaces the URL the user clicks through to enable the API.
			injected := fmt.Errorf("%w: Google Keep API is not enabled on the GCP project. Enable it at https://console.developers.google.com/apis/api/keep.googleapis.com/overview?project=703961900896 and try again. (stage3 HTTP 403: dummy body)", keepgateway.ErrServiceDisabled)
			result = mapKeepConnectorErr(injected)
		})

		It("should return FailedPrecondition", func() {
			st, ok := status.FromError(result)
			Expect(ok).To(BeTrue())
			Expect(st.Code()).To(Equal(codes.FailedPrecondition))
		})

		It("should mention keep_api_not_enabled", func() {
			Expect(result.Error()).To(ContainSubstring("keep_api_not_enabled"))
		})

		It("should preserve the activation URL in the message", func() {
			Expect(result.Error()).To(ContainSubstring("console.developers.google.com"))
		})
	})

	When("err is gateway.ErrPermissionDenied", func() {
		var result error

		BeforeEach(func() {
			result = mapKeepConnectorErr(keepgateway.ErrPermissionDenied)
		})

		It("should return PermissionDenied", func() {
			st, ok := status.FromError(result)
			Expect(ok).To(BeTrue())
			Expect(st.Code()).To(Equal(codes.PermissionDenied))
		})

		It("should mention permission_denied", func() {
			Expect(result.Error()).To(ContainSubstring("permission_denied"))
		})
	})

	When("err is gateway.ErrRateLimited (regression: ensure NOT collapsed to ServiceDisabled)", func() {
		var result error

		BeforeEach(func() {
			result = mapKeepConnectorErr(keepgateway.ErrRateLimited)
		})

		It("should return ResourceExhausted", func() {
			st, ok := status.FromError(result)
			Expect(ok).To(BeTrue())
			Expect(st.Code()).To(Equal(codes.ResourceExhausted))
		})

		It("should NOT mention keep_api_not_enabled (the prior bug)", func() {
			Expect(result.Error()).NotTo(ContainSubstring("keep_api_not_enabled"))
		})
	})

	When("err is an unknown error (default branch)", func() {
		var result error

		BeforeEach(func() {
			result = mapKeepConnectorErr(errors.New("totally unexpected keep error"))
		})

		It("should return Internal", func() {
			st, ok := status.FromError(result)
			Expect(ok).To(BeTrue())
			Expect(st.Code()).To(Equal(codes.Internal))
		})

		It("should mention keep connector", func() {
			Expect(result.Error()).To(ContainSubstring("keep connector"))
		})
	})
})
