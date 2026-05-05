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
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Phase 5-A cuts the gRPC ConnectorService Keep branches over to the
// engine path. Detailed handler tests against the legacy
// *keepsync.Connector + gpsoauth fakes lived in this file pre-cutover;
// they're replaced here by the smaller "engine-path-not-wired ⇒
// FailedPrecondition" surface tests that mirror the Tasks shape.
//
// The full engine-path handler test suite (BeginAuth / CompleteAuth /
// Subscribe / Unsubscribe etc. driven through buildKeepWiring with an
// in-memory page store + fake gateway client + real engine + adapter +
// binding store + credential store) lands in Phase 5-B alongside the
// legacy package deletion, mirroring Tasks's Phase 4-3 commit shape.

var _ = Describe("ConnectorService handlers (GOOGLE_KEEP, engine path)", func() {
	var (
		ctx    context.Context
		server *v1.Server
	)

	const (
		handlerTestPage     = "Shopping"
		handlerTestListName = "groceries"
	)

	BeforeEach(func() {
		_, err := wikipage.ProfileIdentifierFor(keepConnectorTestEmail)
		Expect(err).ToNot(HaveOccurred())
		mock := &MockPageReaderMutator{}
		// Server is constructed without WithGoogleKeep — the
		// engine-path handlers therefore short-circuit on
		// keepWired() == false. This pins the not-configured
		// dispatch shape.
		server = mustNewServer(mock, nil, nil)
		ctx = withCallerIdentity(context.Background(), keepConnectorTestEmail)
	})

	Describe("BeginAuth(GOOGLE_KEEP)", func() {
		It("should be a no-op (no URL, no error)", func() {
			resp, err := server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.GetAuthorizationUrl()).To(BeEmpty())
		})
	})

	Describe("CompleteAuth(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				Email:         keepConnectorTestEmail,
				OauthToken:    "oauth",
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("Disconnect(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.Disconnect(ctx, &apiv1.DisconnectRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("GetState(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.GetState(ctx, &apiv1.GetStateRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("ListMySubscriptions(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.ListMySubscriptions(ctx, &apiv1.ListMySubscriptionsRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("ListRemoteLists(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("Subscribe(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				Page:          handlerTestPage,
				ListName:      handlerTestListName,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})

	Describe("Unsubscribe(GOOGLE_KEEP) when Keep is not wired", func() {
		It("should return FailedPrecondition", func() {
			_, err := server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
				ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				Page:          handlerTestPage,
				ListName:      handlerTestListName,
			})
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured"))
		})
	})
})
