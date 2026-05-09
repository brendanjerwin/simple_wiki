//revive:disable:dot-imports
package google_tasks_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestGoogleTasksAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "google_tasks adapter Suite")
}

// --- fakes -----------------------------------------------------------

// fakeTasksClient is the in-memory stand-in for *gateway.TasksClient.
type fakeTasksClient struct {
	mu sync.Mutex

	// listsForList: tasklistID → ordered slice of pages to return on
	// successive ListTasks calls. Drained as called.
	listsForList map[string][]gateway.TasksPage
	// listAllForList: tasklistID → flat task slice for every
	// non-paginated ListTasks call.
	listAllForList map[string][]gateway.Task

	listTasksErr      error
	insertErr         error
	patchErr          error
	deleteErr         error
	listTaskListsErr  error
	createTaskListErr error

	// taskLists is what ListTaskLists returns.
	taskLists []gateway.TaskList

	inserted         []insertedCall
	patched          []patchedCall
	deleted          []deletedCall
	createdTaskLists []string

	nextInsertID int
}

type insertedCall struct {
	TasklistID string
	Title      string
	Notes      string
	Status     gateway.TaskStatus
	Due        time.Time
	Parent     string
}

type patchedCall struct {
	TasklistID string
	TaskID     string
	Fields     gateway.PatchFields
	Etag       string
}

type deletedCall struct {
	TasklistID string
	TaskID     string
}

func newFakeTasksClient() *fakeTasksClient {
	return &fakeTasksClient{
		listsForList:   map[string][]gateway.TasksPage{},
		listAllForList: map[string][]gateway.Task{},
	}
}

func (f *fakeTasksClient) ListTaskLists(_ context.Context) ([]gateway.TaskList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listTaskListsErr != nil {
		return nil, f.listTaskListsErr
	}
	return append([]gateway.TaskList(nil), f.taskLists...), nil
}

func (f *fakeTasksClient) CreateTaskList(_ context.Context, title string) (gateway.TaskList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createTaskListErr != nil {
		return gateway.TaskList{}, f.createTaskListErr
	}
	f.createdTaskLists = append(f.createdTaskLists, title)
	id := fmt.Sprintf("created-list-%d", len(f.createdTaskLists))
	tl := gateway.TaskList{ID: id, Etag: "etag-" + id, Title: title}
	f.taskLists = append(f.taskLists, tl)
	return tl, nil
}

func (f *fakeTasksClient) ListTasks(_ context.Context, tasklistID string, updatedMin time.Time, pageToken string) (gateway.TasksPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listTasksErr != nil {
		return gateway.TasksPage{}, f.listTasksErr
	}
	if pages, ok := f.listsForList[tasklistID]; ok && len(pages) > 0 {
		page := pages[0]
		f.listsForList[tasklistID] = pages[1:]
		return page, nil
	}
	if pageToken != "" {
		return gateway.TasksPage{}, nil
	}
	if all, ok := f.listAllForList[tasklistID]; ok {
		if updatedMin.IsZero() {
			return gateway.TasksPage{Tasks: all}, nil
		}
		out := make([]gateway.Task, 0, len(all))
		for _, t := range all {
			if t.Updated.After(updatedMin) || t.Updated.Equal(updatedMin) {
				out = append(out, t)
			}
		}
		return gateway.TasksPage{Tasks: out}, nil
	}
	return gateway.TasksPage{}, nil
}

func (f *fakeTasksClient) InsertTask(_ context.Context, tasklistID, title, notes string, status gateway.TaskStatus, due time.Time, parent string) (gateway.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return gateway.Task{}, f.insertErr
	}
	f.nextInsertID++
	id := fmt.Sprintf("inserted-%d", f.nextInsertID)
	f.inserted = append(f.inserted, insertedCall{
		TasklistID: tasklistID, Title: title, Notes: notes, Status: status, Due: due, Parent: parent,
	})
	return gateway.Task{
		ID:    id,
		Etag:  "etag-" + id,
		Title: title, Notes: notes, Status: status, Due: due,
	}, nil
}

func (f *fakeTasksClient) PatchTask(_ context.Context, tasklistID, taskID string, fields gateway.PatchFields, etag string) (gateway.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patched = append(f.patched, patchedCall{
		TasklistID: tasklistID, TaskID: taskID, Fields: fields, Etag: etag,
	})
	if f.patchErr != nil {
		return gateway.Task{}, f.patchErr
	}
	return gateway.Task{
		ID: taskID, Etag: "etag-" + taskID + "-patched",
		Title: fields.Title, Notes: fields.Notes, Status: fields.Status, Due: fields.Due,
	}, nil
}

func (f *fakeTasksClient) DeleteTask(_ context.Context, tasklistID, taskID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, deletedCall{TasklistID: tasklistID, TaskID: taskID})
	return f.deleteErr
}

// fakeCredentialReader returns a fixed refresh token (or an error).
type fakeCredentialReader struct {
	token string
	err   error
}

func (f *fakeCredentialReader) LoadRefreshToken(_ context.Context, _ wikipage.PageIdentifier) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

type silentLogger struct{}

func (silentLogger) Info(string, ...any)  {}
func (silentLogger) Error(string, ...any) {}

// fakeFrontmatterReadWriter is the in-memory stand-in for the wiki's
// page reader/writer. Used for FrontmatterCredentialStore tests.
type fakeFrontmatterReadWriter struct {
	pages map[wikipage.PageIdentifier]wikipage.FrontMatter
	err   map[wikipage.PageIdentifier]error
}

func newFakeFrontmatterReadWriter() *fakeFrontmatterReadWriter {
	return &fakeFrontmatterReadWriter{
		pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		err:   map[wikipage.PageIdentifier]error{},
	}
}

func (f *fakeFrontmatterReadWriter) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if e, ok := f.err[id]; ok {
		return id, nil, e
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeFrontmatterReadWriter) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.pages[id] = fm
	return nil
}

// --- specs -----------------------------------------------------------

var _ = Describe("TasksAdapter", func() {
	var (
		ctx           context.Context
		fakeClient    *fakeTasksClient
		creds         *fakeCredentialReader
		clientFactory googletasks.TasksClientFactory
		factoryErr    error
		adapter       *googletasks.TasksAdapter
		profile       wikipage.PageIdentifier
		remoteHandle  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = newFakeTasksClient()
		creds = &fakeCredentialReader{token: "rt-abc"}
		factoryErr = nil
		clientFactory = func(_ context.Context, _ wikipage.PageIdentifier, _ string) (googletasks.TasksClient, error) {
			if factoryErr != nil {
				return nil, factoryErr
			}
			return fakeClient, nil
		}
		var err error
		adapter, err = googletasks.NewTasksAdapter(creds, clientFactory, silentLogger{})
		Expect(err).NotTo(HaveOccurred())
		profile = wikipage.PageIdentifier("profile_alice")
		remoteHandle = "tasklist-x"
	})

	Describe("constructor", func() {
		When("any dependency is nil", func() {
			It("should reject nil credentials", func() {
				_, err := googletasks.NewTasksAdapter(nil, clientFactory, silentLogger{})
				Expect(err).To(MatchError(ContainSubstring("credentials must not be nil")))
			})

			It("should reject nil clientFactory", func() {
				_, err := googletasks.NewTasksAdapter(creds, nil, silentLogger{})
				Expect(err).To(MatchError(ContainSubstring("clientFactory must not be nil")))
			})

			It("should reject nil logger", func() {
				_, err := googletasks.NewTasksAdapter(creds, clientFactory, nil)
				Expect(err).To(MatchError(ContainSubstring("logger must not be nil")))
			})
		})
	})

	Describe("Kind", func() {
		It("should return ConnectorKindGoogleTasks", func() {
			Expect(adapter.Kind()).To(Equal(connectors.ConnectorKindGoogleTasks))
		})
	})

	Describe("SupportsSubtasks", func() {
		It("should report true (Tasks has parent-child)", func() {
			Expect(adapter.SupportsSubtasks()).To(BeTrue())
		})
	})

	Describe("PullRemote", func() {
		var (
			binding connectors.Binding
			result  connectors.RemotePullResult
			pullErr error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID:    profile,
				RemoteHandle: remoteHandle,
			}
		})

		When("the remote returns flat tasks", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-1", Etag: "e-1", Title: "milk", Status: gateway.TaskStatusNeedsAction, Updated: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
					{ID: "t-2", Etag: "e-2", Title: "eggs", Status: gateway.TaskStatusCompleted, Updated: time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should return both items", func() {
				Expect(result.Items).To(HaveLen(2))
			})

			It("should populate remote refs", func() {
				Expect(result.Items[0].Ref).To(Equal(connectors.RemoteRef("t-1")))
				Expect(result.Items[1].Ref).To(Equal(connectors.RemoteRef("t-2")))
			})

			It("should report Truncated=false", func() {
				Expect(result.Truncated).To(BeFalse())
			})

			It("should set NewCursor to max(updated)", func() {
				cursor, ok := result.NewCursor.(time.Time)
				Expect(ok).To(BeTrue())
				Expect(cursor).To(Equal(time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)))
			})
		})

		When("the credential reader fails", func() {
			BeforeEach(func() {
				creds.err = googletasks.ErrCredentialMissing
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should propagate the error", func() {
				Expect(pullErr).To(MatchError(googletasks.ErrCredentialMissing))
			})
		})

		When("the gateway returns an auth-revoked error", func() {
			BeforeEach(func() {
				fakeClient.listTasksErr = gateway.ErrAuthRevoked
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should bubble the gateway error so ClassifyError can map it", func() {
				Expect(pullErr).To(MatchError(gateway.ErrAuthRevoked))
			})
		})

		When("there are multiple pages", func() {
			BeforeEach(func() {
				fakeClient.listsForList[remoteHandle] = []gateway.TasksPage{
					{Tasks: []gateway.Task{{ID: "p1-1", Updated: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)}}, NextPageToken: "p2"},
					{Tasks: []gateway.Task{{ID: "p2-1", Updated: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}}},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should drain pagination internally", func() {
				Expect(result.Items).To(HaveLen(2))
			})
		})

		When("the binding's adapter state has a last_updated_min cursor", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "old", Updated: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
					{ID: "new", Updated: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)},
				}
				binding.AdapterState = connectors.AdapterState{
					googletasks.AdapterStateKeyLastUpdatedMin: "2026-05-04T00:00:00Z",
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should filter out items older than the cursor", func() {
				Expect(pullErr).NotTo(HaveOccurred())
				Expect(result.Items).To(HaveLen(1))
				Expect(result.Items[0].Ref).To(Equal(connectors.RemoteRef("new")))
			})
		})
	})

	Describe("InsertRemote", func() {
		var (
			binding connectors.Binding
			ref     connectors.RemoteRef
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
			}
			item := connectors.WikiItem{
				UID: "u-1", Text: "milk", Checked: false,
			}
			ref, err = adapter.InsertRemote(ctx, binding, item)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the new remote ref", func() {
			Expect(ref).To(Equal(connectors.RemoteRef("inserted-1")))
		})

		It("should call gateway InsertTask once", func() {
			Expect(fakeClient.inserted).To(HaveLen(1))
		})

		It("should pass the tasklist id and translated fields", func() {
			Expect(fakeClient.inserted[0].TasklistID).To(Equal(remoteHandle))
			Expect(fakeClient.inserted[0].Title).To(Equal("milk"))
			Expect(fakeClient.inserted[0].Status).To(Equal(gateway.TaskStatusNeedsAction))
		})
	})

	Describe("PatchRemote", func() {
		var (
			binding connectors.Binding
			ref     connectors.RemoteRef
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
				AdapterState: connectors.AdapterState{
					googletasks.AdapterStateKeyItemEtags: map[string]any{
						"t-9": "etag-old",
					},
				},
			}
			item := connectors.WikiItem{UID: "u-9", Text: "updated text", Checked: true}
			ref, err = adapter.PatchRemote(ctx, binding, "t-9", item)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the updated ref", func() {
			Expect(ref).To(Equal(connectors.RemoteRef("t-9")))
		})

		It("should pass the etag from adapter state as If-Match", func() {
			Expect(fakeClient.patched).To(HaveLen(1))
			Expect(fakeClient.patched[0].Etag).To(Equal("etag-old"))
		})

		It("should set every Set* flag", func() {
			Expect(fakeClient.patched[0].Fields.SetTitle).To(BeTrue())
			Expect(fakeClient.patched[0].Fields.SetNotes).To(BeTrue())
			Expect(fakeClient.patched[0].Fields.SetStatus).To(BeTrue())
			Expect(fakeClient.patched[0].Fields.SetDue).To(BeTrue())
		})
	})

	Describe("DeleteRemote", func() {
		var (
			binding connectors.Binding
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
			}
			err = adapter.DeleteRemote(ctx, binding, "t-7")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delegate to gateway DeleteTask", func() {
			Expect(fakeClient.deleted).To(HaveLen(1))
			Expect(fakeClient.deleted[0]).To(Equal(deletedCall{TasklistID: remoteHandle, TaskID: "t-7"}))
		})
	})

	Describe("RemoteToWiki", func() {
		var (
			remote connectors.RemoteItem
			wiki   connectors.WikiItem
			err    error
		)

		BeforeEach(func() {
			remote = connectors.RemoteItem{
				Ref:    "t-1",
				Title:  "milk #shopping",
				Notes:  "user note",
				Status: translator.TaskStatusCompleted,
			}
			wiki, err = adapter.RemoteToWiki(remote)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should split tags out of the title", func() {
			Expect(wiki.Text).To(Equal("milk"))
			Expect(wiki.Tags).To(ContainElement("shopping"))
		})

		It("should map completed → checked", func() {
			Expect(wiki.Checked).To(BeTrue())
		})

		It("should preserve user notes as Description", func() {
			Expect(wiki.Description).To(Equal("user note"))
		})

		When("the notes carry a wiki:uid marker", func() {
			BeforeEach(func() {
				remote = connectors.RemoteItem{
					Ref:   "t-2",
					Title: "eggs",
					Notes: "user note" + translator.WikiUIDMarker("UID-FROM-NOTES"),
				}
				wiki, err = adapter.RemoteToWiki(remote)
			})

			It("should expose the marker uid", func() {
				Expect(wiki.UID).To(Equal("UID-FROM-NOTES"))
			})

			It("should strip the marker from Description", func() {
				Expect(wiki.Description).To(Equal("user note"))
			})
		})
	})

	Describe("WikiToRemote", func() {
		var (
			remote connectors.RemoteItem
			err    error
		)

		BeforeEach(func() {
			wiki := connectors.WikiItem{
				UID: "u-1", Text: "milk", Tags: []string{"shopping"}, Checked: true,
			}
			remote, err = adapter.WikiToRemote(wiki)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should encode tags into the title", func() {
			Expect(remote.Title).To(Equal("milk #shopping"))
		})

		It("should encode checked → completed status", func() {
			Expect(remote.Status).To(Equal(translator.TaskStatusCompleted))
		})
	})

	Describe("AdvanceCursor", func() {
		When("NewCursor is a non-zero time", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{},
				}
				cursor := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: cursor})
			})

			It("should subtract the safety buffer (1s) and write to AdapterState", func() {
				rawCursor, ok := updated.AdapterState[googletasks.AdapterStateKeyLastUpdatedMin].(string)
				Expect(ok).To(BeTrue())
				Expect(rawCursor).To(Equal("2026-05-04T11:59:59Z"))
			})
		})

		When("NewCursor is zero", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{},
				}
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: time.Time{}})
			})

			It("should leave AdapterState unchanged", func() {
				_, ok := updated.AdapterState[googletasks.AdapterStateKeyLastUpdatedMin]
				Expect(ok).To(BeFalse())
			})
		})

		When("NewCursor regresses below the existing cursor", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googletasks.AdapterStateKeyLastUpdatedMin: "2026-05-04T20:00:00Z",
					},
				}
				cursor := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: cursor})
			})

			It("should not regress the cursor", func() {
				v := updated.AdapterState[googletasks.AdapterStateKeyLastUpdatedMin]
				Expect(v).To(Equal("2026-05-04T20:00:00Z"))
			})
		})
	})

	Describe("SeedBindingState", func() {
		var (
			state connectors.AdapterState
			err   error
		)

		When("the remote tasklist already has marker-bearing tasks", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-1", Etag: "e-1", Title: "milk", Notes: translator.WikiUIDMarker("U1")},
					{ID: "t-2", Etag: "e-2", Title: "eggs"},
				}
				state, err = adapter.SeedBindingState(ctx, profile, remoteHandle, nil)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should record marker uids → task ids", func() {
				idMap, ok := state[googletasks.AdapterStateKeyItemIDMap].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(idMap).To(HaveKeyWithValue("U1", "t-1"))
			})

			It("should record per-task etags", func() {
				etags, ok := state[googletasks.AdapterStateKeyItemEtags].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(etags).To(HaveKeyWithValue("t-1", "e-1"))
				Expect(etags).To(HaveKeyWithValue("t-2", "e-2"))
			})

			It("should initialize last_updated_min to empty string", func() {
				v, ok := state[googletasks.AdapterStateKeyLastUpdatedMin].(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(BeEmpty())
			})
		})
	})

	Describe("ValidateRemoteBinding", func() {
		When("the tasklist contains subtasks", func() {
			var err error

			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-parent", Title: "parent"},
					{ID: "t-child", Title: "child", Parent: "t-parent"},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should return ErrTasksListHasSubtasks", func() {
				Expect(err).To(MatchError(googletasks.ErrTasksListHasSubtasks))
			})
		})

		When("the tasklist is flat", func() {
			var err error

			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-1", Title: "milk"},
					{ID: "t-2", Title: "eggs"},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("remote_handle is empty", func() {
			It("should error", func() {
				err := adapter.ValidateRemoteBinding(ctx, profile, "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("remote_handle must not be empty"))
			})
		})
	})

	Describe("RebuildAdapterState", func() {
		var (
			binding connectors.Binding
			state   connectors.AdapterState
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
				AdapterState: connectors.AdapterState{
					googletasks.AdapterStateKeyLastUpdatedMin: "2026-05-04T20:00:00Z",
				},
			}
			fakeClient.listAllForList[remoteHandle] = []gateway.Task{
				{ID: "t-x", Etag: "e-x", Title: "marker-task", Notes: translator.WikiUIDMarker("UID-X")},
			}
			state, err = adapter.RebuildAdapterState(ctx, binding)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should rebuild item_id_map by markers", func() {
			idMap, ok := state[googletasks.AdapterStateKeyItemIDMap].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(idMap).To(HaveKeyWithValue("UID-X", "t-x"))
		})

		It("should reset last_updated_min so the next reconcile re-processes", func() {
			v, ok := state[googletasks.AdapterStateKeyLastUpdatedMin].(string)
			Expect(ok).To(BeTrue())
			Expect(v).To(BeEmpty())
		})
	})

	Describe("FetchRemoteListTitle", func() {
		var (
			title string
			ok    bool
			err   error
		)

		When("the tasklist exists", func() {
			BeforeEach(func() {
				fakeClient.taskLists = []gateway.TaskList{
					{ID: remoteHandle, Title: "Groceries"},
				}
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, remoteHandle)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the title and ok=true", func() {
				Expect(title).To(Equal("Groceries"))
				Expect(ok).To(BeTrue())
			})
		})

		When("the tasklist does not exist", func() {
			BeforeEach(func() {
				fakeClient.taskLists = []gateway.TaskList{
					{ID: "other-list", Title: "Other"},
				}
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, remoteHandle)
			})

			It("should return ok=false with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		When("remote_handle is empty", func() {
			BeforeEach(func() {
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, "")
			})

			It("should return ok=false silently", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(title).To(BeEmpty())
			})
		})
	})

	Describe("ListRemoteCollections", func() {
		var (
			collections []connectors.RemoteCollection
			err         error
		)

		BeforeEach(func() {
			fakeClient.taskLists = []gateway.TaskList{
				{ID: "tl-1", Title: "Groceries"},
				{ID: "tl-2", Title: "Travel"},
			}
			collections, err = adapter.ListRemoteCollections(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return every tasklist as a RemoteCollection", func() {
			Expect(collections).To(HaveLen(2))
			Expect(collections[0].Handle).To(Equal("tl-1"))
			Expect(collections[0].Title).To(Equal("Groceries"))
		})
	})

	Describe("EncodeAdapterState / DecodeAdapterState", func() {
		When("encoding a populated state", func() {
			It("should round-trip the keys", func() {
				input := connectors.AdapterState{
					googletasks.AdapterStateKeyItemIDMap: map[string]any{"u-1": "t-1"},
					googletasks.AdapterStateKeyItemEtags: map[string]any{"t-1": "e-1"},
				}
				encoded, err := adapter.EncodeAdapterState(input)
				Expect(err).NotTo(HaveOccurred())
				decoded, err := adapter.DecodeAdapterState(encoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded[googletasks.AdapterStateKeyItemIDMap]).To(Equal(map[string]any{"u-1": "t-1"}))
				Expect(decoded[googletasks.AdapterStateKeyItemEtags]).To(Equal(map[string]any{"t-1": "e-1"}))
			})
		})

		When("encoding nil", func() {
			It("should produce an envelope with empty maps", func() {
				encoded, err := adapter.EncodeAdapterState(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(encoded).To(HaveKey(googletasks.AdapterStateKeyItemIDMap))
				Expect(encoded).To(HaveKey(googletasks.AdapterStateKeyItemEtags))
				Expect(encoded).To(HaveKey(googletasks.AdapterStateKeyLastUpdatedMin))
			})
		})
	})

	Describe("ReadRemoteByRef", func() {
		var (
			binding connectors.Binding
			remote  connectors.RemoteItem
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
			}
		})

		When("the task is present in the list", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-9", Etag: "e-9", Title: "milk"},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "t-9")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should populate the RemoteItem", func() {
				Expect(remote.Ref).To(Equal(connectors.RemoteRef("t-9")))
				Expect(remote.Title).To(Equal("milk"))
				Expect(remote.Etag).To(Equal("e-9"))
			})
		})

		When("the task has been deleted", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "t-9", Title: "milk", Deleted: true},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "t-9")
			})

			It("should report Deleted=true with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(remote.Deleted).To(BeTrue())
			})
		})

		When("the task is not in the list at all", func() {
			BeforeEach(func() {
				fakeClient.listAllForList[remoteHandle] = []gateway.Task{
					{ID: "other"},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "t-9")
			})

			It("should report Deleted=true with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(remote.Deleted).To(BeTrue())
			})
		})
	})

	Describe("ClassifyError", func() {
		It("should map ErrCredentialMissing → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(googletasks.ErrCredentialMissing)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrInvalidGrant → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(gateway.ErrInvalidGrant)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrAuthRevoked → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(gateway.ErrAuthRevoked)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrPreconditionFailed → ErrorClassPreconditionFailed", func() {
			Expect(adapter.ClassifyError(gateway.ErrPreconditionFailed)).To(Equal(connectors.ErrorClassPreconditionFailed))
		})

		It("should map ErrRateLimited → ErrorClassRateLimited", func() {
			Expect(adapter.ClassifyError(gateway.ErrRateLimited)).To(Equal(connectors.ErrorClassRateLimited))
		})

		It("should map ErrNotFound → ErrorClassNotFound", func() {
			Expect(adapter.ClassifyError(gateway.ErrNotFound)).To(Equal(connectors.ErrorClassNotFound))
		})

		It("should map ErrServiceDisabled → ErrorClassFatal", func() {
			Expect(adapter.ClassifyError(gateway.ErrServiceDisabled)).To(Equal(connectors.ErrorClassFatal))
		})

		It("should map ErrPermissionDenied → ErrorClassFatal", func() {
			Expect(adapter.ClassifyError(gateway.ErrPermissionDenied)).To(Equal(connectors.ErrorClassFatal))
		})

		It("should map ErrProtocolDrift → ErrorClassFatal", func() {
			Expect(adapter.ClassifyError(gateway.ErrProtocolDrift)).To(Equal(connectors.ErrorClassFatal))
		})

		It("should map nil → ErrorClassNone", func() {
			Expect(adapter.ClassifyError(nil)).To(Equal(connectors.ErrorClassNone))
		})

		It("should map unknown errors to ErrorClassRetryable", func() {
			Expect(adapter.ClassifyError(errors.New("some random error"))).To(Equal(connectors.ErrorClassRetryable))
		})
	})

	Describe("compile-time contract", func() {
		It("should satisfy connectors.BackendAdapter", func() {
			var _ connectors.BackendAdapter = adapter
		})
	})
})

var _ = Describe("FrontmatterCredentialStore.LoadRefreshToken", func() {
	var (
		ctx    context.Context
		fmRead *fakeFrontmatterReadWriter
		store  *googletasks.FrontmatterCredentialStore
		pid    wikipage.PageIdentifier
	)

	BeforeEach(func() {
		ctx = context.Background()
		fmRead = newFakeFrontmatterReadWriter()
		var err error
		store, err = googletasks.NewFrontmatterCredentialStore(
			fmRead,
			googletasks.SystemClock{},
			silentLogger{},
			nil, // pauseAll: nil for read-only test path
			nil, // resumeAll: nil for read-only test path
		)
		Expect(err).NotTo(HaveOccurred())
		pid = wikipage.PageIdentifier("profile_alice")
	})

	When("the profile page does not exist", func() {
		It("should return ErrCredentialMissing", func() {
			_, err := store.LoadRefreshToken(ctx, pid)
			Expect(err).To(MatchError(googletasks.ErrCredentialMissing))
		})
	})

	When("the page exists but has no Tasks frontmatter", func() {
		BeforeEach(func() {
			fmRead.pages[pid] = wikipage.FrontMatter{}
		})

		It("should return ErrCredentialMissing", func() {
			_, err := store.LoadRefreshToken(ctx, pid)
			Expect(err).To(MatchError(googletasks.ErrCredentialMissing))
		})
	})

	When("the refresh_token is non-empty", func() {
		BeforeEach(func() {
			fmRead.pages[pid] = wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"refresh_token": "rt-real",
						},
					},
				},
			}
		})

		It("should return the token", func() {
			tok, err := store.LoadRefreshToken(ctx, pid)
			Expect(err).NotTo(HaveOccurred())
			Expect(tok).To(Equal("rt-real"))
		})
	})
})

