//revive:disable:dot-imports
package sync_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeClock returns a deterministic time. Tests advance the clock
// explicitly via SetNow.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{now: t.UTC()}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) SetNow(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// silentLogger discards every message; tests assert on side-effects
// rather than log content.
type silentLogger struct{}

func (silentLogger) Info(string, ...any)  {}
func (silentLogger) Error(string, ...any) {}

// fakeTasksClient is the in-memory stand-in for *gateway.TasksClient
// used by sync tests. State is recorded so test assertions can read
// what the orchestrator did.
type fakeTasksClient struct {
	mu sync.Mutex

	// listsForList: tasklistID → ordered list of pages to return on
	// successive ListTasks calls. Each call consumes one page.
	listsForList map[string][]gateway.TasksPage
	// listAllForList: tasklistID → flat task list returned on every
	// non-paginated ListTasks call.
	listAllForList map[string][]gateway.Task

	// errorsForListTasks is consulted before consulting listsForList.
	// First entry is consumed; on empty, no error.
	errorsForListTasks []error

	// inserted captures (tasklistID, payload) tuples in order.
	inserted []insertedCall

	// patched captures (tasklistID, taskID, fields, etag) in order.
	patched []patchedCall

	// patchErrors are returned from PatchTask in order, then nil.
	patchErrors []error

	// deleted captures (tasklistID, taskID) in order.
	deleted []deletedCall

	// taskLists is what ListTaskLists returns.
	taskLists []gateway.TaskList

	// nextInsertID returns ascending ids for InsertTask.
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
	return append([]gateway.TaskList(nil), f.taskLists...), nil
}

func (f *fakeTasksClient) ListTasks(_ context.Context, tasklistID string, _ time.Time, pageToken string) (gateway.TasksPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.errorsForListTasks) > 0 {
		err := f.errorsForListTasks[0]
		f.errorsForListTasks = f.errorsForListTasks[1:]
		if err != nil {
			return gateway.TasksPage{}, err
		}
	}
	pages, hasPaginated := f.listsForList[tasklistID]
	if hasPaginated && len(pages) > 0 {
		page := pages[0]
		f.listsForList[tasklistID] = pages[1:]
		return page, nil
	}
	if pageToken != "" {
		// All pages consumed.
		return gateway.TasksPage{}, nil
	}
	if all, ok := f.listAllForList[tasklistID]; ok {
		return gateway.TasksPage{Tasks: all}, nil
	}
	return gateway.TasksPage{}, nil
}

func (f *fakeTasksClient) InsertTask(_ context.Context, tasklistID, title, notes string, status gateway.TaskStatus, due time.Time, parent string) (gateway.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextInsertID++
	id := fmt.Sprintf("inserted-%d", f.nextInsertID)
	f.inserted = append(f.inserted, insertedCall{
		TasklistID: tasklistID, Title: title, Notes: notes, Status: status, Due: due, Parent: parent,
	})
	return gateway.Task{
		ID:      id,
		Etag:    "etag-" + id,
		Title:   title,
		Notes:   notes,
		Status:  status,
		Due:     due,
		Updated: time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeTasksClient) PatchTask(_ context.Context, tasklistID, taskID string, fields gateway.PatchFields, etag string) (gateway.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patched = append(f.patched, patchedCall{TasklistID: tasklistID, TaskID: taskID, Fields: fields, Etag: etag})
	if len(f.patchErrors) > 0 {
		err := f.patchErrors[0]
		f.patchErrors = f.patchErrors[1:]
		if err != nil {
			return gateway.Task{}, err
		}
	}
	out := gateway.Task{
		ID:      taskID,
		Etag:    "etag-" + taskID + "-patched",
		Title:   fields.Title,
		Notes:   fields.Notes,
		Status:  fields.Status,
		Due:     fields.Due,
		Updated: time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC),
	}
	return out, nil
}

func (f *fakeTasksClient) DeleteTask(_ context.Context, tasklistID, taskID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, deletedCall{TasklistID: tasklistID, TaskID: taskID})
	return nil
}

// fakeChecklistReader returns canned wiki checklists keyed by
// (page, listName).
type fakeChecklistReader struct {
	mu        sync.Mutex
	responses map[string]*apiv1.Checklist
	err       error
}

func newFakeChecklistReader() *fakeChecklistReader {
	return &fakeChecklistReader{responses: map[string]*apiv1.Checklist{}}
}

func (f *fakeChecklistReader) Set(page, listName string, items []*apiv1.ChecklistItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := page + "|" + listName
	f.responses[key] = &apiv1.Checklist{Name: listName, Items: items}
}

func (f *fakeChecklistReader) ListItems(_ context.Context, page, listName string) (*apiv1.Checklist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.responses[page+"|"+listName], nil
}

func (f *fakeChecklistReader) appendItem(page, listName string, item *apiv1.ChecklistItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := page + "|" + listName
	cl := f.responses[key]
	if cl == nil {
		cl = &apiv1.Checklist{Name: listName}
		f.responses[key] = cl
	}
	cl.Items = append(cl.Items, item)
}

func (f *fakeChecklistReader) updateItem(page, listName, uid, text string, checked bool, tags []string, description *string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cl := f.responses[page+"|"+listName]
	if cl == nil {
		return
	}
	for _, it := range cl.Items {
		if it.GetUid() == uid {
			it.Text = text
			it.Checked = checked
			it.Tags = tags
			it.Description = description
			return
		}
	}
}

func (f *fakeChecklistReader) deleteItem(page, listName, uid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cl := f.responses[page+"|"+listName]
	if cl == nil {
		return
	}
	out := cl.Items[:0]
	for _, it := range cl.Items {
		if it.GetUid() != uid {
			out = append(out, it)
		}
	}
	cl.Items = out
}

// fakeChecklistMutator records every Add/Update/Delete the inbound
// apply pass invokes. Returned uids from AddItemForSync are
// ascending integers so the orchestrator's id_map population is
// deterministic.
//
// reader is optional — when set, every Add/Update/Delete also
// mutates the corresponding fakeChecklistReader response so a later
// ListItems call returns the post-mutation view (matching real-world
// consistency between the mutator and the reader's index).
type fakeChecklistMutator struct {
	mu             sync.Mutex
	added          []addItemCall
	updated        []updateItemCall
	deleted        []deleteItemCall
	nextAdded      int
	uidPrefix      string
	addReturnError error
	reader         *fakeChecklistReader
}

type addItemCall struct {
	Page          string
	ListName      string
	OwnerEmail    string
	Text          string
	Checked       bool
	Tags          []string
	Description   string
	SortValueHint string
}

type updateItemCall struct {
	Page        string
	ListName    string
	OwnerEmail  string
	UID         string
	Text        string
	Checked     bool
	Tags        []string
	Description string
}

type deleteItemCall struct {
	Page       string
	ListName   string
	OwnerEmail string
	UID        string
}

func newFakeChecklistMutator() *fakeChecklistMutator {
	return &fakeChecklistMutator{uidPrefix: "wiki-uid-"}
}

// newFakeChecklistMutatorBoundTo wires the mutator to a reader so
// every Add/Update/Delete keeps the reader's view consistent — the
// real wiki's mutator and the reader share the underlying page
// store, so post-mutation reads see the writes.
func newFakeChecklistMutatorBoundTo(reader *fakeChecklistReader) *fakeChecklistMutator {
	return &fakeChecklistMutator{uidPrefix: "wiki-uid-", reader: reader}
}

func (f *fakeChecklistMutator) AddItemForSync(_ context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, sortValueHint string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addReturnError != nil {
		return "", f.addReturnError
	}
	f.nextAdded++
	uid := fmt.Sprintf("%s%d", f.uidPrefix, f.nextAdded)
	f.added = append(f.added, addItemCall{
		Page: page, ListName: listName, OwnerEmail: ownerEmail, Text: text,
		Checked: checked, Tags: tags, Description: description, SortValueHint: sortValueHint,
	})
	if f.reader != nil {
		var desc *string
		if description != "" {
			d := description
			desc = &d
		}
		f.reader.appendItem(page, listName, &apiv1.ChecklistItem{
			Uid: uid, Text: text, Checked: checked, Tags: tags, Description: desc,
		})
	}
	return uid, nil
}

func (f *fakeChecklistMutator) UpdateItemForSync(_ context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, updateItemCall{
		Page: page, ListName: listName, OwnerEmail: ownerEmail, UID: uid, Text: text,
		Checked: checked, Tags: tags, Description: description,
	})
	if f.reader != nil {
		var desc *string
		if description != "" {
			d := description
			desc = &d
		}
		f.reader.updateItem(page, listName, uid, text, checked, tags, desc)
	}
	return nil
}

func (f *fakeChecklistMutator) DeleteItemForSync(_ context.Context, page, listName, ownerEmail, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, deleteItemCall{
		Page: page, ListName: listName, OwnerEmail: ownerEmail, UID: uid,
	})
	if f.reader != nil {
		f.reader.deleteItem(page, listName, uid)
	}
	return nil
}

// fakeSuppressor records Suppress/Unsuppress calls for assertions.
type fakeSuppressor struct {
	mu                sync.Mutex
	suppressCalls     []suppressKey
	unsuppressCalls   []suppressKey
	currentlyHeld     map[suppressKey]int
}

type suppressKey struct {
	Profile  wikipage.PageIdentifier
	Page     string
	ListName string
}

func newFakeSuppressor() *fakeSuppressor {
	return &fakeSuppressor{currentlyHeld: map[suppressKey]int{}}
}

func (f *fakeSuppressor) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	k := suppressKey{Profile: profileID, Page: page, ListName: listName}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suppressCalls = append(f.suppressCalls, k)
	f.currentlyHeld[k]++
}

func (f *fakeSuppressor) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	k := suppressKey{Profile: profileID, Page: page, ListName: listName}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsuppressCalls = append(f.unsuppressCalls, k)
	f.currentlyHeld[k]--
}

// stubFactoryThatReturns returns a TasksClientFactory that always
// returns the given client. Used to inject a single fake into the
// connector under test.
func stubFactoryThatReturns(client taskssync.TasksClient) taskssync.TasksClientFactory {
	return func(wikipage.PageIdentifier, string) (taskssync.TasksClient, gateway.TokenSource, error) {
		return client, nil, nil
	}
}
