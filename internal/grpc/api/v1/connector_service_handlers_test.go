//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:redundant-import-alias
package v1_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- test fakes for handler tests ---

// handlerFakeAuth bypasses the real OAuth exchange for handler tests.
type handlerFakeAuth struct {
	masterToken string
	bearerToken string
}

func (f handlerFakeAuth) ExchangeOAuthTokenForMasterToken(_ context.Context, _, _ string) (string, error) {
	return f.masterToken, nil
}

func (f handlerFakeAuth) ExchangeMasterTokenForBearer(_ context.Context, _, _ string) (string, error) {
	return f.bearerToken, nil
}

// handlerFakeKeepClient is a minimal keep client for handler tests.
type handlerFakeKeepClient struct {
	changesResp gateway.ChangesResponse
}

func (f *handlerFakeKeepClient) Changes(_ context.Context, _ gateway.ChangesRequest) (gateway.ChangesResponse, error) {
	return f.changesResp, nil
}

func (*handlerFakeKeepClient) CreateList(_ context.Context, _ string) (string, error) {
	return "new-keep-note-id", nil
}

func (*handlerFakeKeepClient) CreateListWithItems(_ context.Context, _ string, _ []gateway.ListItemSpec) (gateway.CreateListResult, error) {
	return gateway.CreateListResult{}, nil
}

// buildHandlerConnector constructs a Connector with injectable fakes.
// auth and kc may be nil (production defaults are used when nil).
func buildHandlerConnector(mock *MockPageReaderMutator, auth keepsync.AuthExchanger, kc *handlerFakeKeepClient) *keepsync.Connector {
	store := keepsync.NewBindingStore(mock)
	c := keepsync.NewConnector(store, http.DefaultClient, keepConnectorClock{})
	if auth != nil {
		c.SetAuthBuilder(func(_ string) keepsync.AuthExchanger { return auth })
	}
	if kc != nil {
		c.SetClientBuilder(func(_ string) keepsync.KeepClient { return kc })
	}
	return c
}

// connectedProfileMock returns a MockPageReaderMutator seeded with a
// connected Keep connector (email + master_token, no bindings).
func connectedProfileMock(profileID wikipage.PageIdentifier) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        keepConnectorTestEmail,
						"master_token": "test-master-token",
					},
				},
			},
		},
	}
	return mock
}

// connectedProfileMockWithBinding extends connectedProfileMock with one binding.
func connectedProfileMockWithBinding(profileID wikipage.PageIdentifier, page, listName, noteID string) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        keepConnectorTestEmail,
						"master_token": "test-master-token",
						"bindings": []any{
							map[string]any{
								"page":                  page,
								"list_name":             listName,
								"keep_note_id":          noteID,
								"migrated_fingerprints": true,
								"item_id_map":           map[string]any{},
							},
						},
					},
				},
			},
		},
	}
	return mock
}

var _ = Describe("ConnectorService handlers (GOOGLE_KEEP)", func() {
	var (
		ctx       context.Context
		profileID wikipage.PageIdentifier
	)

	const (
		handlerTestPage     = "Shopping"
		handlerTestListName = "groceries"
		handlerTestNoteID   = "note-abc-123"
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(keepConnectorTestEmail)
		Expect(err).ToNot(HaveOccurred())
		ctx = withCallerIdentity(context.Background(), keepConnectorTestEmail)
	})

	// ------------------------------------------------------------------ CompleteAuth (Keep)

	Describe("CompleteAuth", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepConnectorTestEmail, OauthToken: "oauth",
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock,
					handlerFakeAuth{masterToken: "tok", bearerToken: "bearer"},
					&handlerFakeKeepClient{})
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.CompleteAuth(context.Background(), &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepConnectorTestEmail, OauthToken: "oauth",
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when called with valid credentials and a passing Keep verification", func() {
			var (
				resp *apiv1.CompleteAuthResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				auth := handlerFakeAuth{masterToken: "new-master-token", bearerToken: "bearer"}
				kc := &handlerFakeKeepClient{}
				c := buildHandlerConnector(mock, auth, kc)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepConnectorTestEmail, OauthToken: "captured_oauth",
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a configured state", func() {
				Expect(resp.GetState().GetConfigured()).To(BeTrue())
			})

			It("should return the caller email", func() {
				Expect(resp.GetState().GetEmail()).To(Equal(keepConnectorTestEmail))
			})

			It("should echo connector_kind on the state", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})

		Describe("when called without a connector_kind", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock,
					handlerFakeAuth{masterToken: "tok", bearerToken: "bearer"},
					&handlerFakeKeepClient{})
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					Email: keepConnectorTestEmail, OauthToken: "oauth",
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "connector_kind"))
			})
		})
	})

	// ------------------------------------------------------------------ BeginAuth

	Describe("BeginAuth", func() {
		Describe("when called with GOOGLE_KEEP", func() {
			var (
				resp *apiv1.BeginAuthResponse
				err  error
			)

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				resp, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error (Keep is single-shot)", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty authorization_url", func() {
				Expect(resp.GetAuthorizationUrl()).To(Equal(""))
			})
		})

		Describe("when called without a connector_kind", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "connector_kind"))
			})
		})

		Describe("when called with GOOGLE_TASKS (no Tasks connector wired)", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should return Unimplemented", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unimplemented, "unsupported connector_kind"))
			})
		})
	})

	// ------------------------------------------------------------------ Disconnect

	Describe("Disconnect", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Disconnect(context.Background(), &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user is connected", func() {
			var (
				resp *apiv1.DisconnectResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a disconnected (not-configured) state", func() {
				Expect(resp.GetState().GetConfigured()).To(BeFalse())
			})
		})

		Describe("when called without a connector_kind", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "connector_kind"))
			})
		})
	})

	// ------------------------------------------------------------------ GetState

	Describe("GetState", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.GetState(context.Background(), &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user has a connected connector", func() {
			var (
				resp *apiv1.GetStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report the connector as configured", func() {
				Expect(resp.GetState().GetConfigured()).To(BeTrue())
			})

			It("should return the email", func() {
				Expect(resp.GetState().GetEmail()).To(Equal(keepConnectorTestEmail))
			})
		})
	})

	// ------------------------------------------------------------------ ListMySubscriptions

	Describe("ListMySubscriptions", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.ListMySubscriptions(ctx, &apiv1.ListMySubscriptionsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.ListMySubscriptions(context.Background(), &apiv1.ListMySubscriptionsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user has one subscription", func() {
			var (
				resp *apiv1.ListMySubscriptionsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.ListMySubscriptions(ctx, &apiv1.ListMySubscriptionsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return one subscription", func() {
				Expect(resp.GetSubscriptions()).To(HaveLen(1))
			})

			It("should return the correct page and list_name", func() {
				s := resp.GetSubscriptions()[0]
				Expect(s.GetPage()).To(Equal(handlerTestPage))
				Expect(s.GetListName()).To(Equal(handlerTestListName))
			})

			It("should set connector_kind on the subscription", func() {
				Expect(resp.GetSubscriptions()[0].GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})
	})

	// ------------------------------------------------------------------ ListRemoteLists

	Describe("ListRemoteLists", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				kc := &handlerFakeKeepClient{}
				auth := handlerFakeAuth{masterToken: "tok", bearerToken: "bearer"}
				c := buildHandlerConnector(mock, auth, kc)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.ListRemoteLists(context.Background(), &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user is connected and Keep returns a list-type note", func() {
			var (
				resp *apiv1.ListRemoteListsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				kc := &handlerFakeKeepClient{
					changesResp: gateway.ChangesResponse{
						Nodes: []gateway.Node{
							{
								ServerID: "keep-list-1",
								Type:     gateway.NodeTypeList,
								Title:    "Shopping List",
							},
						},
					},
				}
				auth := handlerFakeAuth{masterToken: "tok", bearerToken: "bearer"}
				c := buildHandlerConnector(mock, auth, kc)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return one remote list", func() {
				Expect(resp.GetLists()).To(HaveLen(1))
			})

			It("should return the list title", func() {
				Expect(resp.GetLists()[0].GetTitle()).To(Equal("Shopping List"))
			})

			It("should return the remote_list_handle", func() {
				Expect(resp.GetLists()[0].GetRemoteListHandle()).To(Equal("keep-list-1"))
			})
		})
	})

	// ------------------------------------------------------------------ Subscribe

	Describe("Subscribe", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Subscribe(context.Background(), &apiv1.SubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when called with empty page", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          "", ListName: handlerTestListName,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page and list_name"))
			})
		})

		Describe("when called with empty list_name", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: "",
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page and list_name"))
			})
		})

		Describe("when subscribing to a new Keep note (empty remote_list_handle)", func() {
			var (
				resp *apiv1.SubscribeResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             handlerTestPage,
					ListName:         handlerTestListName,
					RemoteListHandle: "", // server creates note on first sync
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the subscription", func() {
				Expect(resp.GetSubscription()).ToNot(BeNil())
			})

			It("should return the correct page in the subscription", func() {
				Expect(resp.GetSubscription().GetPage()).To(Equal(handlerTestPage))
			})

			It("should return the correct list_name in the subscription", func() {
				Expect(resp.GetSubscription().GetListName()).To(Equal(handlerTestListName))
			})
		})
	})

	// ------------------------------------------------------------------ Unsubscribe

	Describe("Unsubscribe", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Unsubscribe(context.Background(), &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when unsubscribing an existing subscription", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("when unsubscribing a non-existent subscription", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          "no-such-page", ListName: "no-such-list",
				})
			})

			// Connector.Unbind is idempotent: ErrBindingNotFound is swallowed so
			// that UI rebind/remove flows don't have to disambiguate.
			It("should not error (idempotent unsubscribe)", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	// ------------------------------------------------------------------ GetChecklistSubscriptionState

	Describe("GetChecklistSubscriptionState", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetChecklistSubscriptionState(ctx, &apiv1.GetChecklistSubscriptionStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "not configured"))
			})
		})

		Describe("when called without authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.GetChecklistSubscriptionState(context.Background(), &apiv1.GetChecklistSubscriptionStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user is connected but has no subscription for this checklist", func() {
			var (
				resp *apiv1.GetChecklistSubscriptionStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.GetChecklistSubscriptionState(ctx, &apiv1.GetChecklistSubscriptionStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report connector_configured = true", func() {
				Expect(resp.GetState().GetConnectorConfigured()).To(BeTrue())
			})

			It("should have no current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription()).To(BeNil())
			})
		})

		Describe("when the user has a subscription for this checklist", func() {
			var (
				resp *apiv1.GetChecklistSubscriptionStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.GetChecklistSubscriptionState(ctx, &apiv1.GetChecklistSubscriptionStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription()).ToNot(BeNil())
			})

			It("should return the correct remote_list_handle in current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription().GetRemoteListHandle()).To(Equal(handlerTestNoteID))
			})

			It("should set connector_kind on the current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})
	})
})
