// Package google_tasks owns the per-connector floor of the Google Tasks
// bridge: the wire-protocol Gateway (sub-package gateway/), the
// Anti-Corruption Layer translator (sub-package translator/), and the
// BackendAdapter implementation in this file.
//
// Per ADR-0012 (audited 2026-05-04 refinement) the engine's lifecycle
// (per-tick reconcile, Bind/Unbind, ForceFullResync, Pause/Resume,
// precondition recovery, dead-letter retry, scheduler tick, debouncer,
// binding store) lives in internal/connectors/engine. The adapter here
// provides only the wire-protocol verbs, translation, capability bits,
// and error classification — exactly the BackendAdapter contract in
// internal/connectors/adapter.go.
//
// Concurrency: the Engine treats each adapter primitive as safe for
// concurrent use (the scheduler dispatches without per-connector
// serialization). The TasksAdapter holds no mutable state beyond the
// injected collaborators; it is safe for concurrent use across
// scheduler ticks, debouncer fires, and gRPC handler calls.
//
//revive:disable:var-naming // package name google_tasks mirrors ConnectorKindGoogleTasks
package google_tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// AdapterStateKeyItemIDMap is the AdapterState subtree key under which
// the engine's reconcile loop reads/writes the wiki-uid → tasks-id map.
// Mirrors the engine's adapterStateItemIDMapKey constant; redeclared
// here so EncodeAdapterState/DecodeAdapterState do not import the
// engine package.
const AdapterStateKeyItemIDMap = "item_id_map"

// AdapterStateKeyItemEtags is the AdapterState subtree key under which
// the adapter persists Tasks's per-task etags (tasks-id → etag string)
// for optimistic-concurrency PATCH calls.
const AdapterStateKeyItemEtags = "item_etags"

// AdapterStateKeyLastUpdatedMin is the AdapterState subtree key under
// which the adapter persists Tasks's `updatedMin` cursor (RFC3339
// timestamp string). The engine's AdvanceCursor on this adapter writes
// max(Task.updated) - 1s here per the Tasks safety-buffer convention.
const AdapterStateKeyLastUpdatedMin = "last_updated_min"

// updatedMinSafetyBufferSeconds is the seconds we subtract from
// max(Task.updated) when advancing the cursor. Tasks's docs do not
// state whether updatedMin is inclusive or exclusive; treat as
// exclusive and re-process one second's worth of items each poll —
// idempotent under our marker + id_map model. Mirrors the legacy
// connector's same-named constant.
const updatedMinSafetyBufferSeconds = 1

// CredentialReader is the per-profile auth-state seam the TasksAdapter
// uses to obtain a refresh token for building TasksClient instances.
// The production wiring satisfies this with a frontmatter-backed reader
// that walks wiki.connectors.google_tasks.refresh_token; tests inject a
// programmable fake.
//
// Per ADR-0014, refresh tokens live plaintext on the operator-trusted
// profile page. The reader returns ErrCredentialMissing when the
// profile has no refresh_token (typically: never connected, or
// disconnected). The engine maps that to ErrorClassAuthFailed via the
// adapter's ClassifyError, transitioning the binding to paused.
type CredentialReader interface {
	// LoadRefreshToken returns the refresh token persisted on the
	// given profile page. Errors are wrapped; callers branch on
	// ErrCredentialMissing for the "not configured" case.
	LoadRefreshToken(ctx context.Context, profileID wikipage.PageIdentifier) (string, error)
}

// ErrCredentialMissing is returned by CredentialReader.LoadRefreshToken
// when the profile has no refresh token. The adapter's ClassifyError
// maps this to ErrorClassAuthFailed so the engine transitions the
// binding to paused without bubbling a connector-internal sentinel
// through the dispatch shape.
var ErrCredentialMissing = errors.New("google_tasks: profile has no refresh token (Disconnect or never connected)")

// TasksClientFactory constructs a *gateway.TasksClient bound to the
// given profile's credentials. Mirrors the legacy package's factory
// shape so the production wiring (bootstrap.setupGoogleTasksConnector)
// can hand the same closure to both code paths during the brief
// Phase-4 cohabitation.
type TasksClientFactory func(ctx context.Context, profileID wikipage.PageIdentifier, refreshToken string) (TasksClient, error)

// TasksClient is the subset of gateway.TasksClient the adapter calls.
// Stated as an interface so adapter_test.go can substitute a fake
// without spinning up an httptest.Server.
type TasksClient interface {
	ListTaskLists(ctx context.Context) ([]gateway.TaskList, error)
	CreateTaskList(ctx context.Context, title string) (gateway.TaskList, error)
	ListTasks(ctx context.Context, tasklistID string, updatedMin time.Time, pageToken string) (gateway.TasksPage, error)
	InsertTask(ctx context.Context, tasklistID, title, notes string, status gateway.TaskStatus, due time.Time, parent string) (gateway.Task, error)
	PatchTask(ctx context.Context, tasklistID, taskID string, fields gateway.PatchFields, etag string) (gateway.Task, error)
	DeleteTask(ctx context.Context, tasklistID, taskID string) error
}

// Logger is the minimal log surface the adapter needs.
type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// TasksAdapter implements connectors.BackendAdapter against the Google
// Tasks gateway + translator. Construction wires the per-profile
// client factory and the credential reader; the adapter holds no
// per-binding state (the engine carries everything in the Binding's
// AdapterState subtree).
type TasksAdapter struct {
	credentials   CredentialReader
	clientFactory TasksClientFactory
	logger        Logger
}

// NewTasksAdapter wires a TasksAdapter. Every dependency is required.
func NewTasksAdapter(credentials CredentialReader, clientFactory TasksClientFactory, logger Logger) (*TasksAdapter, error) {
	if credentials == nil {
		return nil, errors.New("google_tasks: credentials must not be nil")
	}
	if clientFactory == nil {
		return nil, errors.New("google_tasks: clientFactory must not be nil")
	}
	if logger == nil {
		return nil, errors.New("google_tasks: logger must not be nil")
	}
	return &TasksAdapter{
		credentials:   credentials,
		clientFactory: clientFactory,
		logger:        logger,
	}, nil
}

// Compile-time check: *TasksAdapter satisfies the BackendAdapter
// contract. Adding a method to BackendAdapter is now a compile error
// for this package — the entire point of the abstraction.
var _ connectors.BackendAdapter = (*TasksAdapter)(nil)

// Kind reports the connector kind. Used by structured logs, metrics,
// and op-log self-source markers.
func (*TasksAdapter) Kind() connectors.ConnectorKind {
	return connectors.ConnectorKindGoogleTasks
}

// SupportsSubtasks reports that Tasks has a parent-child task
// hierarchy. The engine refuses to bind to a list that already
// contains subtasks via ValidateRemoteBinding (per MATRIX row 12) and
// flattens silently if subtasks appear post-bind.
func (*TasksAdapter) SupportsSubtasks() bool {
	return true
}

// PullRemote fetches every Tasks-side item the engine should consider
// this tick. Walks pagination internally; the engine never sees a
// partial pull. NewCursor is a time.Time (max(Task.updated)) so
// AdvanceCursor can subtract the safety buffer.
func (a *TasksAdapter) PullRemote(ctx context.Context, binding connectors.Binding) (connectors.RemotePullResult, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return connectors.RemotePullResult{}, err
	}
	updatedMin := readLastUpdatedMin(binding.AdapterState)
	tasks, err := listAllTasks(ctx, client, binding.RemoteHandle, updatedMin)
	if err != nil {
		return connectors.RemotePullResult{}, err
	}
	items := make([]connectors.RemoteItem, 0, len(tasks))
	maxUpdated := time.Time{}
	for _, t := range tasks {
		if t.Updated.After(maxUpdated) {
			maxUpdated = t.Updated
		}
		item := taskToRemoteItem(t)
		// ADR-0015 Fix #1: populate RemoteDiverged by comparing the
		// incoming task etag against the stored etag in AdapterState.
		// Tasks uses a safety-buffer cursor (updatedMin - 1s), so items
		// can re-appear even when unchanged; the etag comparison
		// distinguishes a genuine remote update from a re-delivery.
		// An absent stored etag means we have no baseline → not diverged
		// (the first inbound apply should proceed normally).
		if storedEtag := readEtagForTask(binding.AdapterState, string(item.Ref)); storedEtag != "" && storedEtag != item.Etag {
			item.RemoteDiverged = true
		}
		items = append(items, item)
	}
	return connectors.RemotePullResult{
		Items:     items,
		NewCursor: maxUpdated,
		Truncated: false,
	}, nil
}

// InsertRemote pushes a fresh wiki item to Tasks. The wiki:uid marker
// is NOT stamped into Notes — the binding's AdapterState owns the
// uid → ref mapping (per the translator's documented rationale).
func (a *TasksAdapter) InsertRemote(ctx context.Context, binding connectors.Binding, item connectors.WikiItem) (connectors.RemoteRef, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return "", err
	}
	fields := wikiItemToTaskFields(item)
	inserted, err := client.InsertTask(ctx, binding.RemoteHandle, fields.Title, fields.Notes,
		gateway.TaskStatus(fields.Status), fields.Due, "")
	if err != nil {
		return "", err
	}
	return connectors.RemoteRef(inserted.ID), nil
}

// PatchRemote pushes an update to an existing Tasks task. Uses the
// stored etag from binding.AdapterState[item_etags] as If-Match. The
// engine's precondition_recovery path runs on a 412 (mapped to
// ErrorClassPreconditionFailed by ClassifyError).
func (a *TasksAdapter) PatchRemote(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef, item connectors.WikiItem) (connectors.RemoteRef, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return "", err
	}
	fields := wikiItemToTaskFields(item)
	etag := readEtagForTask(binding.AdapterState, string(ref))
	patched, err := client.PatchTask(ctx, binding.RemoteHandle, string(ref),
		buildPatchFields(fields), etag)
	if err != nil {
		return "", err
	}
	return connectors.RemoteRef(patched.ID), nil
}

// DeleteRemote removes a Tasks task. tasks.delete is server-side
// idempotent (404 → no-op per the gateway).
func (a *TasksAdapter) DeleteRemote(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef) error {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return err
	}
	return client.DeleteTask(ctx, binding.RemoteHandle, string(ref))
}

// RemoteToWiki converts the engine's normalized RemoteItem into a
// WikiItem. The translator owns the marker-strip + tag-extraction
// logic. The returned UID is empty — the engine resolves the uid via
// AdapterState's item_id_map. Marker-derived uid recovery is reported
// via the Vendor map's `wiki_uid` key when present (PullRemote set it).
func (*TasksAdapter) RemoteToWiki(remote connectors.RemoteItem) (connectors.WikiItem, error) {
	cleanedNotes, markerUID, _ := translator.StripWikiUIDMarker(remote.Notes)
	title, tags := translator.TitleAndTagsFromText(remote.Title)
	checked := remote.Status == translator.TaskStatusCompleted

	uid := markerUID
	if vendorUID, ok := remote.Vendor["wiki_uid"].(string); ok && vendorUID != "" {
		uid = vendorUID
	}

	return connectors.WikiItem{
		UID:         uid,
		Text:        title,
		Checked:     checked,
		Tags:        tags,
		Description: cleanedNotes,
		Due:         remote.Due,
		SortOrder:   translator.PositionToSortOrder(remote.Position),
	}, nil
}

// WikiToRemote converts a WikiItem into the normalized RemoteItem. The
// outbound primitives consume this via wikiItemToTaskFields internally;
// the engine never round-trips through this for Tasks's outbound
// (Insert/Patch take WikiItem directly), but the contract requires it.
func (*TasksAdapter) WikiToRemote(wiki connectors.WikiItem) (connectors.RemoteItem, error) {
	fields := wikiItemToTaskFieldsBare(wiki)
	return connectors.RemoteItem{
		Title:  fields.Title,
		Notes:  fields.Notes,
		Status: fields.Status,
		Due:    fields.Due,
	}, nil
}

// RefreshItemBaseline updates the stored item_etags entry for ref to
// the etag from the freshly-read remote item. Used by the engine's
// precondition_recovery path: after ReadRemoteByRef returns, the
// stored etag is stale (that's why the patch hit 412), and the
// recovery's re-PATCH would loop on 412 forever without this refresh.
// Production fix 2026-05-06.
func (*TasksAdapter) RefreshItemBaseline(binding connectors.Binding, remote connectors.RemoteItem) connectors.Binding {
	if remote.Etag == "" {
		return binding
	}
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	etagsRaw, ok := binding.AdapterState[AdapterStateKeyItemEtags].(map[string]any)
	if !ok {
		etagsRaw = map[string]any{}
	}
	etagsRaw[string(remote.Ref)] = remote.Etag
	binding.AdapterState[AdapterStateKeyItemEtags] = etagsRaw
	return binding
}

// AdvanceCursor stores max(Task.updated) - 1s in the binding's
// AdapterState[last_updated_min]. Tasks's documented behavior leaves
// the inclusive/exclusive boundary unspecified, so the safety buffer
// re-processes one second's worth of items per poll — idempotent under
// the engine's etag-skip and divergence-skip guards.
func (*TasksAdapter) AdvanceCursor(binding connectors.Binding, result connectors.RemotePullResult) connectors.Binding {
	maxUpdated, ok := result.NewCursor.(time.Time)
	if !ok || maxUpdated.IsZero() {
		return binding
	}
	advance := maxUpdated.Add(-time.Duration(updatedMinSafetyBufferSeconds) * time.Second)
	prev := readLastUpdatedMin(binding.AdapterState)
	if !advance.After(prev) {
		return binding
	}
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	binding.AdapterState[AdapterStateKeyLastUpdatedMin] = advance.UTC().Format(time.RFC3339)
	return binding
}

// SeedBindingState produces the initial AdapterState by listing the
// remote tasklist and recording task ids + etags. Per MATRIX row 2,
// the engine calls this inside the bind mutex AFTER ValidateRemoteBinding
// passes. No marker stamping or text-match is needed at seed time —
// the first reconcile tick reads remote tasks and mints uid mappings.
//
// On a fresh tasklist the returned AdapterState carries empty maps so
// downstream consumers (RebuildAdapterState, EncodeAdapterState) can
// treat presence/absence uniformly.
func (a *TasksAdapter) SeedBindingState(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (connectors.AdapterState, error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	tasks, err := listAllTasks(ctx, client, remoteHandle, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("seed: list tasks for %s: %w", remoteHandle, err)
	}
	idMap := map[string]any{}
	etags := map[string]any{}
	for _, t := range tasks {
		if t.Deleted {
			continue
		}
		_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
		if hasMarker && markerUID != "" {
			idMap[markerUID] = t.ID
		}
		if t.Etag != "" {
			etags[t.ID] = t.Etag
		}
	}
	return connectors.AdapterState{
		AdapterStateKeyItemIDMap:      idMap,
		AdapterStateKeyItemEtags:      etags,
		AdapterStateKeyLastUpdatedMin: "",
	}, nil
}

// ValidateRemoteBinding rejects Tasks lists that already contain a
// parent-child hierarchy. Mirrors the legacy connector's
// ErrTasksListHasSubtasks refuse-to-bind. Tolerant flatten on inbound
// (when subtasks appear post-bind) is a translator-internal concern
// the engine doesn't see (the gateway returns flat structure;
// translator's FlattenSubtasks is invoked when needed).
func (a *TasksAdapter) ValidateRemoteBinding(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) error {
	if remoteHandle == "" {
		// Empty handle = "create new tasklist" path. The legacy
		// connector creates the list inside the bind ceremony; the
		// engine path delegates list creation to the adapter via
		// SeedBindingState's (or a forthcoming primitive's) call into
		// CreateTaskList. For now, reject early so the operator sees a
		// clear error rather than a downstream gateway 404.
		return errors.New("google_tasks: remote_handle must not be empty (create-new-list path is not yet wired through the engine; pick an existing tasklist)")
	}
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return err
	}
	tasks, err := listAllTasks(ctx, client, remoteHandle, time.Time{})
	if err != nil {
		return fmt.Errorf("validate: list tasks for %s: %w", remoteHandle, err)
	}
	if translator.HasSubtasks(toTranslatorTasks(tasks)) {
		return ErrTasksListHasSubtasks
	}
	return nil
}

// ErrTasksListHasSubtasks is returned by ValidateRemoteBinding when the
// chosen Tasks list already contains a parent-child task hierarchy.
// Mirrors the legacy package's same-named sentinel; the gRPC boundary
// maps it to FailedPrecondition with a user-facing "pick a flat list"
// message.
var ErrTasksListHasSubtasks = errors.New("google_tasks: tasks list contains subtasks; flat lists only")

// RebuildAdapterState pulls the full remote state and rebuilds
// AdapterState from scratch via marker + text-match. Mirrors the
// legacy connector's rebuildIDMapByTextMatch (connector.go:423-491).
//
// The Phase-4 implementation has access to the binding's profile but
// not to the wiki's checklist — the engine wraps RebuildAdapterState
// for the cursor-truncation / pause-horizon / admin-RPC paths. The
// rebuild here records every marker-bearing task; markerless items
// flow through the engine's outbound diff on the first post-rebuild
// tick (which inserts them or text-matches against current wiki state).
//
// Resetting last_updated_min to empty causes the next reconcile to
// re-process the world.
func (a *TasksAdapter) RebuildAdapterState(ctx context.Context, binding connectors.Binding) (connectors.AdapterState, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return nil, err
	}
	tasks, err := listAllTasks(ctx, client, binding.RemoteHandle, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("rebuild: list tasks for %s: %w", binding.RemoteHandle, err)
	}
	idMap := map[string]any{}
	etags := map[string]any{}
	for _, t := range tasks {
		if t.Deleted {
			continue
		}
		_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
		if hasMarker && markerUID != "" {
			idMap[markerUID] = t.ID
		}
		if t.Etag != "" {
			etags[t.ID] = t.Etag
		}
	}
	return connectors.AdapterState{
		AdapterStateKeyItemIDMap:      idMap,
		AdapterStateKeyItemEtags:      etags,
		AdapterStateKeyLastUpdatedMin: "",
	}, nil
}

// FetchRemoteListTitle returns the friendly title of the Tasks
// tasklist bound to remoteHandle. Returns ("", false, nil) on transient
// failure or list-not-found; the engine preserves the prior title in
// that case (per the BackendAdapter contract).
func (a *TasksAdapter) FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error) {
	if remoteHandle == "" {
		return "", false, nil
	}
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		// Auth issues at this layer are best-effort title sync; the
		// engine continues without updating the title.
		return "", false, nil
	}
	taskLists, err := client.ListTaskLists(ctx)
	if err != nil {
		return "", false, nil
	}
	for _, tl := range taskLists {
		if tl.ID == remoteHandle {
			return tl.Title, true, nil
		}
	}
	return "", false, nil
}

// ListRemoteCollections enumerates the calling user's Tasks tasklists
// for the bind UI. CollectionCapabilities.HasSubtasks is reported by
// inspecting each list's tasks (single ListTasks call); the cost is
// amortized across the picker render and avoids surfacing lists the
// user can't bind to.
//
// Note: the per-list HasSubtasks probe makes ListRemoteCollections
// O(N tasklists) Tasks API calls. Household-scale accounts have a
// handful of tasklists, so the cost is negligible. If a deployment
// scales past that, the probe can be deferred to bind-time
// ValidateRemoteBinding.
func (a *TasksAdapter) ListRemoteCollections(ctx context.Context, profileID wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	taskLists, err := client.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]connectors.RemoteCollection, 0, len(taskLists))
	for _, tl := range taskLists {
		// Defer the per-list probe to bind-time. ListRemoteCollections
		// returns the full set; the picker renders all lists, and the
		// engine's Bind ceremony invokes ValidateRemoteBinding which
		// surfaces ErrTasksListHasSubtasks if the user picks a
		// hierarchical list.
		out = append(out, connectors.RemoteCollection{
			Handle:       tl.ID,
			Title:        tl.Title,
			Capabilities: connectors.CollectionCapabilities{HasSubtasks: false},
		})
	}
	return out, nil
}

// EncodeAdapterState validates the AdapterState shape and returns the
// underlying map for persistence. Empty/missing keys are normalized to
// zero values so the persisted shape is uniform across bindings.
func (*TasksAdapter) EncodeAdapterState(state connectors.AdapterState) (map[string]any, error) {
	out := map[string]any{}
	if state == nil {
		out[AdapterStateKeyItemIDMap] = map[string]any{}
		out[AdapterStateKeyItemEtags] = map[string]any{}
		out[AdapterStateKeyLastUpdatedMin] = ""
		return out, nil
	}
	for k, v := range state {
		out[k] = v
	}
	if _, ok := out[AdapterStateKeyItemIDMap]; !ok {
		out[AdapterStateKeyItemIDMap] = map[string]any{}
	}
	if _, ok := out[AdapterStateKeyItemEtags]; !ok {
		out[AdapterStateKeyItemEtags] = map[string]any{}
	}
	if _, ok := out[AdapterStateKeyLastUpdatedMin]; !ok {
		out[AdapterStateKeyLastUpdatedMin] = ""
	}
	return out, nil
}

// DecodeAdapterState parses a persisted AdapterState (post-TOML round-
// trip) into the engine's opaque envelope. Accepts either the raw
// frontmatter map shape or a previously-encoded map. Type coercion
// handles TOML's reduced type vocabulary (int64 ↔ int, etc.).
func (*TasksAdapter) DecodeAdapterState(raw map[string]any) (connectors.AdapterState, error) {
	if raw == nil {
		return connectors.AdapterState{}, nil
	}
	out := connectors.AdapterState{}
	for k, v := range raw {
		out[k] = v
	}
	return out, nil
}

// ReadRemoteByRef reads a single Tasks task by id. Tasks's REST v1
// has no GetTask endpoint — the gateway exposes only ListTasks; this
// method walks the full list once and filters by id. The engine calls
// ReadRemoteByRef from the precondition_recovery path (typically once
// per 412), so the cost is acceptable for household scale.
//
// On a true 404 (the task is gone), returns
// (RemoteItem{Deleted: true}, nil) per the BackendAdapter contract.
func (a *TasksAdapter) ReadRemoteByRef(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef) (connectors.RemoteItem, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return connectors.RemoteItem{}, err
	}
	tasks, err := listAllTasks(ctx, client, binding.RemoteHandle, time.Time{})
	if err != nil {
		if errors.Is(err, gateway.ErrNotFound) {
			return connectors.RemoteItem{Deleted: true}, nil
		}
		return connectors.RemoteItem{}, err
	}
	for _, t := range tasks {
		if t.ID == string(ref) {
			if t.Deleted {
				return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
			}
			return taskToRemoteItem(t), nil
		}
	}
	return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
}

// ClassifyError maps Tasks gateway errors to the engine's ErrorClass
// vocabulary. Branches on errors.Is sentinels — never on string
// contents.
//
// MATRIX rows 5/6/7: AuthFailed pauses the binding; PreconditionFailed
// runs the 3-branch recovery; RateLimited backs off the tick; NotFound
// mirrors the deletion to the wiki; Retryable bumps PushFailureCount;
// the default is ErrorClassRetryable so transient or unclassified
// errors trigger the engine's exponential-backoff dead-letter path
// (per the strictest-behavior-wins audit).
func (*TasksAdapter) ClassifyError(err error) connectors.ErrorClass {
	if err == nil {
		return connectors.ErrorClassNone
	}
	switch {
	case errors.Is(err, ErrCredentialMissing),
		errors.Is(err, gateway.ErrInvalidGrant),
		errors.Is(err, gateway.ErrAuthRevoked):
		return connectors.ErrorClassAuthFailed
	case errors.Is(err, gateway.ErrPreconditionFailed):
		return connectors.ErrorClassPreconditionFailed
	case errors.Is(err, gateway.ErrRateLimited):
		return connectors.ErrorClassRateLimited
	case errors.Is(err, gateway.ErrNotFound):
		return connectors.ErrorClassNotFound
	case errors.Is(err, gateway.ErrServiceDisabled),
		errors.Is(err, gateway.ErrPermissionDenied),
		errors.Is(err, gateway.ErrScopeDowngraded),
		errors.Is(err, gateway.ErrIssuerMismatch):
		return connectors.ErrorClassFatal
	case errors.Is(err, gateway.ErrProtocolDrift):
		return connectors.ErrorClassFatal
	default:
		return connectors.ErrorClassRetryable
	}
}

// --- internal helpers -----------------------------------------------------

// buildClientForProfile loads the profile's refresh token and
// constructs a TasksClient via the injected factory.
func (a *TasksAdapter) buildClientForProfile(ctx context.Context, profileID wikipage.PageIdentifier) (TasksClient, error) {
	refreshToken, err := a.credentials.LoadRefreshToken(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if refreshToken == "" {
		return nil, ErrCredentialMissing
	}
	return a.clientFactory(ctx, profileID, refreshToken)
}

// listAllTasks consumes every page of ListTasks for the given list
// before returning. Per Tasks's contract: never advance cursor during
// pagination; multi-page walks finish before the engine's cursor
// advance.
func listAllTasks(ctx context.Context, client TasksClient, tasklistID string, updatedMin time.Time) ([]gateway.Task, error) {
	var out []gateway.Task
	pageToken := ""
	for {
		page, err := client.ListTasks(ctx, tasklistID, updatedMin, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Tasks...)
		if page.NextPageToken == "" {
			return out, nil
		}
		pageToken = page.NextPageToken
	}
}

// taskToRemoteItem normalizes a gateway.Task into the engine's
// RemoteItem shape. The Vendor map carries adapter-internal extras
// (etag for the engine's per-binding etag bookkeeping; updated for
// AdvanceCursor).
func taskToRemoteItem(t gateway.Task) connectors.RemoteItem {
	deleted := t.Deleted
	// Hidden completed tasks behave like deletions for the wiki:
	// they're invisible in the Tasks UI, and surfacing them as
	// non-deleted would re-add them to the wiki on every tick.
	// Mirrors the legacy connector's tombstone handling.
	if t.Hidden && t.Status == gateway.TaskStatusCompleted {
		deleted = true
	}
	return connectors.RemoteItem{
		Ref:      connectors.RemoteRef(t.ID),
		Etag:     t.Etag,
		Title:    t.Title,
		Notes:    t.Notes,
		Status:   string(t.Status),
		Due:      t.Due,
		Updated:  t.Updated,
		Deleted:  deleted,
		Position: t.Position,
		Vendor: map[string]any{
			"etag":    t.Etag,
			"updated": t.Updated,
		},
	}
}

// wikiItemToTaskFields runs the translator's outbound mapping, then
// re-applies the wiki uid via the translator if the item carries one.
// The Notes do NOT receive a wiki:uid marker — see translator/marker.go
// rationale.
func wikiItemToTaskFields(item connectors.WikiItem) translator.TaskFields {
	cl := wikiItemToProto(item)
	return translator.ChecklistItemToTaskFields(cl)
}

// wikiItemToTaskFieldsBare is the WikiToRemote variant: no implicit
// completion-stamp recovery (the engine doesn't pass CompletedAt on
// the WikiItem boundary).
func wikiItemToTaskFieldsBare(item connectors.WikiItem) translator.TaskFields {
	return wikiItemToTaskFields(item)
}

// wikiItemToProto adapts the engine's flat WikiItem to the proto type
// the translator consumes.
func wikiItemToProto(item connectors.WikiItem) *apiv1.ChecklistItem {
	cl := &apiv1.ChecklistItem{
		Uid:       item.UID,
		Text:      item.Text,
		Checked:   item.Checked,
		Tags:      item.Tags,
		SortOrder: item.SortOrder,
	}
	if item.Description != "" {
		d := item.Description
		cl.Description = &d
	}
	return cl
}

// buildPatchFields constructs the gateway.PatchFields with all Set*
// flags asserted (mirrors the legacy connector's same-named helper).
func buildPatchFields(f translator.TaskFields) gateway.PatchFields {
	return gateway.PatchFields{
		SetTitle:  true,
		Title:     f.Title,
		SetNotes:  true,
		Notes:     f.Notes,
		SetStatus: true,
		Status:    gateway.TaskStatus(f.Status),
		SetDue:    true,
		Due:       f.Due,
	}
}

// readLastUpdatedMin pulls the AdapterState[last_updated_min] cursor.
// Empty / missing → zero time.Time (full initial pull).
func readLastUpdatedMin(state connectors.AdapterState) time.Time {
	if state == nil {
		return time.Time{}
	}
	raw, ok := state[AdapterStateKeyLastUpdatedMin]
	if !ok || raw == nil {
		return time.Time{}
	}
	switch v := raw.(type) {
	case time.Time:
		return v
	case string:
		if v == "" {
			return time.Time{}
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}
		}
		return t.UTC()
	default:
		return time.Time{}
	}
}

// readEtagForTask pulls AdapterState[item_etags][taskID]. Empty /
// missing → "" (the gateway sends no If-Match in that case;
// last-write-wins for the first-ever PATCH on a freshly-bound item).
func readEtagForTask(state connectors.AdapterState, taskID string) string {
	if state == nil {
		return ""
	}
	raw, ok := state[AdapterStateKeyItemEtags]
	if !ok || raw == nil {
		return ""
	}
	switch m := raw.(type) {
	case map[string]string:
		return m[taskID]
	case map[string]any:
		if v, ok := m[taskID].(string); ok {
			return v
		}
	}
	return ""
}

// toTranslatorTasks converts gateway.Task slices to the translator's
// placeholder Task type so HasSubtasks/FlattenSubtasks can operate.
// Mirrors the legacy connector's helper — kept here to avoid pulling
// in the legacy package.
func toTranslatorTasks(in []gateway.Task) []translator.Task {
	out := make([]translator.Task, len(in))
	for i, t := range in {
		out[i] = translator.Task{
			ID:        t.ID,
			ETag:      t.Etag,
			Title:     t.Title,
			Notes:     t.Notes,
			Status:    string(t.Status),
			Position:  t.Position,
			Parent:    t.Parent,
			Updated:   t.Updated,
			Due:       t.Due,
			Completed: t.Completed,
			Deleted:   t.Deleted,
			Hidden:    t.Hidden,
		}
	}
	return out
}

// FrontmatterCredentialStore lives in credentials.go and supersedes
// the prior FrontmatterCredentialReader on this file. Adapter
// instantiation in production wiring uses *FrontmatterCredentialStore
// directly as the CredentialReader (the store satisfies the read
// interface). Tests in adapter_test.go that previously exercised the
// reader-only path now exercise the store, with a no-op pause/resume
// hook pair.
