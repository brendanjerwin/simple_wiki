//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:redundant-import-alias
package v1_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	googlekeep "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- minimal fakes for ConnectorService Keep-branch tests -----------------

// keepTestClock returns a fixed time so cursor/persisted timestamps are
// deterministic in test assertions.
type keepTestClock struct{}

func (keepTestClock) Now() time.Time {
	return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
}

// silentKeepLogger discards every log line; tests assert on side
// effects rather than log content.
type silentKeepLogger struct{}

func (silentKeepLogger) Info(string, ...any)  {}
func (silentKeepLogger) Warn(string, ...any)  {}
func (silentKeepLogger) Error(string, ...any) {}

// fakeKeepClient is a minimal in-memory stand-in for *gateway.KeepClient.
// Records the calls the engine/adapter make so tests can assert delegation
// reached the gateway.
//
// nodesToReturn is the slice the next Changes call returns under
// ChangesResponse.Nodes (typically a LIST + its LIST_ITEMs).
// labelsToReturn populates ChangesResponse.Labels.
// changesErr is returned on every Changes call when non-nil.
// createListErr is returned on every CreateList[WithItems] call when non-nil.
// createdLists records the (title) of each CreateListWithItems call.
type fakeKeepClient struct {
	mu              sync.Mutex
	nodesToReturn   []gateway.Node
	labelsToReturn  []gateway.LabelEntry
	changesErr      error
	createListErr   error
	createdLists    []string
	changesCallSeen []gateway.ChangesRequest
}

func (f *fakeKeepClient) Changes(_ context.Context, req gateway.ChangesRequest) (gateway.ChangesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changesCallSeen = append(f.changesCallSeen, req)
	if f.changesErr != nil {
		return gateway.ChangesResponse{}, f.changesErr
	}
	// Echo any pushed nodes as write-result successes so push paths
	// don't trip writeFailed.
	results := make([]gateway.NodeWriteResult, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		results = append(results, gateway.NodeWriteResult{ID: n.ID, Status: "SUCCESS"})
	}
	return gateway.ChangesResponse{
		ToVersion:    "v-test",
		Nodes:        f.nodesToReturn,
		Labels:       f.labelsToReturn,
		WriteResults: results,
	}, nil
}

func (f *fakeKeepClient) CreateList(_ context.Context, title string) (string, error) {
	if f.createListErr != nil {
		return "", f.createListErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdLists = append(f.createdLists, title)
	return "created-" + title, nil
}

func (f *fakeKeepClient) CreateListWithItems(_ context.Context, title string, _ []gateway.ListItemSpec) (gateway.CreateListResult, error) {
	if f.createListErr != nil {
		return gateway.CreateListResult{}, f.createListErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdLists = append(f.createdLists, title)
	listServerID := "created-" + title
	listClientID := "client-" + title
	// Stage the freshly-created LIST node into nodesToReturn so the
	// subsequent ValidateRemoteBinding pull (which the engine.Bind
	// ceremony makes immediately after the create) finds it. This
	// mirrors Keep's actual behavior — a freshly-created LIST is
	// visible on the next Changes pull.
	f.nodesToReturn = append(f.nodesToReturn, gateway.Node{
		ServerID: listServerID,
		ID:       listClientID,
		Type:     gateway.NodeTypeList,
		Title:    title,
	})
	return gateway.CreateListResult{
		ListServerID: listServerID,
		ListClientID: listClientID,
	}, nil
}

// keepTestProfileEmail must round-trip through ProfileIdentifierFor so
// the per-profile lookup in the binding/credential store matches what
// the requireRealUser handler derives.
const keepTestProfileEmail = "alice@example.com"

// connectedKeepProfileMock builds a MockPageReaderMutator seeded with
// a Keep-connected profile (email + master_token). The credential
// store reads from this same wiki.connectors.google_keep.* subtree.
func connectedKeepProfileMock(profileID wikipage.PageIdentifier) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        keepTestProfileEmail,
						"master_token": "mt-test-token",
						"android_id":   "1234567890abcdef",
					},
				},
			},
		},
	}
	return mock
}

// connectedKeepProfileMockWithBinding extends the above with one
// engine-shape binding so list/get state tests have something to read
// back. The new shape uses bindings[] (not legacy subscriptions[]).
func connectedKeepProfileMockWithBinding(profileID wikipage.PageIdentifier, page, listName, remoteHandle string) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        keepTestProfileEmail,
						"master_token": "mt-test-token",
						"android_id":   "1234567890abcdef",
						"bindings": []any{
							map[string]any{
								"page":              page,
								"list_name":         listName,
								"remote_handle":     remoteHandle,
								"remote_list_title": "Keep note",
								"state":             "active",
							},
						},
					},
				},
			},
		},
	}
	return mock
}

// stubKeepClientFactory returns a KeepClientFactory that always
// returns the given client (the engine path doesn't need bearer
// exchange in handler tests).
func stubKeepClientFactory(client googlekeep.KeepClient) googlekeep.KeepClientFactory {
	return func(_ context.Context, _ wikipage.PageIdentifier, _, _ string) (googlekeep.KeepClient, error) {
		return client, nil
	}
}

// stubKeepAuthVerifier is a deterministic AuthVerifier for tests.
type stubKeepAuthVerifier struct {
	masterToken string
	err         error
}

func (s stubKeepAuthVerifier) VerifyOAuthToken(_ context.Context, _ wikipage.PageIdentifier, _, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.masterToken, nil
}

// keepTestWiring bundles the engine-path collaborators tests need to
// drive the gRPC handler layer.
type keepTestWiring struct {
	engine          *engine.Engine
	adapter         *googlekeep.KeepAdapter
	bindingStore    engine.BindingStore
	credentialStore *googlekeep.FrontmatterCredentialStore
}

// buildKeepWiring wires the engine path against an in-memory page
// store, an optional fake KeepClient (pass nil for default no-op),
// and a fixed test clock.
func buildKeepWiring(mock *MockPageReaderMutator, client googlekeep.KeepClient) *keepTestWiring {
	GinkgoHelper()
	if client == nil {
		client = &fakeKeepClient{}
	}
	logger := silentKeepLogger{}
	bindingStore, err := engine.NewFrontmatterBindingStore(mock, &memoryProfileLister{}, logger)
	Expect(err).ToNot(HaveOccurred())
	credentialStore, err := googlekeep.NewFrontmatterCredentialStore(
		mock,
		googlekeep.SystemClock{},
		logger,
		nil, // pauseAll: no fan-out in handler tests
		nil, // resumeAll: no fan-out in handler tests
	)
	Expect(err).ToNot(HaveOccurred())
	adapter, err := googlekeep.NewKeepAdapter(credentialStore, stubKeepClientFactory(client), keepTestClock{}, logger)
	Expect(err).ToNot(HaveOccurred())
	eng, err := engine.NewEngine(
		adapter,
		readyLeaseTable(),
		emptyChecklistReader{},
		noopMutator{},
		noopSuppressor{},
		logger,
		keepTestClock{},
		bindingStore,
	)
	Expect(err).ToNot(HaveOccurred())
	return &keepTestWiring{
		engine:          eng,
		adapter:         adapter,
		bindingStore:    bindingStore,
		credentialStore: credentialStore,
	}
}

// withKeep chains a keepTestWiring onto the v1.Server builder.
func withKeep(s *v1.Server, w *keepTestWiring) *v1.Server {
	return s.WithGoogleKeep(w.engine, w.adapter, w.bindingStore, w.credentialStore)
}

// --- tests ---------------------------------------------------------------

var _ = Describe("ConnectorService handlers (GOOGLE_KEEP)", func() {
	var (
		ctx       context.Context
		profileID wikipage.PageIdentifier
	)

	const (
		keepTestPage         = "Shopping"
		keepTestListName     = "groceries"
		keepTestRemoteHandle = "keep-note-server-1"
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(keepTestProfileEmail)
		Expect(err).ToNot(HaveOccurred())
		ctx = withCallerIdentity(context.Background(), keepTestProfileEmail)
	})

	// ---------------------------------------------------------- BeginAuth (Keep)

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

			It("should not error — Keep skips BeginAuth (single-shot CompleteAuth flow)", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty response", func() {
				Expect(resp.GetAuthorizationUrl()).To(BeEmpty())
				Expect(resp.GetState()).To(BeEmpty())
			})
		})
	})

	// ---------------------------------------------------------- CompleteAuth (Keep)

	Describe("CompleteAuth", func() {
		Describe("when no Keep engine is wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepTestProfileEmail,
					OauthToken:    "ot-test",
				})
			})

			It("should return FailedPrecondition pointing the user at operator setup", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
			})
		})

		Describe("when Keep is wired but no auth verifier is configured", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepTestProfileEmail,
					OauthToken:    "ot-test",
				})
			})

			It("should return FailedPrecondition because the auth verifier is unset", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "auth verifier not configured"))
			})
		})

		Describe("when Keep is wired with a working auth verifier", func() {
			var (
				resp *apiv1.CompleteAuthResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w).
					WithKeepAuthVerifier(stubKeepAuthVerifier{masterToken: "mt-fresh"})
				resp, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepTestProfileEmail,
					OauthToken:    "ot-test",
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report the connector as configured after Connect succeeds", func() {
				Expect(resp.GetState().GetConfigured()).To(BeTrue())
			})

			It("should set GOOGLE_KEEP as the connector_kind on the state", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})

		Describe("when the auth verifier rejects the oauth_token", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w).
					WithKeepAuthVerifier(stubKeepAuthVerifier{err: gateway.ErrInvalidCredentials})
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Email:         keepTestProfileEmail,
					OauthToken:    "ot-bad",
				})
			})

			It("should return Unauthenticated", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unauthenticated, "invalid_credentials"))
			})
		})
	})

	// ---------------------------------------------------------- Disconnect (Keep)

	Describe("Disconnect", func() {
		Describe("when no Keep engine is wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
			})
		})

		Describe("when the user is connected", func() {
			var (
				resp *apiv1.DisconnectResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a not-configured state", func() {
				Expect(resp.GetState().GetConfigured()).To(BeFalse())
			})

			It("should echo the GOOGLE_KEEP connector_kind on the state", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})
	})

	// ---------------------------------------------------------- GetState (Keep)

	Describe("GetState", func() {
		Describe("when no Keep engine is wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
			})
		})

		Describe("when the user is connected", func() {
			var (
				resp *apiv1.GetStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
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
				Expect(resp.GetState().GetEmail()).To(Equal(keepTestProfileEmail))
			})

			It("should set GOOGLE_KEEP as the connector_kind", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})

		Describe("when the user has never connected", func() {
			var (
				resp *apiv1.GetStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := &MockPageReaderMutator{}
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report the connector as not configured", func() {
				Expect(resp.GetState().GetConfigured()).To(BeFalse())
			})
		})
	})

	// ---------------------------------------------------------- ListMySubscriptions (Keep)

	Describe("ListMySubscriptions", func() {
		Describe("when the user has one binding", func() {
			var (
				resp *apiv1.ListMySubscriptionsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMockWithBinding(profileID, keepTestPage, keepTestListName, keepTestRemoteHandle)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
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
				Expect(s.GetPage()).To(Equal(keepTestPage))
				Expect(s.GetListName()).To(Equal(keepTestListName))
			})

			It("should populate the remote_list_handle from the Keep ServerID", func() {
				Expect(resp.GetSubscriptions()[0].GetRemoteListHandle()).To(Equal(keepTestRemoteHandle))
			})

			It("should set GOOGLE_KEEP as the connector_kind on the subscription", func() {
				Expect(resp.GetSubscriptions()[0].GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})

		Describe("when the user has no bindings", func() {
			var (
				resp *apiv1.ListMySubscriptionsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.ListMySubscriptions(ctx, &apiv1.ListMySubscriptionsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return zero subscriptions", func() {
				Expect(resp.GetSubscriptions()).To(BeEmpty())
			})
		})
	})

	// ---------------------------------------------------------- ListRemoteLists (Keep)

	Describe("ListRemoteLists", func() {
		Describe("when the gateway returns two LIST nodes", func() {
			var (
				resp *apiv1.ListRemoteListsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				client := &fakeKeepClient{
					nodesToReturn: []gateway.Node{
						{ServerID: "list-1", Type: gateway.NodeTypeList, Title: "Groceries"},
						{ServerID: "list-2", Type: gateway.NodeTypeList, Title: "Errands"},
					},
				}
				w := buildKeepWiring(mock, client)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return both LIST nodes", func() {
				Expect(resp.GetLists()).To(HaveLen(2))
			})

			It("should map ServerID into remote_list_handle", func() {
				Expect(resp.GetLists()[0].GetRemoteListHandle()).To(Equal("list-1"))
			})

			It("should map Title into title", func() {
				Expect(resp.GetLists()[0].GetTitle()).To(Equal("Groceries"))
			})
		})

		Describe("when the user is not connected", func() {
			var err error

			BeforeEach(func() {
				mock := &MockPageReaderMutator{}
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
				})
			})

			It("should return FailedPrecondition keep_connector_not_configured", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "keep_connector_not_configured"))
			})
		})
	})

	// ---------------------------------------------------------- Subscribe (Keep)

	Describe("Subscribe", func() {
		Describe("when called with empty remote_list_handle (Bind to a new Keep note)", func() {
			var (
				resp   *apiv1.SubscribeResponse
				err    error
				client *fakeKeepClient
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				client = &fakeKeepClient{}
				w := buildKeepWiring(mock, client)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             keepTestPage,
					ListName:         keepTestListName,
					RemoteListHandle: "",
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should call CreateListWithItems with the wiki list name as the title", func() {
				Expect(client.createdLists).To(ConsistOf(keepTestListName))
			})

			It("should bind to the freshly-created Keep note's ServerID", func() {
				Expect(resp.GetSubscription().GetRemoteListHandle()).To(Equal("created-" + keepTestListName))
			})
		})

		Describe("when called with empty page", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             "",
					ListName:         keepTestListName,
					RemoteListHandle: keepTestRemoteHandle,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page and list_name"))
			})
		})

		Describe("when subscribing to an existing Keep LIST node", func() {
			var (
				resp *apiv1.SubscribeResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				client := &fakeKeepClient{
					nodesToReturn: []gateway.Node{
						{ServerID: keepTestRemoteHandle, ID: "client-id-list", Type: gateway.NodeTypeList, Title: "Groceries"},
					},
				}
				w := buildKeepWiring(mock, client)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             keepTestPage,
					ListName:         keepTestListName,
					RemoteListHandle: keepTestRemoteHandle,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the persisted subscription", func() {
				Expect(resp.GetSubscription()).ToNot(BeNil())
			})

			It("should populate page", func() {
				Expect(resp.GetSubscription().GetPage()).To(Equal(keepTestPage))
			})

			It("should populate list_name", func() {
				Expect(resp.GetSubscription().GetListName()).To(Equal(keepTestListName))
			})

			It("should populate remote_list_handle", func() {
				Expect(resp.GetSubscription().GetRemoteListHandle()).To(Equal(keepTestRemoteHandle))
			})
		})

		Describe("when subscribing to a remote handle that is not a LIST node", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				client := &fakeKeepClient{
					// The handle exists but is a NOTE (free-form), not LIST.
					nodesToReturn: []gateway.Node{
						{ServerID: keepTestRemoteHandle, Type: gateway.NodeTypeNote, Title: "Just a note"},
					},
				}
				w := buildKeepWiring(mock, client)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             keepTestPage,
					ListName:         keepTestListName,
					RemoteListHandle: keepTestRemoteHandle,
				})
			})

			It("should return FailedPrecondition remote_list_not_a_checklist", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "remote_list_not_a_checklist"))
			})
		})

		Describe("when the user has no master_token", func() {
			var err error

			BeforeEach(func() {
				mock := &MockPageReaderMutator{}
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:             keepTestPage,
					ListName:         keepTestListName,
					RemoteListHandle: keepTestRemoteHandle,
				})
			})

			It("should return FailedPrecondition keep_connector_not_configured", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "keep_connector_not_configured"))
			})
		})
	})

	// ---------------------------------------------------------- Unsubscribe (Keep)

	Describe("Unsubscribe", func() {
		Describe("when unsubscribing an existing binding", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMockWithBinding(profileID, keepTestPage, keepTestListName, keepTestRemoteHandle)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          keepTestPage,
					ListName:      keepTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("when unsubscribing a missing binding", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          "no-such-page",
					ListName:      "no-such-list",
				})
			})

			It("should return NotFound", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "subscription_not_found"))
			})
		})
	})

	// ---------------------------------------------------------- ListDeadLetters / ClearDeadLetter (Keep)

	Describe("ListDeadLetters", func() {
		Describe("when called for Keep with a wired engine", func() {
			var (
				resp *apiv1.ListDeadLettersResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          keepTestPage,
					ListName:      keepTestListName,
				})
			})

			It("should not error — Keep's engine path has no dead-letter ledger surface yet", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty list", func() {
				Expect(resp.GetItems()).To(BeEmpty())
			})
		})
	})

	Describe("ClearDeadLetter", func() {
		Describe("when called for Keep with a wired engine", func() {
			var err error

			BeforeEach(func() {
				mock := connectedKeepProfileMock(profileID)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
					Page:          keepTestPage,
					ListName:      keepTestListName,
					ItemUid:       "uid-1",
				})
			})

			It("should return NotFound — Keep's engine path has no dead-letter ledger surface yet", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "dead_letter_item_not_found"))
			})
		})
	})

	// ---------------------------------------------------------- GetChecklistSubscriptionState (Keep)

	Describe("GetChecklistSubscriptionState", func() {
		Describe("when only the Keep connector is wired and the user has a Keep binding", func() {
			var (
				resp *apiv1.GetChecklistSubscriptionStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedKeepProfileMockWithBinding(profileID, keepTestPage, keepTestListName, keepTestRemoteHandle)
				w := buildKeepWiring(mock, nil)
				server := withKeep(mustNewServer(mock, nil, nil), w)
				resp, err = server.GetChecklistSubscriptionState(ctx, &apiv1.GetChecklistSubscriptionStateRequest{
					Page:     keepTestPage,
					ListName: keepTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report connector_configured = true", func() {
				Expect(resp.GetState().GetConnectorConfigured()).To(BeTrue())
			})

			It("should return a current_subscription with the Keep remote_list_handle", func() {
				Expect(resp.GetState().GetCurrentSubscription()).ToNot(BeNil())
				Expect(resp.GetState().GetCurrentSubscription().GetRemoteListHandle()).To(Equal(keepTestRemoteHandle))
			})

			It("should set GOOGLE_KEEP as the connector_kind on the current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP))
			})
		})
	})
})

var _ = Describe("Server.WithGoogleKeep", func() {
	Describe("when the builder is invoked", func() {
		var server *v1.Server

		BeforeEach(func() {
			mock := &MockPageReaderMutator{}
			w := buildKeepWiring(mock, nil)
			server = withKeep(mustNewServer(nil, nil, nil), w)
		})

		It("should return the server for fluent chaining", func() {
			Expect(server).ToNot(BeNil())
		})
	})

	Describe("Server.WithKeepAuthVerifier", func() {
		Describe("when the builder is invoked", func() {
			var server *v1.Server

			BeforeEach(func() {
				mock := &MockPageReaderMutator{}
				w := buildKeepWiring(mock, nil)
				server = withKeep(mustNewServer(nil, nil, nil), w).
					WithKeepAuthVerifier(stubKeepAuthVerifier{masterToken: "mt"})
			})

			It("should return the server for fluent chaining", func() {
				Expect(server).ToNot(BeNil())
			})
		})
	})

	Describe("KeepAdapter compile-time interface check", func() {
		It("should satisfy connectors.BackendAdapter", func() {
			var _ connectors.BackendAdapter = &googlekeep.KeepAdapter{}
		})
	})
})
