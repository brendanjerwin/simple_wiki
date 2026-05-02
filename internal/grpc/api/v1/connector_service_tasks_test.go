//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:redundant-import-alias
package v1_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- minimal fakes for ConnectorService Tasks-branch tests ----------------

// tasksTestClock returns a fixed time so cursor/persisted timestamps are
// deterministic in test assertions.
type tasksTestClock struct{}

func (tasksTestClock) Now() time.Time {
	return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
}

// silentTasksLogger discards every log line; tests assert on side
// effects rather than log content.
type silentTasksLogger struct{}

func (silentTasksLogger) Info(string, ...any)  {}
func (silentTasksLogger) Error(string, ...any) {}

// fakeTasksClient is a minimal in-memory stand-in for *gateway.TasksClient.
// Records the calls the connector makes so tests can assert delegation
// reached the gateway, but doesn't model multi-page semantics — handler
// tests don't need that.
type fakeTasksClient struct {
	taskListsToReturn  []gateway.TaskList
	listTasksResp      gateway.TasksPage
	listTaskListsErr   error
	insertTaskErr      error
	listsListedForList []string
}

func (f *fakeTasksClient) ListTaskLists(_ context.Context) ([]gateway.TaskList, error) {
	return f.taskListsToReturn, f.listTaskListsErr
}

func (f *fakeTasksClient) ListTasks(_ context.Context, tasklistID string, _ time.Time, _ string) (gateway.TasksPage, error) {
	f.listsListedForList = append(f.listsListedForList, tasklistID)
	return f.listTasksResp, nil
}

func (f *fakeTasksClient) InsertTask(_ context.Context, _ string, _, _ string, _ gateway.TaskStatus, _ time.Time, _ string) (gateway.Task, error) {
	if f.insertTaskErr != nil {
		return gateway.Task{}, f.insertTaskErr
	}
	return gateway.Task{ID: "inserted-id"}, nil
}

func (*fakeTasksClient) PatchTask(_ context.Context, _, _ string, _ gateway.PatchFields, _ string) (gateway.Task, error) {
	return gateway.Task{}, nil
}

func (*fakeTasksClient) DeleteTask(_ context.Context, _, _ string) error {
	return nil
}

// tasksTestProfileEmail must round-trip through ProfileIdentifierFor so
// the per-profile lookup in the SubscriptionStore matches what the
// requireRealUser handler derives.
const tasksTestProfileEmail = "alice@example.com"

// connectedTasksProfileMock builds a MockPageReaderMutator seeded with
// a Tasks-connected profile (email + refresh_token). Mirrors
// connectedProfileMock for Keep but writes the wiki.connectors.google_tasks.*
// subtree.
func connectedTasksProfileMock(profileID wikipage.PageIdentifier) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_tasks": map[string]any{
						"email":         tasksTestProfileEmail,
						"refresh_token": "rt-test-token",
					},
				},
			},
		},
	}
	return mock
}

// connectedTasksProfileMockWithSubscription extends the above with one
// subscription so list/get state tests have something to read back.
func connectedTasksProfileMockWithSubscription(profileID wikipage.PageIdentifier, page, listName, remoteID string) *MockPageReaderMutator {
	mock := &MockPageReaderMutator{}
	mock.WrittenFrontmatterByID = map[string]map[string]any{
		string(profileID): {
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_tasks": map[string]any{
						"email":         tasksTestProfileEmail,
						"refresh_token": "rt-test-token",
						"subscriptions": []any{
							map[string]any{
								"page":              page,
								"list_name":         listName,
								"remote_list_id":    remoteID,
								"remote_list_title": "Tasks list",
							},
						},
					},
				},
			},
		},
	}
	return mock
}

// readyLeaseTable returns a LeaseTable that has already signaled ready
// so Subscribe doesn't block waiting for a boot rebuild.
func readyLeaseTable() *connectors.LeaseTable {
	lt := connectors.NewLeaseTable()
	lt.SignalReady()
	return lt
}

// emptyChecklistReader returns an empty Checklist for any (page, list).
// Sufficient for handler tests that exercise Subscribe's seed-id-map
// path without driving real wiki state.
type emptyChecklistReader struct{}

func (emptyChecklistReader) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	return &apiv1.Checklist{}, nil
}

// stubTasksFactoryReturning returns a TasksClientFactory that always
// returns the given client (and no TokenSource — the connector never
// uses it in these tests).
func stubTasksFactoryReturning(client taskssync.TasksClient) taskssync.TasksClientFactory {
	return func(wikipage.PageIdentifier, string) (taskssync.TasksClient, gateway.TokenSource, error) {
		return client, nil, nil
	}
}

// buildTasksConnector wires a Tasks Connector pointed at the supplied
// page-store and an optional fake TasksClient. Pass nil for a default
// no-op fake.
func buildTasksConnector(mock *MockPageReaderMutator, client taskssync.TasksClient) *taskssync.Connector {
	store, err := taskssync.NewSubscriptionStore(mock)
	Expect(err).ToNot(HaveOccurred())
	if client == nil {
		client = &fakeTasksClient{}
	}
	c, err := taskssync.NewConnector(
		store,
		readyLeaseTable(),
		stubTasksFactoryReturning(client),
		silentTasksLogger{},
		tasksTestClock{},
	)
	Expect(err).ToNot(HaveOccurred())
	c.SetChecklistReader(emptyChecklistReader{})
	return c
}

// --- tests ---------------------------------------------------------------

var _ = Describe("ConnectorService handlers (GOOGLE_TASKS)", func() {
	var (
		ctx       context.Context
		profileID wikipage.PageIdentifier
	)

	const (
		tasksTestPage     = "Shopping"
		tasksTestListName = "groceries"
		tasksTestRemoteID = "tasklist-remote-1"
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(tasksTestProfileEmail)
		Expect(err).ToNot(HaveOccurred())
		ctx = withCallerIdentity(context.Background(), tasksTestProfileEmail)
	})

	// ---------------------------------------------------------- BeginAuth (Tasks)

	Describe("BeginAuth", func() {
		Describe("when called with GOOGLE_TASKS but no Tasks connector wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should return FailedPrecondition pointing the user at operator setup", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "not configured by this wiki's operator"))
			})
		})

		Describe("when called with GOOGLE_TASKS and a connector but no auth-URL builder", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should return FailedPrecondition because the OAuth client config is unset", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "OAuth client not configured"))
			})
		})

		Describe("when called with GOOGLE_TASKS and a working auth-URL builder", func() {
			var (
				resp *apiv1.BeginAuthResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).
					WithGoogleTasksConnector(c).
					WithTasksAuthURLBuilder(stubTasksAuthURLBuilder{
						url:   "https://accounts.google.com/auth?state=abc",
						state: "abc",
					})
				resp, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					AccountEmail:  tasksTestProfileEmail,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the URL the builder produced", func() {
				Expect(resp.GetAuthorizationUrl()).To(Equal("https://accounts.google.com/auth?state=abc"))
			})

			It("should return the state token the builder issued", func() {
				Expect(resp.GetState()).To(Equal("abc"))
			})
		})

		Describe("when called without an authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).
					WithGoogleTasksConnector(c).
					WithTasksAuthURLBuilder(stubTasksAuthURLBuilder{url: "x", state: "y"})
				_, err = server.BeginAuth(context.Background(), &apiv1.BeginAuthRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})
	})

	// ---------------------------------------------------------- CompleteAuth (Tasks)

	Describe("CompleteAuth", func() {
		Describe("when called with GOOGLE_TASKS", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.CompleteAuth(ctx, &apiv1.CompleteAuthRequest{
					ConnectorKind:     apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					AuthorizationCode: "code",
					State:             "state",
				})
			})

			It("should return FailedPrecondition steering the caller to the OAuth callback flow", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "/oauth/google/callback"))
			})
		})
	})

	// ---------------------------------------------------------- Disconnect (Tasks)

	Describe("Disconnect", func() {
		Describe("when no Tasks connector is wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
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
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.Disconnect(ctx, &apiv1.DisconnectRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a not-configured state", func() {
				Expect(resp.GetState().GetConfigured()).To(BeFalse())
			})

			It("should echo the GOOGLE_TASKS connector_kind on the state", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS))
			})
		})
	})

	// ---------------------------------------------------------- GetState (Tasks)

	Describe("GetState", func() {
		Describe("when no Tasks connector is wired", func() {
			var err error

			BeforeEach(func() {
				server := mustNewServer(nil, nil, nil)
				_, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
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
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.GetState(ctx, &apiv1.GetStateRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report the connector as configured", func() {
				Expect(resp.GetState().GetConfigured()).To(BeTrue())
			})

			It("should return the email", func() {
				Expect(resp.GetState().GetEmail()).To(Equal(tasksTestProfileEmail))
			})

			It("should set GOOGLE_TASKS as the connector_kind", func() {
				Expect(resp.GetState().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS))
			})
		})
	})

	// ---------------------------------------------------------- ListMySubscriptions (Tasks)

	Describe("ListMySubscriptions", func() {
		Describe("when the user has one subscription", func() {
			var (
				resp *apiv1.ListMySubscriptionsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMockWithSubscription(profileID, tasksTestPage, tasksTestListName, tasksTestRemoteID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.ListMySubscriptions(ctx, &apiv1.ListMySubscriptionsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
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
				Expect(s.GetPage()).To(Equal(tasksTestPage))
				Expect(s.GetListName()).To(Equal(tasksTestListName))
			})

			It("should populate the remote_list_handle from the Tasks tasklist id", func() {
				Expect(resp.GetSubscriptions()[0].GetRemoteListHandle()).To(Equal(tasksTestRemoteID))
			})

			It("should set GOOGLE_TASKS as the connector_kind on the subscription", func() {
				Expect(resp.GetSubscriptions()[0].GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS))
			})
		})
	})

	// ---------------------------------------------------------- ListRemoteLists (Tasks)

	Describe("ListRemoteLists", func() {
		Describe("when the gateway returns two tasklists", func() {
			var (
				resp *apiv1.ListRemoteListsResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				client := &fakeTasksClient{
					taskListsToReturn: []gateway.TaskList{
						{ID: "list-1", Title: "Groceries"},
						{ID: "list-2", Title: "Errands"},
					},
				}
				c := buildTasksConnector(mock, client)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return both tasklists", func() {
				Expect(resp.GetLists()).To(HaveLen(2))
			})

			It("should map ID into remote_list_handle", func() {
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
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.ListRemoteLists(ctx, &apiv1.ListRemoteListsRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				})
			})

			It("should return FailedPrecondition tasks_connector_not_configured", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "tasks_connector_not_configured"))
			})
		})
	})

	// ---------------------------------------------------------- Subscribe (Tasks)

	Describe("Subscribe", func() {
		Describe("when called with empty remote_list_handle", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:             tasksTestPage,
					ListName:         tasksTestListName,
					RemoteListHandle: "",
				})
			})

			It("should return InvalidArgument because Tasks Subscribe requires picking an existing list", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "remote_list_handle is required"))
			})
		})

		Describe("when called with empty page", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:             "",
					ListName:         tasksTestListName,
					RemoteListHandle: tasksTestRemoteID,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page and list_name"))
			})
		})

		Describe("when subscribing to a remote tasklist with no subtasks", func() {
			var (
				resp *apiv1.SubscribeResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				client := &fakeTasksClient{
					taskListsToReturn: []gateway.TaskList{{ID: tasksTestRemoteID, Title: "Groceries"}},
				}
				c := buildTasksConnector(mock, client)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
					ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:             tasksTestPage,
					ListName:         tasksTestListName,
					RemoteListHandle: tasksTestRemoteID,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the persisted subscription", func() {
				Expect(resp.GetSubscription()).ToNot(BeNil())
			})

			It("should populate page", func() {
				Expect(resp.GetSubscription().GetPage()).To(Equal(tasksTestPage))
			})

			It("should populate list_name", func() {
				Expect(resp.GetSubscription().GetListName()).To(Equal(tasksTestListName))
			})

			It("should populate remote_list_handle", func() {
				Expect(resp.GetSubscription().GetRemoteListHandle()).To(Equal(tasksTestRemoteID))
			})
		})
	})

	// ---------------------------------------------------------- Unsubscribe (Tasks)

	Describe("Unsubscribe", func() {
		Describe("when unsubscribing an existing subscription", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMockWithSubscription(profileID, tasksTestPage, tasksTestListName, tasksTestRemoteID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:          tasksTestPage,
					ListName:      tasksTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("when unsubscribing a missing subscription", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.Unsubscribe(ctx, &apiv1.UnsubscribeRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:          "no-such-page",
					ListName:      "no-such-list",
				})
			})

			It("should return NotFound", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "subscription_not_found"))
			})
		})
	})

	// ---------------------------------------------------------- ListDeadLetters / ClearDeadLetter (Tasks)

	Describe("ListDeadLetters", func() {
		Describe("when called for Tasks", func() {
			var (
				resp *apiv1.ListDeadLettersResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:          tasksTestPage,
					ListName:      tasksTestListName,
				})
			})

			It("should not error — Tasks has no dead-letter ledger yet", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty list", func() {
				Expect(resp.GetItems()).To(BeEmpty())
			})
		})
	})

	Describe("ClearDeadLetter", func() {
		Describe("when called for Tasks", func() {
			var err error

			BeforeEach(func() {
				mock := connectedTasksProfileMock(profileID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
					Page:          tasksTestPage,
					ListName:      tasksTestListName,
					ItemUid:       "uid-1",
				})
			})

			It("should return NotFound — Tasks has no dead-letter ledger yet", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "dead_letter_item_not_found"))
			})
		})
	})

	// ---------------------------------------------------------- GetChecklistSubscriptionState

	Describe("GetChecklistSubscriptionState", func() {
		Describe("when only the Tasks connector is wired and the user has a Tasks subscription", func() {
			var (
				resp *apiv1.GetChecklistSubscriptionStateResponse
				err  error
			)

			BeforeEach(func() {
				mock := connectedTasksProfileMockWithSubscription(profileID, tasksTestPage, tasksTestListName, tasksTestRemoteID)
				c := buildTasksConnector(mock, nil)
				server := mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
				resp, err = server.GetChecklistSubscriptionState(ctx, &apiv1.GetChecklistSubscriptionStateRequest{
					Page:     tasksTestPage,
					ListName: tasksTestListName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should report connector_configured = true", func() {
				Expect(resp.GetState().GetConnectorConfigured()).To(BeTrue())
			})

			It("should return a current_subscription with the Tasks remote_list_handle", func() {
				Expect(resp.GetState().GetCurrentSubscription()).ToNot(BeNil())
				Expect(resp.GetState().GetCurrentSubscription().GetRemoteListHandle()).To(Equal(tasksTestRemoteID))
			})

			It("should set GOOGLE_TASKS as the connector_kind on the current_subscription", func() {
				Expect(resp.GetState().GetCurrentSubscription().GetConnectorKind()).To(Equal(apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS))
			})
		})
	})
})

var _ = Describe("Server.WithGoogleTasksConnector", func() {
	Describe("when the builder is invoked", func() {
		var server *v1.Server

		BeforeEach(func() {
			mock := &MockPageReaderMutator{}
			c := buildTasksConnector(mock, nil)
			server = mustNewServer(nil, nil, nil).WithGoogleTasksConnector(c)
		})

		It("should return the server for fluent chaining", func() {
			Expect(server).ToNot(BeNil())
		})
	})
})

// stubTasksAuthURLBuilder is a deterministic AuthURLBuilder for tests.
type stubTasksAuthURLBuilder struct {
	url   string
	state string
	err   error
}

func (s stubTasksAuthURLBuilder) BuildAuthURL(_ context.Context, _, _ string) (authURL string, stateToken string, err error) {
	if s.err != nil {
		return "", "", s.err
	}
	return s.url, s.state, nil
}

