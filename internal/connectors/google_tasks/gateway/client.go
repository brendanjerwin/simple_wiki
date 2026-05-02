package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTasksBaseURL is the production Tasks REST v1 base. Tests
// inject a stand-in URL via NewTasksClient.
const DefaultTasksBaseURL = "https://tasks.googleapis.com/tasks/v1/"

// dueDateFormat is Tasks' wire format for `due` — RFC3339 with the
// time portion always zeroed to 00:00:00Z. Google's docs label the
// field "date-only," but the wire shape is a full timestamp; the
// time-of-day is always discarded server-side.
const dueDateFormat = "2006-01-02T15:04:05.000Z"

// errEmptyTasklistID is the canonical error message for the four
// TasksClient methods that require a non-empty tasklist id.
const errEmptyTasklistID = "tasks: tasklistID must not be empty"

// TokenSource is the contract the TasksClient uses to obtain a fresh
// bearer access token on each request. The RefreshClient implements
// this; tests inject a static stub.
type TokenSource interface {
	// AccessToken returns a fresh access token. Implementations MUST
	// refresh proactively when the cached token is near expiry — the
	// TasksClient assumes the returned token is good "right now."
	AccessToken(ctx context.Context) (string, error)

	// Invalidate signals to the source that the previously-issued
	// token was rejected (401 from the Tasks API). The next AccessToken
	// call should round-trip to the token endpoint.
	Invalidate()
}

// TasksClient performs Tasks REST v1 calls. It does NOT manage OAuth
// state itself — it asks the supplied TokenSource for a bearer on
// every call. On 401 from the Tasks API it Invalidates the source
// and retries once; if that retry also fails with 401 the error
// surfaces as ErrAuthRevoked.
type TasksClient struct {
	httpClient  *http.Client
	baseURL     string
	tokenSource TokenSource
}

// NewTasksClient wires a TasksClient. baseURL is normally
// DefaultTasksBaseURL; tests inject httptest.Server.URL + "/".
func NewTasksClient(httpClient *http.Client, baseURL string, tokenSource TokenSource) (*TasksClient, error) {
	if httpClient == nil {
		return nil, errors.New("tasks: httpClient must not be nil")
	}
	if baseURL == "" {
		return nil, errors.New("tasks: baseURL must not be empty")
	}
	if !strings.HasSuffix(baseURL, "/") {
		return nil, fmt.Errorf("tasks: baseURL %q must end with /", baseURL)
	}
	if tokenSource == nil {
		return nil, errors.New("tasks: tokenSource must not be nil")
	}
	return &TasksClient{
		httpClient:  httpClient,
		baseURL:     baseURL,
		tokenSource: tokenSource,
	}, nil
}

// ListTaskLists enumerates the user's tasklists. Used by the
// subscribe-picker UI in Phase 8 to show the operator which Tasks
// lists are available to subscribe to.
//
// Google paginates this endpoint, but a household-scale account has
// at most a handful of tasklists; the wiki consumes all pages eagerly
// and returns the flat list.
func (c *TasksClient) ListTaskLists(ctx context.Context) ([]TaskList, error) {
	var out []TaskList
	pageToken := ""
	for {
		page, next, err := c.listTaskListsPage(ctx, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		pageToken = next
	}
}

func (c *TasksClient) listTaskListsPage(ctx context.Context, pageToken string) ([]TaskList, string, error) {
	q := url.Values{}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	body, err := c.do(ctx, http.MethodGet, "users/@me/lists", q, nil, "")
	if err != nil {
		return nil, "", err
	}
	var wire wireTaskListsPage
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, "", fmt.Errorf("%w: decode tasklists: %w", ErrProtocolDrift, err)
	}
	out := make([]TaskList, 0, len(wire.Items))
	for _, w := range wire.Items {
		tl, err := decodeTaskList(w)
		if err != nil {
			return nil, "", fmt.Errorf("%w: tasklist %q: %w", ErrProtocolDrift, w.ID, err)
		}
		out = append(out, tl)
	}
	return out, wire.NextPageToken, nil
}

// CreateTaskList creates a new Google Tasks tasklist with the given
// title. Mirrors the wire shape of the tasklists.insert endpoint —
// POST /tasks/v1/users/@me/lists with body {"title": "..."}. Returns
// the server-assigned TaskList (id, etag, updated populated).
//
// Used by the subscribe-ceremony when the user opts to "Bind to a new
// Tasks list" rather than picking from an existing tasklist. Mirrors
// the Keep gateway's CreateList convenience.
func (c *TasksClient) CreateTaskList(ctx context.Context, title string) (TaskList, error) {
	if title == "" {
		return TaskList{}, errors.New("tasks: title must not be empty")
	}
	wire := wireTaskList{Title: title}
	reqBody, err := json.Marshal(wire)
	if err != nil {
		return TaskList{}, fmt.Errorf("encode tasklist: %w", err)
	}
	respBody, err := c.do(ctx, http.MethodPost, "users/@me/lists", nil, reqBody, "")
	if err != nil {
		return TaskList{}, err
	}
	var got wireTaskList
	if err := json.Unmarshal(respBody, &got); err != nil {
		return TaskList{}, fmt.Errorf("%w: decode createTaskList response: %w", ErrProtocolDrift, err)
	}
	return decodeTaskList(got)
}

// ListTasks fetches one page of tasks.list from the given tasklist.
// updatedMin is the cursor (zero = first sync); pageToken is the
// continuation cursor when walking a multi-page response. The caller
// is responsible for the apply-then-advance ordering — see plan
// "Cursor" semantics.
//
// showHidden=true and showDeleted=true are always set so the wiki sees
// completed-and-hidden items and tombstones (required for the
// inbound-deletion path).
func (c *TasksClient) ListTasks(ctx context.Context, tasklistID string, updatedMin time.Time, pageToken string) (TasksPage, error) {
	if tasklistID == "" {
		return TasksPage{}, errors.New(errEmptyTasklistID)
	}
	q := url.Values{}
	q.Set("showHidden", "true")
	q.Set("showDeleted", "true")
	q.Set("showCompleted", "true")
	q.Set("maxResults", "100")
	if !updatedMin.IsZero() {
		q.Set("updatedMin", updatedMin.UTC().Format(time.RFC3339Nano))
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	body, err := c.do(ctx, http.MethodGet, "lists/"+url.PathEscape(tasklistID)+"/tasks", q, nil, "")
	if err != nil {
		return TasksPage{}, err
	}
	var wire wireTasksPage
	if err := json.Unmarshal(body, &wire); err != nil {
		return TasksPage{}, fmt.Errorf("%w: decode tasks: %w", ErrProtocolDrift, err)
	}
	out := TasksPage{NextPageToken: wire.NextPageToken}
	for _, w := range wire.Items {
		t, err := decodeTask(w)
		if err != nil {
			return TasksPage{}, fmt.Errorf("%w: task %q: %w", ErrProtocolDrift, w.ID, err)
		}
		out.Tasks = append(out.Tasks, t)
	}
	return out, nil
}

// InsertTask creates a new task in the given tasklist. Returns the
// server-assigned Task (id, etag, updated populated).
//
// The `parent` argument is for Tasks subtask hierarchy — the wiki
// refuses-to-subscribe on initial subtasks (per plan §3) and flattens
// post-subscribe subtasks, so callers from the sync layer pass "" and
// only the cmd/tasks-debug exploratory CLI exercises non-empty values.
func (c *TasksClient) InsertTask(ctx context.Context, tasklistID, title, notes string, status TaskStatus, due time.Time, parent string) (Task, error) {
	if tasklistID == "" {
		return Task{}, errors.New(errEmptyTasklistID)
	}
	if title == "" {
		return Task{}, errors.New("tasks: title must not be empty")
	}
	if status == "" {
		status = TaskStatusNeedsAction
	}
	wire := wireTask{
		Title:  title,
		Notes:  notes,
		Status: string(status),
	}
	if !due.IsZero() {
		wire.Due = due.UTC().Format(dueDateFormat)
	}
	q := url.Values{}
	if parent != "" {
		q.Set("parent", parent)
	}
	reqBody, err := json.Marshal(wire)
	if err != nil {
		return Task{}, fmt.Errorf("encode task: %w", err)
	}
	respBody, err := c.do(ctx, http.MethodPost, "lists/"+url.PathEscape(tasklistID)+"/tasks", q, reqBody, "")
	if err != nil {
		return Task{}, err
	}
	var got wireTask
	if err := json.Unmarshal(respBody, &got); err != nil {
		return Task{}, fmt.Errorf("%w: decode insert response: %w", ErrProtocolDrift, err)
	}
	return decodeTask(got)
}

// PatchTask sends a tasks.patch with If-Match. On HTTP 412 returns
// ErrPreconditionFailed so the caller can pull-and-retry-once with a
// fresh etag. Empty etag skips the If-Match header (last-write-wins).
func (c *TasksClient) PatchTask(ctx context.Context, tasklistID, taskID string, fields PatchFields, etag string) (Task, error) {
	if tasklistID == "" {
		return Task{}, errors.New(errEmptyTasklistID)
	}
	if taskID == "" {
		return Task{}, errors.New("tasks: taskID must not be empty")
	}
	body, err := encodePatch(fields)
	if err != nil {
		return Task{}, err
	}
	respBody, err := c.do(ctx, http.MethodPatch, "lists/"+url.PathEscape(tasklistID)+"/tasks/"+url.PathEscape(taskID), nil, body, etag)
	if err != nil {
		return Task{}, err
	}
	var got wireTask
	if err := json.Unmarshal(respBody, &got); err != nil {
		return Task{}, fmt.Errorf("%w: decode patch response: %w", ErrProtocolDrift, err)
	}
	return decodeTask(got)
}

// DeleteTask issues tasks.delete. Google-side idempotent — repeated
// deletes return 204 (or 404 when the task is fully gone). The wiki
// treats both as success.
func (c *TasksClient) DeleteTask(ctx context.Context, tasklistID, taskID string) error {
	if tasklistID == "" {
		return errors.New(errEmptyTasklistID)
	}
	if taskID == "" {
		return errors.New("tasks: taskID must not be empty")
	}
	_, err := c.do(ctx, http.MethodDelete, "lists/"+url.PathEscape(tasklistID)+"/tasks/"+url.PathEscape(taskID), nil, nil, "")
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// do is the single request entrypoint. It attaches the bearer, sends
// the request, classifies non-2xx into typed errors, and returns the
// raw success body. On 401 it retries once after invalidating the
// token source — this handles the "access token expired between
// AccessToken() and Tasks API call" race transparently.
//
//revive:disable-next-line:cyclomatic // single-purpose dispatcher; splitting hurts readability
func (c *TasksClient) do(ctx context.Context, method, relPath string, query url.Values, body []byte, ifMatch string) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.tokenSource.AccessToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("acquire access token: %w", err)
		}
		full := c.baseURL + relPath
		if len(query) > 0 {
			full = full + "?" + query.Encode()
		}
		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, full, reqBody)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, relPath, err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("%w: read response: %w", ErrProtocolDrift, readErr)
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			// Stale access token race — invalidate and refresh once.
			c.tokenSource.Invalidate()
			continue
		}
		if classified := classifyTasksHTTPStatus(resp.StatusCode); classified != nil {
			return nil, fmt.Errorf("%w: HTTP %d: %s", classified, resp.StatusCode, truncateBody(respBody))
		}
		return respBody, nil
	}
	return nil, fmt.Errorf("%w: token rejected after retry", ErrAuthRevoked)
}

// classifyTasksHTTPStatus maps Tasks REST v1 status codes to typed
// errors. nil means "proceed with body decode."
func classifyTasksHTTPStatus(code int) error {
	switch code {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrAuthRevoked
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusForbidden:
		// Google often uses 403 for rate-limit; the bridge's caller
		// can't easily disambiguate without the response body, so
		// classify as rate-limited — back-off-and-retry is the right
		// answer for either reason.
		return ErrRateLimited
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusPreconditionFailed:
		return ErrPreconditionFailed
	case http.StatusBadRequest:
		return errors.New("tasks: bad request")
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return errors.New("tasks: upstream unavailable")
	default:
		return fmt.Errorf("tasks: unexpected status %d", code)
	}
}

// --- wire types -----------------------------------------------------

type wireTaskListsPage struct {
	Kind          string         `json:"kind,omitempty"`
	Etag          string         `json:"etag,omitempty"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
	Items         []wireTaskList `json:"items,omitempty"`
}

type wireTaskList struct {
	Kind     string `json:"kind,omitempty"`
	ID       string `json:"id"`
	Etag     string `json:"etag,omitempty"`
	Title    string `json:"title,omitempty"`
	Updated  string `json:"updated,omitempty"`
	SelfLink string `json:"selfLink,omitempty"`
}

type wireTasksPage struct {
	Kind          string     `json:"kind,omitempty"`
	Etag          string     `json:"etag,omitempty"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
	Items         []wireTask `json:"items,omitempty"`
}

// wireTask mirrors the Google Tasks Task resource. We only model the
// fields the wiki cares about; selfLink, kind, links, webViewLink
// round-trip but are dropped on decode.
type wireTask struct {
	Kind      string `json:"kind,omitempty"`
	ID        string `json:"id,omitempty"`
	Etag      string `json:"etag,omitempty"`
	Title     string `json:"title,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Status    string `json:"status,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Position  string `json:"position,omitempty"`
	Due       string `json:"due,omitempty"`
	Completed string `json:"completed,omitempty"`
	Updated   string `json:"updated,omitempty"`
	Hidden    bool   `json:"hidden,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
	SelfLink  string `json:"selfLink,omitempty"`
}

// encodePatch builds the JSON body for tasks.patch. Only fields with
// `Set*` flags are serialized — omitting a field tells Google to leave
// it alone. Cleared fields (e.g. Due == zero with SetDue == true) are
// emitted as JSON null so Google clears them server-side.
func encodePatch(fields PatchFields) ([]byte, error) {
	out := map[string]any{}
	if fields.SetTitle {
		out["title"] = fields.Title
	}
	if fields.SetNotes {
		out["notes"] = fields.Notes
	}
	if fields.SetStatus {
		out["status"] = string(fields.Status)
	}
	if fields.SetDue {
		if fields.Due.IsZero() {
			out["due"] = nil
		} else {
			out["due"] = fields.Due.UTC().Format(dueDateFormat)
		}
	}
	if fields.SetParent {
		out["parent"] = fields.Parent
	}
	if len(out) == 0 {
		return nil, errors.New("tasks: patch must mutate at least one field")
	}
	return json.Marshal(out)
}

// decodeTaskList parses a wireTaskList into the wiki's TaskList shape.
func decodeTaskList(w wireTaskList) (TaskList, error) {
	if w.ID == "" {
		return TaskList{}, errors.New("missing id")
	}
	updated, err := parseTasksTimestamp(w.Updated)
	if err != nil {
		return TaskList{}, fmt.Errorf("updated: %w", err)
	}
	return TaskList{
		ID:      w.ID,
		Etag:    w.Etag,
		Title:   w.Title,
		Updated: updated,
	}, nil
}

// decodeTask parses a wireTask into the wiki's Task shape.
func decodeTask(w wireTask) (Task, error) {
	if w.ID == "" {
		return Task{}, errors.New("missing id")
	}
	due, err := parseTasksTimestamp(w.Due)
	if err != nil {
		return Task{}, fmt.Errorf("due: %w", err)
	}
	completed, err := parseTasksTimestamp(w.Completed)
	if err != nil {
		return Task{}, fmt.Errorf("completed: %w", err)
	}
	updated, err := parseTasksTimestamp(w.Updated)
	if err != nil {
		return Task{}, fmt.Errorf("updated: %w", err)
	}
	status := TaskStatus(w.Status)
	if status != "" && status != TaskStatusNeedsAction && status != TaskStatusCompleted {
		return Task{}, fmt.Errorf("unknown status %q", w.Status)
	}
	return Task{
		ID:        w.ID,
		Etag:      w.Etag,
		Title:     w.Title,
		Notes:     w.Notes,
		Status:    status,
		Parent:    w.Parent,
		Position:  w.Position,
		Due:       due,
		Completed: completed,
		Updated:   updated,
		Hidden:    w.Hidden,
		Deleted:   w.Deleted,
	}, nil
}

// parseTasksTimestamp accepts an RFC3339-ish wire string. Returns
// zero/no-error for empty input; errors loudly on a non-empty
// unparseable value (mirrors the Keep gateway's parseRFC3339Micros
// rationale — silently zeroing a malformed timestamp would collapse
// "completed" / "due" into "live and unchanged.")
func parseTasksTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Tasks docs say RFC3339 with millis and Z; in practice we've seen
	// nano-second precision too. Try the canonical form first, then
	// the standard RFC3339Nano fallback.
	if t, err := time.Parse(dueDateFormat, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a valid RFC3339 timestamp: %w", err)
	}
	return t.UTC(), nil
}
