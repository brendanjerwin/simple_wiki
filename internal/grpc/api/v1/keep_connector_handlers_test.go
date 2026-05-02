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
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
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

var _ = Describe("KeepConnectorService handlers", func() {
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

	// ------------------------------------------------------------------ ExchangeAndStore

	Describe("ExchangeAndStore", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.ExchangeAndStore(ctx, &apiv1.ExchangeAndStoreRequest{
					Email: keepConnectorTestEmail, OauthToken: "oauth",
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
				_, err = server.ExchangeAndStore(context.Background(), &apiv1.ExchangeAndStoreRequest{
					Email: keepConnectorTestEmail, OauthToken: "oauth",
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when called with valid credentials and a passing Keep verification", func() {
			var (
				resp *apiv1.ExchangeAndStoreResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				auth := handlerFakeAuth{masterToken: "new-master-token", bearerToken: "bearer"}
				kc := &handlerFakeKeepClient{}
				c := buildHandlerConnector(mock, auth, kc)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.ExchangeAndStore(ctx, &apiv1.ExchangeAndStoreRequest{
					Email: keepConnectorTestEmail, OauthToken: "captured_oauth",
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
		})
	})

	// ------------------------------------------------------------------ Disconnect

	Describe("Disconnect", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{})
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
				_, err = server.Disconnect(context.Background(), &apiv1.DisconnectRequest{})
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
				resp, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a disconnected (not-configured) state", func() {
				Expect(resp.GetState().GetConfigured()).To(BeFalse())
			})
		})
	})

	// ------------------------------------------------------------------ GetState

	Describe("GetState", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetState(ctx, &apiv1.GetStateRequest{})
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
				_, err = server.GetState(context.Background(), &apiv1.GetStateRequest{})
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
				resp, err = server.GetState(ctx, &apiv1.GetStateRequest{})
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

	// ------------------------------------------------------------------ ListMyBindings

	Describe("ListMyBindings", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.ListMyBindings(ctx, &apiv1.ListMyBindingsRequest{})
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
				_, err = server.ListMyBindings(context.Background(), &apiv1.ListMyBindingsRequest{})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user has one binding", func() {
			var (
				resp *apiv1.ListMyBindingsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.ListMyBindings(ctx, &apiv1.ListMyBindingsRequest{})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return one binding", func() {
				Expect(resp.GetBindings()).To(HaveLen(1))
			})

			It("should return the correct page and list_name", func() {
				b := resp.GetBindings()[0]
				Expect(b.GetPage()).To(Equal(handlerTestPage))
				Expect(b.GetListName()).To(Equal(handlerTestListName))
			})
		})
	})

	// ------------------------------------------------------------------ ListNotes

	Describe("ListNotes", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.ListNotes(ctx, &apiv1.ListNotesRequest{})
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
				_, err = server.ListNotes(context.Background(), &apiv1.ListNotesRequest{})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user is connected and Keep returns a list-type note", func() {
			var (
				resp *apiv1.ListNotesResponse
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
				resp, err = server.ListNotes(ctx, &apiv1.ListNotesRequest{})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return one note", func() {
				Expect(resp.GetNotes()).To(HaveLen(1))
			})

			It("should return the note title", func() {
				Expect(resp.GetNotes()[0].GetTitle()).To(Equal("Shopping List"))
			})

			It("should return the keep_note_id", func() {
				Expect(resp.GetNotes()[0].GetKeepNoteId()).To(Equal("keep-list-1"))
			})
		})
	})

	// ------------------------------------------------------------------ BindChecklist

	Describe("BindChecklist", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.BindChecklist(ctx, &apiv1.BindChecklistRequest{
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
				_, err = server.BindChecklist(context.Background(), &apiv1.BindChecklistRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
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
				_, err = server.BindChecklist(ctx, &apiv1.BindChecklistRequest{
					Page: "", ListName: handlerTestListName,
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
				_, err = server.BindChecklist(ctx, &apiv1.BindChecklistRequest{
					Page: handlerTestPage, ListName: "",
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page and list_name"))
			})
		})

		Describe("when binding to a new Keep note (empty keepNoteId)", func() {
			var (
				resp *apiv1.BindChecklistResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.BindChecklist(ctx, &apiv1.BindChecklistRequest{
					Page:       handlerTestPage,
					ListName:   handlerTestListName,
					KeepNoteId: "", // server creates note on first sync
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the binding", func() {
				Expect(resp.GetBinding()).ToNot(BeNil())
			})

			It("should return the correct page in the binding", func() {
				Expect(resp.GetBinding().GetPage()).To(Equal(handlerTestPage))
			})

			It("should return the correct list_name in the binding", func() {
				Expect(resp.GetBinding().GetListName()).To(Equal(handlerTestListName))
			})
		})
	})

	// ------------------------------------------------------------------ UnbindChecklist

	Describe("UnbindChecklist", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.UnbindChecklist(ctx, &apiv1.UnbindChecklistRequest{
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
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.UnbindChecklist(context.Background(), &apiv1.UnbindChecklistRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when unbinding an existing binding", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.UnbindChecklist(ctx, &apiv1.UnbindChecklistRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("when unbinding a non-existent binding", func() {
			var err error

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				_, err = server.UnbindChecklist(ctx, &apiv1.UnbindChecklistRequest{
					Page: "no-such-page", ListName: "no-such-list",
				})
			})

			// Connector.Unbind is idempotent: ErrBindingNotFound is swallowed so
			// that UI rebind/remove flows don't have to disambiguate.
			It("should not error (idempotent unbind)", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	// ------------------------------------------------------------------ GetChecklistBindingState

	Describe("GetChecklistBindingState", func() {
		Describe("when no connector is wired on the server", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetChecklistBindingState(ctx, &apiv1.GetChecklistBindingStateRequest{
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
				_, err = server.GetChecklistBindingState(context.Background(), &apiv1.GetChecklistBindingStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when the user is connected but has no binding for this checklist", func() {
			var (
				resp *apiv1.GetChecklistBindingStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMock(profileID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.GetChecklistBindingState(ctx, &apiv1.GetChecklistBindingStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report connector_configured = true", func() {
				Expect(resp.GetState().GetConnectorConfigured()).To(BeTrue())
			})

			It("should have no current_binding", func() {
				Expect(resp.GetState().GetCurrentBinding()).To(BeNil())
			})
		})

		Describe("when the user has a binding for this checklist", func() {
			var (
				resp *apiv1.GetChecklistBindingStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedProfileMockWithBinding(profileID, handlerTestPage, handlerTestListName, handlerTestNoteID)
				c := buildHandlerConnector(mock, nil, nil)
				server := mustNewServer(mock, nil, nil).WithKeepConnector(c)
				resp, err = server.GetChecklistBindingState(ctx, &apiv1.GetChecklistBindingStateRequest{
					Page: handlerTestPage, ListName: handlerTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a current_binding", func() {
				Expect(resp.GetState().GetCurrentBinding()).ToNot(BeNil())
			})

			It("should return the correct keep_note_id in current_binding", func() {
				Expect(resp.GetState().GetCurrentBinding().GetKeepNoteId()).To(Equal(handlerTestNoteID))
			})
		})
	})
})
