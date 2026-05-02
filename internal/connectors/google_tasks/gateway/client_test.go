//revive:disable:dot-imports
package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
)

// staticTokenSource is a TokenSource for the client tests. Records
// the number of AccessToken and Invalidate calls.
type staticTokenSource struct {
	tokens          []string
	tokensIndex     int32
	invalidateCalls int32
}

func newStaticTokenSource(tokens ...string) *staticTokenSource {
	return &staticTokenSource{tokens: tokens}
}

func (s *staticTokenSource) AccessToken(_ context.Context) (string, error) {
	idx := atomic.LoadInt32(&s.tokensIndex)
	if int(idx) >= len(s.tokens) {
		return s.tokens[len(s.tokens)-1], nil
	}
	return s.tokens[idx], nil
}

func (s *staticTokenSource) Invalidate() {
	atomic.AddInt32(&s.invalidateCalls, 1)
	atomic.AddInt32(&s.tokensIndex, 1)
}

func (s *staticTokenSource) InvalidateCount() int { return int(atomic.LoadInt32(&s.invalidateCalls)) }

// recordedRequest is a captured HTTP request the fake server has seen.
type recordedRequest struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

// fakeTasksAPI is the test harness for the Tasks REST endpoints.
type fakeTasksAPI struct {
	server   *httptest.Server
	requests []recordedRequest
}

func newFakeTasksAPI(handler http.HandlerFunc) *fakeTasksAPI {
	f := &fakeTasksAPI{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.requests = append(f.requests, recordedRequest{
			method:  r.Method,
			path:    r.URL.Path,
			query:   r.URL.RawQuery,
			body:    string(body),
			headers: r.Header.Clone(),
		})
		handler(w, r)
	}))
	return f
}

func (f *fakeTasksAPI) URL() string { return f.server.URL + "/" }
func (f *fakeTasksAPI) Close()      { f.server.Close() }

var _ = Describe("TasksClient constructor", func() {
	When("httpClient is nil", func() {
		It("should error", func() {
			_, err := gateway.NewTasksClient(nil, "https://x/", newStaticTokenSource("t"))
			Expect(err).To(MatchError(ContainSubstring("httpClient")))
		})
	})

	When("baseURL does not end with /", func() {
		It("should error", func() {
			_, err := gateway.NewTasksClient(http.DefaultClient, "https://x", newStaticTokenSource("t"))
			Expect(err).To(MatchError(ContainSubstring("must end with")))
		})
	})

	When("tokenSource is nil", func() {
		It("should error", func() {
			_, err := gateway.NewTasksClient(http.DefaultClient, "https://x/", nil)
			Expect(err).To(MatchError(ContainSubstring("tokenSource")))
		})
	})
})

var _ = Describe("TasksClient.ListTaskLists", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ts     *staticTokenSource
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		ts = newStaticTokenSource("ya29.first")
	})

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the server returns a single page", func() {
		var (
			lists []gateway.TaskList
			err   error
		)

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{
					"kind":"tasks#taskLists",
					"items":[
						{"id":"list-A","etag":"etag-A","title":"Groceries","updated":"2026-04-25T17:14:00.000Z"},
						{"id":"list-B","etag":"etag-B","title":"Errands","updated":"2026-04-25T17:15:00.000Z"}
					]
				}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), ts)
			lists, err = client.ListTaskLists(ctx)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return both tasklists", func() {
			Expect(lists).To(HaveLen(2))
			Expect(lists[0].ID).To(Equal("list-A"))
			Expect(lists[0].Title).To(Equal("Groceries"))
			Expect(lists[1].ID).To(Equal("list-B"))
		})

		It("should send Authorization: Bearer ya29.first", func() {
			Expect(api.requests[0].headers.Get("Authorization")).To(Equal("Bearer ya29.first"))
		})

		It("should target the lists endpoint", func() {
			Expect(api.requests[0].path).To(Equal("/users/@me/lists"))
		})
	})

	When("the server returns multiple pages", func() {
		var (
			lists []gateway.TaskList
			err   error
		)

		BeforeEach(func() {
			pageNum := 0
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				pageNum++
				if pageNum == 1 {
					_, _ = io.WriteString(w, `{
						"items":[{"id":"list-A","title":"A"}],
						"nextPageToken":"page-2"
					}`)
				} else {
					_, _ = io.WriteString(w, `{"items":[{"id":"list-B","title":"B"}]}`)
				}
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), ts)
			lists, err = client.ListTaskLists(ctx)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should consume both pages", func() {
			Expect(lists).To(HaveLen(2))
			Expect(api.requests).To(HaveLen(2))
			Expect(api.requests[1].query).To(ContainSubstring("pageToken=page-2"))
		})
	})
})

var _ = Describe("TasksClient.ListTasks", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ts     *staticTokenSource
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		ts = newStaticTokenSource("tok")
	})

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the request includes an updatedMin and pageToken", func() {
		var (
			page gateway.TasksPage
			err  error
		)

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// The notes field embeds the wiki uid marker, which begins
				// with U+200B (zero-width space). Use the ​ escape
				// (split out of the raw string via concatenation) so the
				// source form is explicit and staticcheck ST1018 stays
				// quiet — the runtime byte sequence is unchanged.
				_, _ = io.WriteString(w, `{
					"items":[
						{
							"id":"task-1",
							"etag":"etag-1",
							"title":"Buy milk",
							"notes":"`+"\u200b"+`— wiki:uid=01H...",
							"status":"needsAction",
							"position":"00000000000000000001",
							"updated":"2026-04-25T17:14:00.000Z"
						},
						{
							"id":"task-2",
							"etag":"etag-2",
							"title":"Eggs",
							"status":"completed",
							"completed":"2026-04-25T17:15:00.000Z",
							"updated":"2026-04-25T17:15:01.000Z"
						}
					],
					"nextPageToken":"next"
				}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), ts)
			page, err = client.ListTasks(ctx, "list-A", time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC), "ptok")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return all tasks", func() {
			Expect(page.Tasks).To(HaveLen(2))
			Expect(page.Tasks[0].Title).To(Equal("Buy milk"))
			Expect(page.Tasks[1].Status).To(Equal(gateway.TaskStatusCompleted))
		})

		It("should propagate the next page token", func() {
			Expect(page.NextPageToken).To(Equal("next"))
		})

		It("should target the tasks endpoint with the tasklist id path-escaped", func() {
			Expect(api.requests[0].path).To(Equal("/lists/list-A/tasks"))
		})

		It("should set showHidden=true and showDeleted=true", func() {
			Expect(api.requests[0].query).To(ContainSubstring("showHidden=true"))
			Expect(api.requests[0].query).To(ContainSubstring("showDeleted=true"))
		})

		It("should send the updatedMin", func() {
			Expect(api.requests[0].query).To(ContainSubstring("updatedMin="))
		})

		It("should send the pageToken", func() {
			Expect(api.requests[0].query).To(ContainSubstring("pageToken=ptok"))
		})
	})

	When("the tasklist id is empty", func() {
		It("should error", func() {
			client, _ = gateway.NewTasksClient(http.DefaultClient, "https://x/", ts)
			_, err := client.ListTasks(ctx, "", time.Time{}, "")
			Expect(err).To(MatchError(ContainSubstring("tasklistID")))
		})
	})
})

var _ = Describe("TasksClient.CreateTaskList", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ctx    context.Context
	)

	BeforeEach(func() { ctx = context.Background() })

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the server accepts the create", func() {
		var (
			tasklist gateway.TaskList
			err      error
			body     map[string]any
		)

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.Unmarshal([]byte(api.requests[len(api.requests)-1].body), &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{
					"kind":"tasks#taskList",
					"id":"new-list-id",
					"etag":"etag-new",
					"title":"Groceries",
					"updated":"2026-04-25T17:14:00.000Z"
				}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("tok"))
			tasklist, err = client.CreateTaskList(ctx, "Groceries")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the server-assigned tasklist id", func() {
			Expect(tasklist.ID).To(Equal("new-list-id"))
		})

		It("should return the title", func() {
			Expect(tasklist.Title).To(Equal("Groceries"))
		})

		It("should POST to the lists endpoint", func() {
			Expect(api.requests[0].method).To(Equal(http.MethodPost))
			Expect(api.requests[0].path).To(Equal("/users/@me/lists"))
		})

		It("should send the title in the JSON body", func() {
			Expect(body["title"]).To(Equal("Groceries"))
		})
	})

	When("title is empty", func() {
		It("should error before sending a request", func() {
			client, _ = gateway.NewTasksClient(http.DefaultClient, "https://x/", newStaticTokenSource("t"))
			_, err := client.CreateTaskList(ctx, "")
			Expect(err).To(MatchError(ContainSubstring("title")))
		})
	})

	When("the server returns 401 twice in a row", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":{"code":401}}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("a", "b"))
			_, err = client.CreateTaskList(ctx, "Groceries")
		})

		It("should return ErrAuthRevoked", func() {
			Expect(err).To(MatchError(gateway.ErrAuthRevoked))
		})
	})
})

var _ = Describe("TasksClient.InsertTask", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ctx    context.Context
	)

	BeforeEach(func() { ctx = context.Background() })

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the server accepts the insert", func() {
		var (
			task gateway.Task
			err  error
			body map[string]any
		)

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, r *http.Request) {
				_ = json.Unmarshal([]byte(api.requests[len(api.requests)-1].body), &body)
				_ = r
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{
					"id":"task-new",
					"etag":"etag-new",
					"title":"Buy milk",
					"notes":"…trailer",
					"status":"needsAction",
					"position":"00000000000000000005",
					"updated":"2026-04-25T17:14:00.000Z"
				}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("tok"))
			task, err = client.InsertTask(ctx, "list-A", "Buy milk", "…trailer",
				gateway.TaskStatusNeedsAction,
				time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
				"")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the server-assigned task id", func() {
			Expect(task.ID).To(Equal("task-new"))
		})

		It("should return the etag", func() {
			Expect(task.Etag).To(Equal("etag-new"))
		})

		It("should send the title in the JSON body", func() {
			Expect(body["title"]).To(Equal("Buy milk"))
		})

		It("should send the notes in the JSON body", func() {
			Expect(body["notes"]).To(Equal("…trailer"))
		})

		It("should send the due date in the wire format", func() {
			Expect(body["due"]).To(Equal("2026-05-01T00:00:00.000Z"))
		})
	})

	When("title is empty", func() {
		It("should error before sending a request", func() {
			client, _ = gateway.NewTasksClient(http.DefaultClient, "https://x/", newStaticTokenSource("t"))
			_, err := client.InsertTask(ctx, "list-A", "", "", "", time.Time{}, "")
			Expect(err).To(MatchError(ContainSubstring("title")))
		})
	})

	When("the server returns 401 once and then 200", func() {
		var (
			task gateway.Task
			err  error
			ts   *staticTokenSource
		)

		BeforeEach(func() {
			ts = newStaticTokenSource("stale", "fresh")
			callCount := 0
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				callCount++
				if callCount == 1 {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = io.WriteString(w, `{"error":{"code":401,"message":"invalid auth"}}`)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"task-new","etag":"e","title":"x","status":"needsAction","updated":"2026-04-25T17:14:00.000Z"}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), ts)
			task, err = client.InsertTask(ctx, "list-A", "x", "", "", time.Time{}, "")
		})

		It("should retry and succeed", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(task.ID).To(Equal("task-new"))
		})

		It("should call Invalidate on the token source", func() {
			Expect(ts.InvalidateCount()).To(Equal(1))
		})

		It("should use the fresh token on the retry", func() {
			Expect(api.requests).To(HaveLen(2))
			Expect(api.requests[1].headers.Get("Authorization")).To(Equal("Bearer fresh"))
		})
	})

	When("the server returns 401 twice in a row", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":{"code":401}}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("a", "b"))
			_, err = client.InsertTask(ctx, "list-A", "x", "", "", time.Time{}, "")
		})

		It("should return ErrAuthRevoked", func() {
			Expect(err).To(MatchError(gateway.ErrAuthRevoked))
		})
	})
})

var _ = Describe("TasksClient.PatchTask", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ctx    context.Context
	)

	BeforeEach(func() { ctx = context.Background() })

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the patch sets multiple fields with a fresh etag", func() {
		var (
			task gateway.Task
			err  error
			body map[string]any
		)

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.Unmarshal([]byte(api.requests[len(api.requests)-1].body), &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"task-1","etag":"etag-after","title":"new","status":"completed","updated":"2026-04-25T17:14:00.000Z"}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			task, err = client.PatchTask(ctx, "list-A", "task-1", gateway.PatchFields{
				SetTitle:  true,
				Title:     "new",
				SetStatus: true,
				Status:    gateway.TaskStatusCompleted,
			}, "etag-before")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the updated task", func() {
			Expect(task.Title).To(Equal("new"))
			Expect(task.Status).To(Equal(gateway.TaskStatusCompleted))
		})

		It("should send If-Match with the supplied etag", func() {
			Expect(api.requests[0].headers.Get("If-Match")).To(Equal("etag-before"))
		})

		It("should use HTTP PATCH", func() {
			Expect(api.requests[0].method).To(Equal(http.MethodPatch))
		})

		It("should send only the requested fields", func() {
			Expect(body).To(HaveKeyWithValue("title", "new"))
			Expect(body).To(HaveKeyWithValue("status", "completed"))
			Expect(body).NotTo(HaveKey("notes"))
		})
	})

	When("clearing the due date", func() {
		var body map[string]any

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.Unmarshal([]byte(api.requests[len(api.requests)-1].body), &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"task-1","etag":"e","title":"x","status":"needsAction","updated":"2026-04-25T17:14:00.000Z"}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			_, err := client.PatchTask(ctx, "list-A", "task-1", gateway.PatchFields{SetDue: true, Due: time.Time{}}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send due:null", func() {
			Expect(body).To(HaveKey("due"))
			Expect(body["due"]).To(BeNil())
		})
	})

	When("the patch has no fields set", func() {
		var err error

		BeforeEach(func() {
			client, _ = gateway.NewTasksClient(http.DefaultClient, "https://x/", newStaticTokenSource("t"))
			_, err = client.PatchTask(ctx, "list-A", "task-1", gateway.PatchFields{}, "")
		})

		It("should error", func() {
			Expect(err).To(MatchError(ContainSubstring("at least one field")))
		})
	})

	When("the server returns 412", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusPreconditionFailed)
				_, _ = io.WriteString(w, `{"error":{"code":412,"message":"etag mismatch"}}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			_, err = client.PatchTask(ctx, "list-A", "task-1", gateway.PatchFields{SetTitle: true, Title: "x"}, "stale")
		})

		It("should return ErrPreconditionFailed", func() {
			Expect(err).To(MatchError(gateway.ErrPreconditionFailed))
		})
	})
})

var _ = Describe("TasksClient.DeleteTask", func() {
	var (
		api    *fakeTasksAPI
		client *gateway.TasksClient
		ctx    context.Context
	)

	BeforeEach(func() { ctx = context.Background() })

	AfterEach(func() {
		if api != nil {
			api.Close()
		}
	})

	When("the server returns 204", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			err = client.DeleteTask(ctx, "list-A", "task-1")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use HTTP DELETE", func() {
			Expect(api.requests[0].method).To(Equal(http.MethodDelete))
		})
	})

	When("the server returns 404", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":{"code":404,"message":"already gone"}}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			err = client.DeleteTask(ctx, "list-A", "task-1")
		})

		It("should treat 404 as success (Google delete is idempotent)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the server returns 429", func() {
		var err error

		BeforeEach(func() {
			api = newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":{"code":429}}`)
			})
			client, _ = gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			err = client.DeleteTask(ctx, "list-A", "task-1")
		})

		It("should return ErrRateLimited", func() {
			Expect(err).To(MatchError(gateway.ErrRateLimited))
		})
	})
})

var _ = Describe("Task decode", func() {
	When("the wire payload includes an unknown status value", func() {
		var (
			err error
		)

		BeforeEach(func() {
			api := newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"items":[{"id":"x","status":"bogus"}]}`)
			})
			defer api.Close()
			client, _ := gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			_, err = client.ListTasks(context.Background(), "list-A", time.Time{}, "")
		})

		It("should return ErrProtocolDrift", func() {
			Expect(err).To(MatchError(gateway.ErrProtocolDrift))
		})
	})

	When("the wire payload uses RFC3339Nano timestamps", func() {
		var (
			page gateway.TasksPage
			err  error
		)

		BeforeEach(func() {
			api := newFakeTasksAPI(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"items":[{"id":"x","status":"needsAction","title":"x","updated":"2026-04-25T17:14:00.123456789Z"}]}`)
			})
			defer api.Close()
			client, _ := gateway.NewTasksClient(api.server.Client(), api.URL(), newStaticTokenSource("t"))
			page, err = client.ListTasks(context.Background(), "list-A", time.Time{}, "")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse the nano-second timestamp", func() {
			Expect(page.Tasks[0].Updated.Nanosecond()).NotTo(BeZero())
		})
	})
})
