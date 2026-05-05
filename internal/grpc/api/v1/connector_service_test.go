//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:redundant-import-alias
package v1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// keepConnectorTestEmail is the test user's login. Must round-trip
// through ProfileIdentifierFor to derive the binding-store key.
const keepConnectorTestEmail = "alice@example.com"

// withCallerIdentity wraps a context with a real-user Tailscale identity
// so the requireRealUser handler gate accepts the request.
func withCallerIdentity(ctx context.Context, login string) context.Context {
	return tailscale.ContextWithIdentity(ctx, tailscale.NewIdentity(login, "Alice", "node-1"))
}

var _ = Describe("ConnectorService dead-letter handlers (GOOGLE_KEEP, engine path)", func() {
	var (
		ctx    context.Context
		server *v1.Server
		mock   *MockPageReaderMutator
	)

	const (
		page     = "Groceries"
		listName = "weekly"
	)

	BeforeEach(func() {
		var err error
		_, err = wikipage.ProfileIdentifierFor(keepConnectorTestEmail)
		Expect(err).ToNot(HaveOccurred())

		mock = &MockPageReaderMutator{}
		server = mustNewServer(mock, nil, nil)
		ctx = withCallerIdentity(context.Background(), keepConnectorTestEmail)
	})

	Describe("ListDeadLetters when Keep is not wired", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				Page:          page,
				ListName:      listName,
			})
		})

		It("should return FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
		})
	})

	Describe("ListDeadLetters with empty page or list_name", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				Page:          "",
				ListName:      listName,
			})
		})

		It("should return FailedPrecondition (Keep not wired short-circuits before the InvalidArgument check)", func() {
			// With Keep unwired, the not-configured short-circuit
			// fires before the InvalidArgument check. The legacy
			// tests asserted InvalidArgument here; that contract
			// moves to the engine-path test suite which lands in
			// 5-B alongside the legacy package deletion.
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("ListDeadLetters without connector_kind", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
				Page:     page,
				ListName: listName,
			})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "connector_kind"))
		})
	})

	Describe("ListDeadLetters with Tasks not wired", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:          page,
				ListName:      listName,
			})
		})

		It("should return FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
		})
	})

	Describe("ClearDeadLetter without connector_kind", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
				Page:     page,
				ListName: listName,
				ItemUid:  "uid-at",
			})
		})

		It("should return InvalidArgument", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "connector_kind"))
		})
	})

	// NOTE: Detailed dead-letter listing + clearing tests against the
	// legacy *keepsync.Connector lived here pre-Phase-5-A. The engine
	// path's per-binding dead-letter ledger is not yet exposed via the
	// gRPC layer (the listDeadLettersKeep / clearDeadLetterKeep
	// handlers are no-op stubs returning empty / NotFound, mirroring
	// the Tasks engine path). When the engine grows a ledger query,
	// the tests for that surface land here.
})
