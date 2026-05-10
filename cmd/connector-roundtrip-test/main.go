// connector-roundtrip-test exercises the wiki ↔ Google Tasks sync
// loop end-to-end against a deployed wiki and the real Tasks API. It
// is the regression harness for the "Tasks 412 destroys local edit"
// class of bugs — every scenario is one round-trip with a hard
// timeout, a pass/fail line on stdout, and an aggregate exit code.
//
// Configuration via env (preferred) or flags:
//
//	WIKI_URL                 wiki base URL (default: https://wiki.monster-orfe.ts.net)
//	WIKI_PAGE                page identifier carrying the test checklist
//	WIKI_LIST                checklist name on that page
//	TASKS_CLIENT_ID          Google OAuth client ID
//	TASKS_CLIENT_SECRET      Google OAuth client secret
//	TASKS_REFRESH_TOKEN      refresh token for the connected account
//	TASKS_LIST_ID            Google Tasks tasklist id paired with WIKI_PAGE/WIKI_LIST
//
// Scenarios (each takes-or-times-out):
//
//  1. wiki AddItem → wait → Tasks has new task            (timeout 30s)
//  2. wiki ToggleItem → wait → Tasks task status flipped   (timeout 30s)
//  3. Tasks PatchTask → wait → wiki ListItems shows change (timeout 60s)
//  4. wiki DeleteItem → wait → Tasks task soft-deleted     (timeout 30s)
//
// Exit code: 0 if every scenario passed, 1 if any failed, 2 on
// configuration error.
//
//revive:disable:deep-exit
//revive:disable:unhandled-error
//revive:disable:cognitive-complexity
//revive:disable:cyclomatic
//revive:disable:function-length
//revive:disable:add-constant
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks/gateway"
)

const (
	defaultWikiURL = "https://wiki.monster-orfe.ts.net"

	// scenarioTimeoutShort is the wait budget for wiki→Tasks
	// (outbound debounce + Tasks PATCH round-trip). The Tasks
	// connector debounces ~10s and the cron runs every 30s.
	scenarioTimeoutShort = 30 * time.Second

	// scenarioTimeoutLong is the wait budget for Tasks→wiki
	// (Tasks-side change must be polled by the inbound apply path,
	// which runs every 30s).
	scenarioTimeoutLong = 60 * time.Second

	// pollInterval is how often we re-check the destination side.
	pollInterval = 2 * time.Second
)

type result struct {
	scenario string
	passed   bool
	detail   string
	duration time.Duration
}

func main() {
	wikiURL := flag.String("wiki-url", getenvDefault("WIKI_URL", defaultWikiURL), "wiki base URL")
	page := flag.String("page", os.Getenv("WIKI_PAGE"), "wiki page identifier (env WIKI_PAGE)")
	listName := flag.String("list", os.Getenv("WIKI_LIST"), "wiki checklist name (env WIKI_LIST)")
	tasksListID := flag.String("tasklist", os.Getenv("TASKS_LIST_ID"), "Google Tasks tasklist id (env TASKS_LIST_ID)")
	flag.Parse()

	clientID := os.Getenv("TASKS_CLIENT_ID")
	clientSecret := os.Getenv("TASKS_CLIENT_SECRET")
	refreshToken := os.Getenv("TASKS_REFRESH_TOKEN")

	if *page == "" || *listName == "" || *tasksListID == "" {
		fmt.Fprintln(os.Stderr, "set WIKI_PAGE, WIKI_LIST, and TASKS_LIST_ID (or pass via flags)")
		os.Exit(2)
	}
	if clientID == "" || clientSecret == "" || refreshToken == "" {
		fmt.Fprintln(os.Stderr, "set TASKS_CLIENT_ID, TASKS_CLIENT_SECRET, TASKS_REFRESH_TOKEN")
		os.Exit(2)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
		Timeout: 30 * time.Second,
	}
	checklistClient := apiv1connect.NewChecklistServiceClient(httpClient, *wikiURL)

	tokenStore := gateway.NewStaticRefreshTokenStore(refreshToken)
	refreshClient, err := gateway.NewRefreshClient(httpClient, gateway.DefaultGoogleTokenURL, clientID, clientSecret, tokenStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build refresh client:", err)
		os.Exit(2)
	}
	tasksClient, err := gateway.NewTasksClient(httpClient, gateway.DefaultTasksBaseURL, refreshClient)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build tasks client:", err)
		os.Exit(2)
	}

	ctx := context.Background()

	results := make([]result, 0, 4)
	results = append(results, scenarioWikiAddPropagatesToTasks(ctx, checklistClient, tasksClient, *page, *listName, *tasksListID))
	results = append(results, scenarioWikiTogglePropagatesToTasks(ctx, checklistClient, tasksClient, *page, *listName, *tasksListID))
	results = append(results, scenarioTasksPatchPropagatesToWiki(ctx, checklistClient, tasksClient, *page, *listName, *tasksListID))
	results = append(results, scenarioWikiDeletePropagatesToTasks(ctx, checklistClient, tasksClient, *page, *listName, *tasksListID))

	failed := 0
	fmt.Println("\n==== ROUND-TRIP HARNESS RESULTS ====")
	for _, r := range results {
		marker := "PASS"
		if !r.passed {
			marker = "FAIL"
			failed++
		}
		fmt.Printf("[%s] %s (%.1fs) — %s\n", marker, r.scenario, r.duration.Seconds(), r.detail)
	}
	fmt.Printf("==== %d/%d passed ====\n", len(results)-failed, len(results))

	if failed > 0 {
		os.Exit(1)
	}
}

// scenarioWikiAddPropagatesToTasks: wiki AddItem → Tasks has new task within timeout.
func scenarioWikiAddPropagatesToTasks(ctx context.Context, w apiv1connect.ChecklistServiceClient, t *gateway.TasksClient, page, listName, tasksListID string) result {
	start := time.Now()
	scenario := "wiki AddItem → Tasks"
	uniqueText := fmt.Sprintf("[roundtrip] add %s", time.Now().UTC().Format(time.RFC3339Nano))

	addResp, err := w.AddItem(ctx, connect.NewRequest(&apiv1.AddItemRequest{
		Page:     page,
		ListName: listName,
		Text:     uniqueText,
	}))
	if err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("AddItem failed: %v", err), duration: time.Since(start)}
	}
	uid := addResp.Msg.GetItem().GetUid()

	// Poll Tasks for a task whose title contains uniqueText (the
	// outbound translator will encode tags into the title).
	deadline := time.Now().Add(scenarioTimeoutShort)
	for time.Now().Before(deadline) {
		tasks, listErr := listAllTasks(ctx, t, tasksListID)
		if listErr == nil {
			for _, task := range tasks {
				if !task.Deleted && strings.Contains(task.Title, uniqueText) {
					return result{
						scenario: scenario,
						passed:   true,
						detail:   fmt.Sprintf("uid=%s task=%s", uid, task.ID),
						duration: time.Since(start),
					}
				}
			}
		}
		time.Sleep(pollInterval)
	}
	return result{scenario: scenario, passed: false, detail: fmt.Sprintf("uid=%s never appeared on Tasks within %s", uid, scenarioTimeoutShort), duration: time.Since(start)}
}

// scenarioWikiTogglePropagatesToTasks: wiki ToggleItem → Tasks task status flipped within timeout.
func scenarioWikiTogglePropagatesToTasks(ctx context.Context, w apiv1connect.ChecklistServiceClient, t *gateway.TasksClient, page, listName, tasksListID string) result {
	start := time.Now()
	scenario := "wiki ToggleItem → Tasks"

	// Find an existing task on the wiki side. Pick the first.
	listResp, err := w.ListItems(ctx, connect.NewRequest(&apiv1.ListItemsRequest{Page: page, ListName: listName}))
	if err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("ListItems failed: %v", err), duration: time.Since(start)}
	}
	items := listResp.Msg.GetChecklist().GetItems()
	if len(items) == 0 {
		return result{scenario: scenario, passed: false, detail: "no items on wiki list", duration: time.Since(start)}
	}
	uid := items[0].GetUid()
	wantChecked := !items[0].GetChecked()

	if _, err := w.ToggleItem(ctx, connect.NewRequest(&apiv1.ToggleItemRequest{Page: page, ListName: listName, Uid: uid})); err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("ToggleItem failed: %v", err), duration: time.Since(start)}
	}

	wantStatus := gateway.TaskStatusNeedsAction
	if wantChecked {
		wantStatus = gateway.TaskStatusCompleted
	}

	deadline := time.Now().Add(scenarioTimeoutShort)
	for time.Now().Before(deadline) {
		tasks, listErr := listAllTasks(ctx, t, tasksListID)
		if listErr == nil {
			for _, task := range tasks {
				if strings.Contains(task.Notes, uid) && task.Status == wantStatus {
					return result{
						scenario: scenario,
						passed:   true,
						detail:   fmt.Sprintf("uid=%s status=%s", uid, wantStatus),
						duration: time.Since(start),
					}
				}
			}
		}
		time.Sleep(pollInterval)
	}
	return result{scenario: scenario, passed: false, detail: fmt.Sprintf("uid=%s never reached status=%s on Tasks within %s", uid, wantStatus, scenarioTimeoutShort), duration: time.Since(start)}
}

// scenarioTasksPatchPropagatesToWiki: edit a task via Tasks API → wiki ListItems reflects within timeout.
func scenarioTasksPatchPropagatesToWiki(ctx context.Context, w apiv1connect.ChecklistServiceClient, t *gateway.TasksClient, page, listName, tasksListID string) result {
	start := time.Now()
	scenario := "Tasks PatchTask → wiki"

	// Find a task on the Tasks side that has a wiki:uid marker, so
	// we know it's paired and the inbound apply will route the change.
	tasks, err := listAllTasks(ctx, t, tasksListID)
	if err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("Tasks list failed: %v", err), duration: time.Since(start)}
	}
	var pairedTask *gateway.Task
	for i := range tasks {
		if tasks[i].Deleted {
			continue
		}
		if strings.Contains(tasks[i].Notes, "wiki:uid=") {
			pairedTask = &tasks[i]
			break
		}
	}
	if pairedTask == nil {
		return result{scenario: scenario, passed: false, detail: "no paired task with wiki:uid marker found", duration: time.Since(start)}
	}

	wantStatus := gateway.TaskStatusCompleted
	if pairedTask.Status == gateway.TaskStatusCompleted {
		wantStatus = gateway.TaskStatusNeedsAction
	}
	patch := gateway.PatchFields{SetStatus: true, Status: wantStatus}
	if _, err := t.PatchTask(ctx, tasksListID, pairedTask.ID, patch, pairedTask.Etag); err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("PatchTask failed: %v", err), duration: time.Since(start)}
	}

	wantWikiChecked := wantStatus == gateway.TaskStatusCompleted

	// Extract uid from notes for wiki-side lookup.
	uid := extractWikiUID(pairedTask.Notes)

	deadline := time.Now().Add(scenarioTimeoutLong)
	for time.Now().Before(deadline) {
		listResp, listErr := w.ListItems(ctx, connect.NewRequest(&apiv1.ListItemsRequest{Page: page, ListName: listName}))
		if listErr == nil {
			for _, item := range listResp.Msg.GetChecklist().GetItems() {
				if item.GetUid() == uid && item.GetChecked() == wantWikiChecked {
					return result{
						scenario: scenario,
						passed:   true,
						detail:   fmt.Sprintf("uid=%s checked=%t", uid, wantWikiChecked),
						duration: time.Since(start),
					}
				}
			}
		}
		time.Sleep(pollInterval)
	}
	return result{scenario: scenario, passed: false, detail: fmt.Sprintf("wiki uid=%s never reached checked=%t within %s", uid, wantWikiChecked, scenarioTimeoutLong), duration: time.Since(start)}
}

// scenarioWikiDeletePropagatesToTasks: wiki DeleteItem → Tasks task gone within timeout.
func scenarioWikiDeletePropagatesToTasks(ctx context.Context, w apiv1connect.ChecklistServiceClient, t *gateway.TasksClient, page, listName, tasksListID string) result {
	start := time.Now()
	scenario := "wiki DeleteItem → Tasks"

	// Add a fresh item, wait for it to appear on Tasks, then delete it.
	uniqueText := fmt.Sprintf("[roundtrip] delete %s", time.Now().UTC().Format(time.RFC3339Nano))
	addResp, err := w.AddItem(ctx, connect.NewRequest(&apiv1.AddItemRequest{Page: page, ListName: listName, Text: uniqueText}))
	if err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("AddItem failed: %v", err), duration: time.Since(start)}
	}
	uid := addResp.Msg.GetItem().GetUid()

	// Wait for it to appear on Tasks.
	var taskID string
	deadline := time.Now().Add(scenarioTimeoutShort)
	for time.Now().Before(deadline) && taskID == "" {
		tasks, listErr := listAllTasks(ctx, t, tasksListID)
		if listErr == nil {
			for _, task := range tasks {
				if !task.Deleted && strings.Contains(task.Notes, uid) {
					taskID = task.ID
					break
				}
			}
		}
		if taskID == "" {
			time.Sleep(pollInterval)
		}
	}
	if taskID == "" {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("uid=%s never propagated to Tasks before delete", uid), duration: time.Since(start)}
	}

	// Delete from wiki.
	if _, err := w.DeleteItem(ctx, connect.NewRequest(&apiv1.DeleteItemRequest{Page: page, ListName: listName, Uid: uid})); err != nil {
		return result{scenario: scenario, passed: false, detail: fmt.Sprintf("DeleteItem failed: %v", err), duration: time.Since(start)}
	}

	// Wait for Tasks to no longer return the task as alive.
	deadline = time.Now().Add(scenarioTimeoutShort)
	for time.Now().Before(deadline) {
		tasks, listErr := listAllTasks(ctx, t, tasksListID)
		if listErr == nil {
			alive := false
			for _, task := range tasks {
				if task.ID == taskID && !task.Deleted {
					alive = true
					break
				}
			}
			if !alive {
				return result{scenario: scenario, passed: true, detail: fmt.Sprintf("uid=%s task=%s gone", uid, taskID), duration: time.Since(start)}
			}
		}
		time.Sleep(pollInterval)
	}
	return result{scenario: scenario, passed: false, detail: fmt.Sprintf("uid=%s task=%s still alive on Tasks after %s", uid, taskID, scenarioTimeoutShort), duration: time.Since(start)}
}

// listAllTasks paginates through all tasks in the tasklist. The
// gateway's ListTasks always passes showHidden + showDeleted + show
// Completed so soft-deleted items remain visible.
func listAllTasks(ctx context.Context, t *gateway.TasksClient, tasklistID string) ([]gateway.Task, error) {
	var out []gateway.Task
	pageToken := ""
	for {
		resp, err := t.ListTasks(ctx, tasklistID, time.Time{}, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Tasks...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return out, nil
}

// extractWikiUID returns the ULID following "wiki:uid=" in notes, or "" if absent.
func extractWikiUID(notes string) string {
	idx := strings.Index(notes, "wiki:uid=")
	if idx < 0 {
		return ""
	}
	rest := notes[idx+len("wiki:uid="):]
	end := strings.IndexAny(rest, "\n \t")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func getenvDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
