//revive:disable:dot-imports
//revive:disable:add-constant
package v1_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errMapperTasksClient is a TasksClient that injects a specific error
// on the first ListTasks call (before Subscribe inspects the list for subtasks).
type errMapperTasksClient struct {
	listTasksErr   error
	taskListsToReturn []tasksgateway.TaskList
	tasksToReturn    []tasksgateway.Task
}

func (f *errMapperTasksClient) ListTaskLists(_ context.Context) ([]tasksgateway.TaskList, error) {
	return f.taskListsToReturn, nil
}

func (*errMapperTasksClient) CreateTaskList(_ context.Context, _ string) (tasksgateway.TaskList, error) {
	return tasksgateway.TaskList{}, nil
}

func (f *errMapperTasksClient) ListTasks(_ context.Context, _ string, _ time.Time, _ string) (tasksgateway.TasksPage, error) {
	if f.listTasksErr != nil {
		return tasksgateway.TasksPage{}, f.listTasksErr
	}
	return tasksgateway.TasksPage{Tasks: f.tasksToReturn}, nil
}

func (*errMapperTasksClient) InsertTask(_ context.Context, _, _, _ string, _ tasksgateway.TaskStatus, _ time.Time, _ string) (tasksgateway.Task, error) {
	return tasksgateway.Task{}, nil
}

func (*errMapperTasksClient) PatchTask(_ context.Context, _, _ string, _ tasksgateway.PatchFields, _ string) (tasksgateway.Task, error) {
	return tasksgateway.Task{}, nil
}

func (*errMapperTasksClient) DeleteTask(_ context.Context, _, _ string) error {
	return nil
}

// buildErrMapperTasksConnector wires a Tasks connector with the supplied client.
func buildErrMapperTasksConnector(mock *MockPageReaderMutator, client taskssync.TasksClient) *taskssync.Connector {
	store, err := taskssync.NewSubscriptionStore(mock)
	Expect(err).ToNot(HaveOccurred())
	lt := connectors.NewLeaseTable()
	lt.SignalReady()
	c, nerr := taskssync.NewConnector(
		store,
		lt,
		func(_ wikipage.PageIdentifier, _ string) (taskssync.TasksClient, tasksgateway.TokenSource, error) {
			return client, nil, nil
		},
		silentTasksLogger{},
		tasksTestClock{},
	)
	Expect(nerr).ToNot(HaveOccurred())
	c.SetChecklistReader(emptyChecklistReader{})
	return c
}

var _ = Describe("mapTasksConnectorErr coverage", func() {
	var (
		ctx       context.Context
		profileID wikipage.PageIdentifier
		server    *v1.Server
	)

	const (
		errMapPage     = "Erroring"
		errMapListName = "err-list"
		errMapRemoteID = "tasklist-err-1"
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(tasksTestProfileEmail)
		Expect(err).ToNot(HaveOccurred())
		ctx = withCallerIdentity(context.Background(), tasksTestProfileEmail)
	})

	// ------------------------------------------------------------------ ErrConnectorNotConfigured

	Describe("when profile has no refresh token (ErrConnectorNotConfigured)", func() {
		var err error

		BeforeEach(func() {
			// A profile page with NO refresh_token → IsConfigured() == false.
			mock := &MockPageReaderMutator{}
			mock.WrittenFrontmatterByID = map[string]map[string]any{
				string(profileID): {
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"email": tasksTestProfileEmail,
								// no refresh_token
							},
						},
					},
				},
			}
			c := buildErrMapperTasksConnector(mock, &errMapperTasksClient{})
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return FailedPrecondition tasks_connector_not_configured", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "tasks_connector_not_configured"))
		})
	})

	// ------------------------------------------------------------------ ErrAlreadySubscribedForChecklist

	Describe("when subscribing to a checklist that already exists in the store (ErrAlreadySubscribedForChecklist)", func() {
		var err error

		BeforeEach(func() {
			// Pre-load the subscription in the store so AddSubscription finds it
			// immediately.  The lease table is empty, so the lease attempt itself
			// succeeds; the error comes from the store's duplicate-detection logic.
			mock := connectedTasksProfileMockWithSubscription(profileID, errMapPage, errMapListName, errMapRemoteID)
			c := buildErrMapperTasksConnector(mock, &errMapperTasksClient{})
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)

			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: "tasklist-err-2",
			})
		})

		It("should return AlreadyExists already_subscribed_for_checklist", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.AlreadyExists, "already_subscribed"))
		})
	})

	// ------------------------------------------------------------------ ErrTasksListHasSubtasks

	Describe("when the remote tasklist has subtasks (ErrTasksListHasSubtasks)", func() {
		var err error

		BeforeEach(func() {
			// Return tasks where one task has a non-empty Parent (= subtask).
			client := &errMapperTasksClient{
				tasksToReturn: []tasksgateway.Task{
					{ID: "parent-1", Title: "Buy groceries"},
					{ID: "child-1", Title: "Milk", Parent: "parent-1"},
				},
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return FailedPrecondition tasks_list_has_subtasks", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "tasks_list_has_subtasks"))
		})
	})

	// ------------------------------------------------------------------ ErrAuthRevoked (gateway)

	Describe("when the tasks gateway returns ErrAuthRevoked during Subscribe (auth_revoked)", func() {
		var err error

		BeforeEach(func() {
			client := &errMapperTasksClient{
				listTasksErr: tasksgateway.ErrAuthRevoked,
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return Unauthenticated auth_revoked", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unauthenticated, "auth_revoked"))
		})
	})

	// ------------------------------------------------------------------ ErrRateLimited (gateway)

	Describe("when the tasks gateway returns ErrRateLimited during Subscribe (rate_limited)", func() {
		var err error

		BeforeEach(func() {
			client := &errMapperTasksClient{
				listTasksErr: tasksgateway.ErrRateLimited,
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return ResourceExhausted rate_limited", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.ResourceExhausted, "rate_limited"))
		})
	})

	// ------------------------------------------------------------------ ErrServiceDisabled (gateway)

	Describe("when the tasks gateway returns ErrServiceDisabled during Subscribe (tasks_api_not_enabled)", func() {
		var err error

		BeforeEach(func() {
			// Simulate the activation-URL embedded message the gateway
			// produces when Google's 403 body advertises SERVICE_DISABLED.
			wrapped := errors.New("https://console.developers.google.com/apis/api/tasks.googleapis.com/overview?project=703961900896")
			injected := errors.Join(tasksgateway.ErrServiceDisabled, wrapped)
			client := &errMapperTasksClient{
				listTasksErr: injected,
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return FailedPrecondition tasks_api_not_enabled", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "tasks_api_not_enabled"))
		})

		It("should preserve the activationUrl in the message", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "console.developers.google.com"))
		})
	})

	// ------------------------------------------------------------------ ErrPermissionDenied (gateway)

	Describe("when the tasks gateway returns ErrPermissionDenied during Subscribe (permission_denied)", func() {
		var err error

		BeforeEach(func() {
			client := &errMapperTasksClient{
				listTasksErr: tasksgateway.ErrPermissionDenied,
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return PermissionDenied permission_denied", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "permission_denied"))
		})
	})

	// ------------------------------------------------------------------ default branch

	Describe("when the tasks gateway returns an unrecognised error during Subscribe (default branch)", func() {
		var err error

		BeforeEach(func() {
			client := &errMapperTasksClient{
				listTasksErr: errors.New("totally unexpected tasks error"),
			}
			mock := connectedTasksProfileMock(profileID)
			c := buildErrMapperTasksConnector(mock, client)
			server = mustNewServer(mock, nil, nil).WithGoogleTasksConnector(c)
			_, err = server.Subscribe(ctx, &apiv1.SubscribeRequest{
				ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
				Page:             errMapPage,
				ListName:         errMapListName,
				RemoteListHandle: errMapRemoteID,
			})
		})

		It("should return Internal tasks connector", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "tasks connector"))
		})
	})
})

// ------------------------------------------------------------------ errUnsupportedConnectorKind

var _ = Describe("errUnsupportedConnectorKind", func() {
	Describe("when BeginAuth is called with an unknown connector_kind", func() {
		var err error

		BeforeEach(func() {
			ctx := withCallerIdentity(context.Background(), tasksTestProfileEmail)
			server := mustNewServer(nil, nil, nil)
			_, err = server.BeginAuth(ctx, &apiv1.BeginAuthRequest{
				// Use an integer value that maps to no defined ConnectorKind.
				ConnectorKind: apiv1.ConnectorKind(999),
			})
		})

		It("should return Unimplemented unsupported connector_kind", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unimplemented, "unsupported connector_kind"))
		})
	})
})
