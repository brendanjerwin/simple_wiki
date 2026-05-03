// tasks-debug is a single-binary diagnostic CLI for the Google Tasks
// bridge. Mirrors cmd/keep-debug — branchy subcommand handlers, exit
// on error, rich progress text on stdout. Useful for poking the live
// Tasks API and the OAuth refresh flow by hand.
//
// Reads creds from env:
//
//	TASKS_CLIENT_ID
//	TASKS_CLIENT_SECRET
//	TASKS_REFRESH_TOKEN
//
// Subcommands:
//
//	list                          — enumerate tasklists, then list tasks in -tasklist
//	insert                        — create a task in -tasklist
//	patch                         — patch -task in -tasklist with -title / -notes / -status / -due
//	delete                        — delete -task in -tasklist
//	verify-position-monotonic     — insert N tasks, fetch back, assert position values strictly increase
//	verify-pkce-flow              — generate PKCE pair + state token; print the auth URL
//	verify-updatedmin-boundary    — empirically determine inclusive/exclusive semantics
//
// The CLI is for live API exploration; failure modes can be log.Fatal
// style (matches cmd/keep-debug).
//
//revive:disable:deep-exit
//revive:disable:unhandled-error
//revive:disable:cognitive-complexity
//revive:disable:cyclomatic
//revive:disable:function-length
//revive:disable:add-constant
//revive:disable:flag-parameter
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
)

func main() {
	cmd := flag.String("cmd", "list", "list | insert | patch | delete | verify-position-monotonic | verify-pkce-flow | verify-updatedmin-boundary")
	tasklistID := flag.String("tasklist", "", "tasklist id (required for list/insert/patch/delete/verify-*)")
	taskID := flag.String("task", "", "task id (required for patch/delete)")
	title := flag.String("title", "", "title for insert / patch")
	notes := flag.String("notes", "", "notes for insert / patch")
	status := flag.String("status", "", "status for insert / patch (needsAction|completed)")
	dueStr := flag.String("due", "", "due date YYYY-MM-DD (insert / patch)")
	count := flag.Int("count", 5, "number of items for verify-position-monotonic")
	redirectURI := flag.String("redirect-uri", "https://wiki.example/oauth/google/callback", "redirect URI for verify-pkce-flow")
	flag.Parse()

	clientID := os.Getenv("TASKS_CLIENT_ID")
	clientSecret := os.Getenv("TASKS_CLIENT_SECRET")
	refreshToken := os.Getenv("TASKS_REFRESH_TOKEN")

	// verify-pkce-flow is the one subcommand that doesn't need OAuth
	// credentials at all — it just generates and prints PKCE state.
	if *cmd == "verify-pkce-flow" {
		runVerifyPKCEFlow(clientID, *redirectURI)
		return
	}

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		fmt.Fprintln(os.Stderr, "set TASKS_CLIENT_ID, TASKS_CLIENT_SECRET, TASKS_REFRESH_TOKEN")
		os.Exit(2)
	}

	ctx := context.Background()
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
		Timeout: 30 * time.Second,
	}
	store := gateway.NewStaticRefreshTokenStore(refreshToken)
	refresh, err := gateway.NewRefreshClient(httpClient, gateway.DefaultGoogleTokenURL, clientID, clientSecret, store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "refresh client:", err)
		os.Exit(1)
	}

	tasks, err := gateway.NewTasksClient(httpClient, gateway.DefaultTasksBaseURL, refresh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tasks client:", err)
		os.Exit(1)
	}

	switch *cmd {
	case "list":
		runList(ctx, tasks, *tasklistID)
	case "insert":
		mustNonEmpty("tasklist", *tasklistID)
		mustNonEmpty("title", *title)
		runInsert(ctx, tasks, *tasklistID, *title, *notes, *status, *dueStr)
	case "patch":
		mustNonEmpty("tasklist", *tasklistID)
		mustNonEmpty("task", *taskID)
		runPatch(ctx, tasks, *tasklistID, *taskID, *title, *notes, *status, *dueStr)
	case "delete":
		mustNonEmpty("tasklist", *tasklistID)
		mustNonEmpty("task", *taskID)
		runDelete(ctx, tasks, *tasklistID, *taskID)
	case "verify-position-monotonic":
		mustNonEmpty("tasklist", *tasklistID)
		runVerifyPositionMonotonic(ctx, tasks, *tasklistID, *count)
	case "verify-updatedmin-boundary":
		mustNonEmpty("tasklist", *tasklistID)
		runVerifyUpdatedMinBoundary(ctx, tasks, *tasklistID)
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(2)
	}
}

func mustNonEmpty(name, val string) {
	if val == "" {
		fmt.Fprintf(os.Stderr, "missing -%s\n", name)
		os.Exit(2)
	}
}

func runList(ctx context.Context, c *gateway.TasksClient, tasklistID string) {
	if tasklistID == "" {
		// Enumerate tasklists.
		lists, err := c.ListTaskLists(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list tasklists:", err)
			os.Exit(1)
		}
		fmt.Printf("found %d tasklists\n", len(lists))
		for _, l := range lists {
			fmt.Printf("  %s  %s  (updated %s)\n", l.ID, l.Title, l.Updated.Format(time.RFC3339))
		}
		return
	}
	// Tasks in the named tasklist.
	page, err := c.ListTasks(ctx, tasklistID, time.Time{}, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "list tasks:", err)
		os.Exit(1)
	}
	fmt.Printf("found %d tasks (nextPageToken=%q)\n", len(page.Tasks), page.NextPageToken)
	dumpTasks(page.Tasks)
	for page.NextPageToken != "" {
		page, err = c.ListTasks(ctx, tasklistID, time.Time{}, page.NextPageToken)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list tasks (next):", err)
			os.Exit(1)
		}
		fmt.Printf("(next page) %d tasks (nextPageToken=%q)\n", len(page.Tasks), page.NextPageToken)
		dumpTasks(page.Tasks)
	}
}

func runInsert(ctx context.Context, c *gateway.TasksClient, tasklistID, title, notes, statusStr, dueStr string) {
	due, err := parseDueFlag(dueStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad -due:", err)
		os.Exit(2)
	}
	status := gateway.TaskStatus(statusStr)
	if status == "" {
		status = gateway.TaskStatusNeedsAction
	}
	task, err := c.InsertTask(ctx, tasklistID, title, notes, status, due, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "insert:", err)
		os.Exit(1)
	}
	fmt.Printf("inserted task id=%s etag=%s updated=%s\n", task.ID, task.Etag, task.Updated.Format(time.RFC3339Nano))
	pretty, _ := json.MarshalIndent(task, "", "  ")
	fmt.Println(string(pretty))
}

func runPatch(ctx context.Context, c *gateway.TasksClient, tasklistID, taskID, title, notes, statusStr, dueStr string) {
	fields := gateway.PatchFields{}
	if title != "" {
		fields.SetTitle = true
		fields.Title = title
	}
	if notes != "" {
		fields.SetNotes = true
		fields.Notes = notes
	}
	if statusStr != "" {
		fields.SetStatus = true
		fields.Status = gateway.TaskStatus(statusStr)
	}
	if dueStr != "" {
		due, err := parseDueFlag(dueStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad -due:", err)
			os.Exit(2)
		}
		fields.SetDue = true
		fields.Due = due
	}
	task, err := c.PatchTask(ctx, tasklistID, taskID, fields, "")
	if errors.Is(err, gateway.ErrPreconditionFailed) {
		fmt.Fprintln(os.Stderr, "412 precondition failed (caller should pull-and-retry)")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "patch:", err)
		os.Exit(1)
	}
	fmt.Printf("patched task id=%s etag=%s updated=%s\n", task.ID, task.Etag, task.Updated.Format(time.RFC3339Nano))
}

func runDelete(ctx context.Context, c *gateway.TasksClient, tasklistID, taskID string) {
	if err := c.DeleteTask(ctx, tasklistID, taskID); err != nil {
		fmt.Fprintln(os.Stderr, "delete:", err)
		os.Exit(1)
	}
	fmt.Printf("deleted task %s/%s\n", tasklistID, taskID)
}

// runVerifyPositionMonotonic inserts -count tasks back-to-back and
// fetches the tasklist; verifies the `position` strings strictly
// increase in the order Google returns them. Catches a regression
// where Google would re-issue collisions on near-instant inserts.
func runVerifyPositionMonotonic(ctx context.Context, c *gateway.TasksClient, tasklistID string, count int) {
	if count < 2 {
		fmt.Fprintln(os.Stderr, "-count must be >= 2")
		os.Exit(2)
	}
	fmt.Printf("inserting %d tasks into %s ...\n", count, tasklistID)
	insertedIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		title := fmt.Sprintf("position-monotonic-test-%d-%d", time.Now().UnixNano(), i)
		t, err := c.InsertTask(ctx, tasklistID, title, "tasks-debug verify-position-monotonic", gateway.TaskStatusNeedsAction, time.Time{}, "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "insert:", err)
			os.Exit(1)
		}
		insertedIDs = append(insertedIDs, t.ID)
	}

	// Fetch and look at positions.
	all := fetchAll(ctx, c, tasklistID)
	byID := make(map[string]gateway.Task, len(all))
	for _, t := range all {
		byID[t.ID] = t
	}
	positions := make([]string, 0, len(insertedIDs))
	for _, id := range insertedIDs {
		t, ok := byID[id]
		if !ok {
			fmt.Fprintf(os.Stderr, "inserted task %s missing from list\n", id)
			os.Exit(1)
		}
		positions = append(positions, t.Position)
	}
	fmt.Println("inserted positions (in insert order):")
	for i, p := range positions {
		fmt.Printf("  [%d] %s  (id=%s)\n", i, p, insertedIDs[i])
	}

	// Sort a copy and compare to natural insert order — but Google
	// returns "newest at top" by default, so insert order maps to
	// REVERSE position order. Verify strictly monotonic in either
	// direction.
	sortedAsc := append([]string(nil), positions...)
	sort.Strings(sortedAsc)
	allAsc := stringsEqual(sortedAsc, positions)
	allDesc := stringsEqualReversed(sortedAsc, positions)
	if !allAsc && !allDesc {
		fmt.Fprintln(os.Stderr, "FAIL: positions not strictly monotonic in either direction")
		os.Exit(1)
	}
	fmt.Println("PASS: positions are strictly monotonic")

	// Cleanup.
	for _, id := range insertedIDs {
		_ = c.DeleteTask(ctx, tasklistID, id)
	}
}

// runVerifyUpdatedMinBoundary inserts a task, captures its `updated`
// timestamp, immediately calls tasks.list with that timestamp as
// updatedMin, and reports whether the inserted task is included
// (inclusive) or excluded (exclusive). Per the plan's "Boundary
// semantics" section.
func runVerifyUpdatedMinBoundary(ctx context.Context, c *gateway.TasksClient, tasklistID string) {
	title := fmt.Sprintf("updatedmin-boundary-%d", time.Now().UnixNano())
	fmt.Printf("inserting probe task %q ...\n", title)
	probe, err := c.InsertTask(ctx, tasklistID, title, "tasks-debug verify-updatedmin-boundary", gateway.TaskStatusNeedsAction, time.Time{}, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "insert probe:", err)
		os.Exit(1)
	}
	fmt.Printf("probe task id=%s updated=%s\n", probe.ID, probe.Updated.Format(time.RFC3339Nano))
	defer func() { _ = c.DeleteTask(ctx, tasklistID, probe.ID) }()

	// Wait briefly to give Google's index a moment.
	time.Sleep(2 * time.Second)

	page, err := c.ListTasks(ctx, tasklistID, probe.Updated, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "list with updatedMin:", err)
		os.Exit(1)
	}
	included := false
	for _, t := range page.Tasks {
		if t.ID == probe.ID {
			included = true
			break
		}
	}
	if included {
		fmt.Println("RESULT: updatedMin is INCLUSIVE — the probe task IS in the result")
		fmt.Println("(plan's 1s safety buffer is unnecessary; cursor can advance to max(updated))")
	} else {
		fmt.Println("RESULT: updatedMin is EXCLUSIVE — the probe task is NOT in the result")
		fmt.Println("(plan's 1s safety buffer is needed; cursor advance to max(updated)-1s is correct)")
	}
}

// runVerifyPKCEFlow generates a PKCE verifier+challenge and a state
// token, then prints the auth URL. Operator can copy/paste this URL
// into a browser to verify Google accepts the parameters.
func runVerifyPKCEFlow(clientID, redirectURI string) {
	if clientID == "" {
		clientID = os.Getenv("TASKS_CLIENT_ID")
	}
	if clientID == "" {
		fmt.Fprintln(os.Stderr, "set TASKS_CLIENT_ID (or pass --client-id) for verify-pkce-flow")
		os.Exit(2)
	}
	verifier, err := gateway.GeneratePKCEVerifier()
	if err != nil {
		fmt.Fprintln(os.Stderr, "verifier:", err)
		os.Exit(1)
	}
	challenge := gateway.PKCEChallengeS256(verifier)
	state, err := gateway.GenerateStateToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, "state:", err)
		os.Exit(1)
	}
	authURL, err := gateway.BuildAuthURL(gateway.DefaultGoogleAuthURL, clientID, redirectURI, gateway.TasksScope, state, challenge)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build auth url:", err)
		os.Exit(1)
	}
	fmt.Println("PKCE verifier:")
	fmt.Println("  ", verifier)
	fmt.Println("PKCE challenge (S256):")
	fmt.Println("  ", challenge)
	fmt.Println("State token:")
	fmt.Println("  ", state)
	fmt.Println()
	fmt.Println("Authorization URL (open in a browser):")
	fmt.Println("  ", authURL)
	fmt.Println()
	fmt.Println("Expected callback shape:")
	fmt.Println("  ", redirectURI, "?code=...&state=", state, "&iss=", gateway.GoogleIssuer)
	fmt.Println()
	fmt.Println("On callback, exchange code with this verifier (POST oauth2/token):")
	fmt.Println("    code, code_verifier:", verifier, ", grant_type=authorization_code")
}

// fetchAll pulls every page of tasks.list (no updatedMin) and returns
// the flat slice.
func fetchAll(ctx context.Context, c *gateway.TasksClient, tasklistID string) []gateway.Task {
	var out []gateway.Task
	pageToken := ""
	for {
		page, err := c.ListTasks(ctx, tasklistID, time.Time{}, pageToken)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetchAll:", err)
			os.Exit(1)
		}
		out = append(out, page.Tasks...)
		if page.NextPageToken == "" {
			return out
		}
		pageToken = page.NextPageToken
	}
}

func dumpTasks(tasks []gateway.Task) {
	for _, t := range tasks {
		notesPreview := strings.ReplaceAll(t.Notes, "\n", "\\n")
		if len(notesPreview) > 60 {
			notesPreview = notesPreview[:60] + "..."
		}
		fmt.Printf("  id=%s status=%s pos=%s title=%q notes=%q updated=%s\n",
			t.ID, t.Status, t.Position, t.Title, notesPreview, t.Updated.Format(time.RFC3339))
	}
}

func parseDueFlag(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringsEqualReversed(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[len(b)-1-i] {
			return false
		}
	}
	return true
}
