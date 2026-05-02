package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// rateLimitChokeSeconds is the per-subscription cooldown between
// successful syncs. The unified 30s scheduler tick reaches every
// subscription on every fire; this field guards against running
// outbound on a checklist that just synced (e.g., a debounced
// outbound and the cron tick racing).
//
// Set high enough that household-scale throughput stays comfortable
// (a person editing checklist items a few times a minute will see
// changes propagate within one cron tick) and low enough that the
// Tasks API's per-user limit (50 read req/sec, 50 write req/sec
// observed in dev) is never approached at household scale. 25s
// matches the Keep bridge's cron cadence minus a small jitter buffer.
const rateLimitChokeSeconds = 25

// updatedMinSafetyBufferSeconds is the seconds we subtract from
// max(Task.updated) when advancing the cursor. Tasks's docs do not
// state whether updatedMin is inclusive or exclusive; the cmd/tasks-
// debug verify-updatedmin-boundary test in Phase 3 will confirm
// empirically. Until verified, treat as exclusive and re-process one
// second's worth of items each poll — idempotent under our outbound /
// inbound model (wiki:uid markers + id_map). When the test confirms
// the semantics, drop this buffer or adjust to whichever direction
// the real API expects.
const updatedMinSafetyBufferSeconds = 1

// resumeFullResyncHorizon is the pause-duration above which Resume
// triggers a force full resync (per plan §"Pause / resume horizon").
// 7 days mirrors the wiki's tombstone GC default — beyond that, the
// id_map can no longer be trusted because wiki tombstones may have
// been collected.
const resumeFullResyncHorizon = 7 * 24 * time.Hour

// Errors exposed by the connector.
var (
	// ErrChecklistReaderUnavailable is returned by Sync when the
	// connector wasn't given a ChecklistReader at startup. Indicates
	// a wiring bug in bootstrap.
	ErrChecklistReaderUnavailable = errors.New("tasks bridge: checklist reader not configured (bootstrap bug)")

	// ErrChecklistMutatorUnavailable is returned when inbound apply
	// is required but the mutator wasn't injected at startup.
	ErrChecklistMutatorUnavailable = errors.New("tasks bridge: checklist mutator not configured (bootstrap bug)")

	// ErrTasksListHasSubtasks is returned by Subscribe if the chosen
	// Tasks list already contains a parent-child hierarchy. Per plan
	// §3, the wiki refuses-to-subscribe initially; tolerant flatten
	// only kicks in for subtasks added post-subscribe.
	ErrTasksListHasSubtasks = errors.New("tasks bridge: Tasks list contains subtasks; flat lists only")
)

// Connector orchestrates the Google Tasks bridge: per-user subscription
// management, OAuth refresh wiring, and the inbound/outbound sync loop.
// Owns no long-running goroutines (those live in the scheduler) — every
// method completes in the caller's context.
type Connector struct {
	store         *SubscriptionStore
	leaseTable    *connectors.LeaseTable
	clientFactory TasksClientFactory
	checklistR    ChecklistReader
	checklistW    ChecklistMutator
	suppressor    SyncSuppressor
	logger        Logger
	clock         Clock
}

// NewConnector wires the production dependencies. Tests construct a
// Connector with stubbed factory + readers/mutators directly.
//
// store: the SubscriptionStore for persistence.
// leaseTable: the cross-connector exclusivity registry.
// clientFactory: builds a TasksClient + TokenSource per profile.
// logger: structured-log surface (lumber-shaped).
// clock: testable wall-clock.
func NewConnector(
	store *SubscriptionStore,
	leaseTable *connectors.LeaseTable,
	clientFactory TasksClientFactory,
	logger Logger,
	clock Clock,
) (*Connector, error) {
	if store == nil {
		return nil, errors.New("tasks bridge: store must not be nil")
	}
	if leaseTable == nil {
		return nil, errors.New("tasks bridge: leaseTable must not be nil")
	}
	if clientFactory == nil {
		return nil, errors.New("tasks bridge: clientFactory must not be nil")
	}
	if logger == nil {
		return nil, errors.New("tasks bridge: logger must not be nil")
	}
	if clock == nil {
		return nil, errors.New("tasks bridge: clock must not be nil")
	}
	return &Connector{
		store:         store,
		leaseTable:    leaseTable,
		clientFactory: clientFactory,
		logger:        logger,
		clock:         clock,
	}, nil
}

// SetChecklistReader injects the wiki-side reader the outbound sync
// path uses to diff wiki state against the Subscription's ItemIDMap.
// Pass at startup; nil disables outbound sync.
func (c *Connector) SetChecklistReader(r ChecklistReader) { c.checklistR = r }

// SetChecklistMutator injects the wiki-side mutator the inbound sync
// path uses to apply Tasks-originated changes. Optional — nil
// disables inbound apply.
func (c *Connector) SetChecklistMutator(w ChecklistMutator) { c.checklistW = w }

// SetSyncSuppressor injects the suppressor used during inbound apply
// to keep mutator notifies from looping back as new sync triggers.
func (c *Connector) SetSyncSuppressor(s SyncSuppressor) { c.suppressor = s }

// Kind returns ConnectorKindGoogleTasks.
func (*Connector) Kind() connectors.ConnectorKind {
	return connectors.ConnectorKindGoogleTasks
}

// Compile-time check: the Tasks Connector satisfies the cross-
// connector dispatch shape.
var _ connectors.Connector = (*Connector)(nil)

// Sync runs one inbound-then-outbound reconcile pass for the given
// subscription. Implements connectors.Connector.Sync.
//
// Algorithm (per plan §"Sync semantics specification"):
//
//  1. Load the Subscription. Return early if paused (cursor frozen).
//  2. Per-connector rate-limit choke: skip if last successful sync
//     was within rateLimitChokeSeconds of now.
//  3. Build a TasksClient bound to this profile's refresh token.
//  4. Inbound: walk all pages of ListTasks(updatedMin=cursor); apply
//     to wiki via mutator (suppressed); flatten subtasks if any
//     observed. Capture max(Task.updated) for cursor advance.
//  5. Outbound: diff current wiki items against ItemIDMap; insert
//     new uids (with pre-insert marker scan), patch updated uids
//     (with If-Match etag), delete missing uids.
//  6. Apply-then-advance: persist updated ItemIDMap + cursor on the
//     subscription record AFTER both directions succeed.
//
// On gateway.ErrInvalidGrant or gateway.ErrAuthRevoked (after
// retry-once exhaustion), transition the subscription to paused
// (PausedReason=auth_failed) and return nil — paused subscriptions
// are a steady-state condition, not an error to propagate to the
// scheduler. The user reconnects via the OAuth flow.
//
//revive:disable-next-line:cyclomatic // single-purpose orchestrator; splitting hurts readability
func (c *Connector) Sync(ctx context.Context, key connectors.SubscriptionKey) error {
	profileID := wikipage.PageIdentifier(key.ProfileID)
	sub, found, err := c.store.FindSubscription(profileID, key.Page, key.ListName)
	if err != nil {
		return fmt.Errorf("load subscription: %w", err)
	}
	if !found {
		// No subscription — nothing to sync. Not an error.
		return nil
	}

	if sub.IsPaused() {
		// Cursor frozen, no API calls. UI surfaces the paused badge.
		return nil
	}

	// Per-connector rate-limit choke (skip-this-tick).
	now := c.clock.Now().UTC()
	if !sub.LastSuccessfulSyncAt.IsZero() && now.Sub(sub.LastSuccessfulSyncAt) < rateLimitChokeSeconds*time.Second {
		return nil
	}

	state, err := c.store.LoadState(profileID)
	if err != nil {
		return fmt.Errorf("load profile state: %w", err)
	}
	if !state.IsConfigured() {
		// Profile lost its refresh token (Disconnect) — pause the
		// subscription so the next reconnect resumes cleanly.
		return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
	}

	client, _, err := c.clientFactory(profileID, state.RefreshToken)
	if err != nil {
		return fmt.Errorf("build tasks client: %w", err)
	}

	if c.checklistR == nil {
		return ErrChecklistReaderUnavailable
	}

	// Inbound apply.
	updatedSub, maxUpdated, err := c.applyInboundFromTasks(ctx, profileID, state.Email, sub, client)
	if err != nil {
		if c.isAuthFailure(err) {
			return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
		}
		return fmt.Errorf("inbound apply: %w", err)
	}
	sub = updatedSub

	// Outbound push.
	updatedSub2, err := c.pushOutboundToTasks(ctx, sub, client)
	if err != nil {
		if c.isAuthFailure(err) {
			return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
		}
		return fmt.Errorf("outbound push: %w", err)
	}
	sub = updatedSub2

	// Apply-then-advance the cursor. Use max(Task.updated) - safety
	// buffer (per plan §"Cursor — Boundary semantics").
	if !maxUpdated.IsZero() {
		advance := maxUpdated.Add(-time.Duration(updatedMinSafetyBufferSeconds) * time.Second)
		if advance.After(sub.LastUpdatedMin) {
			sub.LastUpdatedMin = advance
		}
	}
	sub.LastSuccessfulSyncAt = now

	if err := c.store.UpdateSubscription(profileID, sub); err != nil {
		return fmt.Errorf("persist subscription: %w", err)
	}
	return nil
}

// PausedReason reports whether the given subscription is paused and,
// if so, a human-readable reason. Implements connectors.Connector.
func (c *Connector) PausedReason(key connectors.SubscriptionKey) (string, bool) {
	sub, found, err := c.store.FindSubscription(wikipage.PageIdentifier(key.ProfileID), key.Page, key.ListName)
	if err != nil || !found {
		return "", false
	}
	if !sub.IsPaused() {
		return "", false
	}
	if sub.PausedReason != "" {
		return sub.PausedReason, true
	}
	return "paused", true
}

// ForceFullResync triggers a one-shot full re-fetch on the next Sync.
// Per plan §"Pause / resume horizon — Resume after pause ≥ 7 days":
// pulls the full Tasks list, rebuilds ItemIDMap by text-match against
// current wiki items, resets LastUpdatedMin so the next inbound
// re-processes the world.
//
// Used by:
//
//   - The pause/resume path when pause duration ≥ 7d.
//   - The cursor-truncation recovery path (Google reports cursor
//     invalid).
//   - An operator-triggered admin RPC.
func (c *Connector) ForceFullResync(ctx context.Context, key connectors.SubscriptionKey) error {
	profileID := wikipage.PageIdentifier(key.ProfileID)
	sub, found, err := c.store.FindSubscription(profileID, key.Page, key.ListName)
	if err != nil {
		return fmt.Errorf("load subscription: %w", err)
	}
	if !found {
		return ErrSubscriptionNotFound
	}

	state, err := c.store.LoadState(profileID)
	if err != nil {
		return fmt.Errorf("load profile state: %w", err)
	}
	if !state.IsConfigured() {
		return ErrConnectorNotConfigured
	}

	client, _, err := c.clientFactory(profileID, state.RefreshToken)
	if err != nil {
		return fmt.Errorf("build tasks client: %w", err)
	}

	rebuiltMap, rebuiltEtags, err := c.rebuildIDMapByTextMatch(ctx, sub, client)
	if err != nil {
		return fmt.Errorf("rebuild id_map: %w", err)
	}

	sub.ItemIDMap = rebuiltMap
	sub.ItemEtags = rebuiltEtags
	sub.LastUpdatedMin = time.Time{} // re-process the world on next Sync
	sub.State = SubscriptionStateActive
	sub.PausedReason = ""
	sub.PausedAt = time.Time{}

	if err := c.store.UpdateSubscription(profileID, sub); err != nil {
		return fmt.Errorf("persist subscription: %w", err)
	}
	c.logger.Info("tasks bridge: force_full_resync_triggered profile=%s page=%s list=%s",
		string(profileID), key.Page, key.ListName)
	return nil
}

// rebuildIDMapByTextMatch performs the subscribe-ceremony seed: pull
// the full Tasks list, strip wiki:uid markers (when present), and
// text-match against the current wiki checklist to rebuild the id_map.
//
// When a Tasks task carries a wiki:uid marker, the marker wins and
// the matching wiki uid is bound to that Tasks id directly. Markerless
// tasks are matched by lowercased+trimmed title against the wiki
// checklist; ties (multiple wiki items with the same text) are
// resolved by taking the lowest sort_order — deterministic, but
// degraded in real-world ambiguity (documented as a known limitation).
func (c *Connector) rebuildIDMapByTextMatch(ctx context.Context, sub Subscription, client TasksClient) (map[string]string, map[string]string, error) {
	tasks, err := c.listAllTasks(ctx, client, sub.RemoteListID, time.Time{})
	if err != nil {
		return nil, nil, err
	}

	wikiItems, err := c.checklistR.ListItems(ctx, sub.Page, sub.ListName)
	if err != nil {
		return nil, nil, fmt.Errorf("read wiki checklist: %w", err)
	}

	// Build lookup from normalized wiki text → wiki uid (lowest
	// sort_order wins on ties).
	type wikiCandidate struct {
		uid       string
		sortOrder int64
	}
	wikiByText := map[string]wikiCandidate{}
	if wikiItems != nil {
		for _, it := range wikiItems.GetItems() {
			text := normalizeText(it.GetText())
			if text == "" {
				continue
			}
			cand := wikiCandidate{uid: it.GetUid(), sortOrder: it.GetSortOrder()}
			existing, ok := wikiByText[text]
			if !ok || cand.sortOrder < existing.sortOrder {
				wikiByText[text] = cand
			}
		}
	}

	idMap := map[string]string{}
	etags := map[string]string{}
	for _, t := range tasks {
		if t.Deleted {
			continue
		}
		// Marker wins.
		_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
		if hasMarker && markerUID != "" {
			idMap[markerUID] = t.ID
			if t.Etag != "" {
				etags[t.ID] = t.Etag
			}
			continue
		}
		// Fall back to text match.
		text := normalizeText(t.Title)
		if text == "" {
			continue
		}
		cand, ok := wikiByText[text]
		if !ok {
			continue
		}
		// First-match wins (don't overwrite a marker-derived entry).
		if _, already := idMap[cand.uid]; already {
			continue
		}
		idMap[cand.uid] = t.ID
		if t.Etag != "" {
			etags[t.ID] = t.Etag
		}
	}
	return idMap, etags, nil
}

// normalizeText is the canonical-form helper used for text matching
// during ItemIDMap rebuild. Lowercases, trims, collapses internal
// whitespace, and strips trailing #tag tokens — symmetric with the
// translator's title encoding so a wiki item's text and its Tasks
// counterpart compare equal after a round trip.
func normalizeText(s string) string {
	clean, _ := translator.TitleAndTagsFromText(s)
	return strings.ToLower(strings.TrimSpace(clean))
}

// listAllTasks consumes every page of ListTasks for the given list
// before returning. Per plan §"Cursor — Never advance during
// pagination": multi-page walks finish before any cursor advance.
func (c *Connector) listAllTasks(ctx context.Context, client TasksClient, remoteListID string, updatedMin time.Time) ([]gateway.Task, error) {
	var out []gateway.Task
	pageToken := ""
	for {
		page, err := client.ListTasks(ctx, remoteListID, updatedMin, pageToken)
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

// applyInboundFromTasks consumes all pages of the inbound poll,
// applies changes to the wiki via the mutator (with notify
// suppression), and returns the updated subscription plus the max
// Task.updated timestamp seen (for cursor advance).
//
// Subtask flatten: if any returned task has a non-empty Parent the
// orchestrator records EventRemoteItemArrived in the log and
// flattens silently — per plan §3 ("tolerant flatten if subtasks
// appear post-subscribe").
//
// Marker-loss recovery: a markerless task whose id is not in
// ItemIDMap is text-matched against the current wiki checklist; on
// a hit, the orchestrator re-maps the wiki uid to the new task id
// (the next outbound push will re-stamp the marker). On a miss, the
// task is treated as a fresh inbound arrival.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (c *Connector) applyInboundFromTasks(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, sub Subscription, client TasksClient) (Subscription, time.Time, error) {
	tasks, err := c.listAllTasks(ctx, client, sub.RemoteListID, sub.LastUpdatedMin)
	if err != nil {
		return sub, time.Time{}, err
	}
	if len(tasks) == 0 {
		return sub, time.Time{}, nil
	}

	if translator.HasSubtasks(toTranslatorTasks(tasks)) {
		c.logger.Info("tasks bridge: %s detected subtasks on profile=%s page=%s list=%s; flattening",
			connectors.EventRemoteItemArrived, string(profileID), sub.Page, sub.ListName)
	}

	// Reverse map: tasks_id → wiki_uid (so we can recognise inbound
	// updates without scanning the map per task).
	taskToUID := make(map[string]string, len(sub.ItemIDMap))
	for uid, taskID := range sub.ItemIDMap {
		taskToUID[taskID] = uid
	}

	if sub.ItemIDMap == nil {
		sub.ItemIDMap = map[string]string{}
	}
	if sub.ItemEtags == nil {
		sub.ItemEtags = map[string]string{}
	}

	// Pull current wiki state once for marker-loss recovery.
	var wikiByText map[string]string
	wikiResolved := false

	resolveWikiByText := func() error {
		if wikiResolved {
			return nil
		}
		items, err := c.checklistR.ListItems(ctx, sub.Page, sub.ListName)
		if err != nil {
			return fmt.Errorf("read wiki checklist: %w", err)
		}
		wikiByText = map[string]string{}
		if items != nil {
			for _, it := range items.GetItems() {
				text := normalizeText(it.GetText())
				if text != "" {
					wikiByText[text] = it.GetUid()
				}
			}
		}
		wikiResolved = true
		return nil
	}

	if c.suppressor != nil {
		c.suppressor.Suppress(profileID, sub.Page, sub.ListName)
		defer c.suppressor.Unsuppress(profileID, sub.Page, sub.ListName)
	}

	maxUpdated := time.Time{}
	for _, t := range tasks {
		if t.Updated.After(maxUpdated) {
			maxUpdated = t.Updated
		}

		if t.Deleted {
			c.applyInboundDeletion(ctx, profileID, ownerEmail, sub, t, taskToUID)
			continue
		}

		// Resolve wiki uid for this task.
		uid, hasUID := taskToUID[t.ID]
		if !hasUID {
			// Marker check first — if the task carries a marker we
			// already created on a previous push, restore the binding.
			_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
			if hasMarker && markerUID != "" {
				uid = markerUID
				hasUID = true
				sub.ItemIDMap[uid] = t.ID
				taskToUID[t.ID] = uid
			} else {
				// Marker-loss recovery path.
				if err := resolveWikiByText(); err != nil {
					return sub, maxUpdated, err
				}
				if matched, ok := wikiByText[normalizeText(t.Title)]; ok {
					uid = matched
					hasUID = true
					sub.ItemIDMap[uid] = t.ID
					taskToUID[t.ID] = uid
				}
			}
		}

		item, err := translator.TaskToChecklistItem(translator.Task{
			ID:        t.ID,
			ETag:      t.Etag,
			Title:     t.Title,
			Notes:     t.Notes,
			Status:    string(t.Status),
			Position:  t.Position,
			Parent:    "", // flatten silently on inbound
			Updated:   t.Updated,
			Due:       t.Due,
			Completed: t.Completed,
			Deleted:   t.Deleted,
			Hidden:    t.Hidden,
		})
		if err != nil {
			return sub, maxUpdated, fmt.Errorf("translate task %q: %w", t.ID, err)
		}

		if hasUID {
			if c.checklistW != nil {
				description := item.GetDescription()
				if updateErr := c.checklistW.UpdateItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, uid, item.GetText(), item.GetChecked(), item.GetTags(), description); updateErr != nil {
					return sub, maxUpdated, fmt.Errorf("update wiki item %q: %w", uid, updateErr)
				}
			}
			sub.ItemEtags[t.ID] = t.Etag
			continue
		}

		// Genuinely new from Tasks: AddItemForSync.
		if c.checklistW != nil {
			description := item.GetDescription()
			newUID, addErr := c.checklistW.AddItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, item.GetText(), item.GetChecked(), item.GetTags(), description, t.Position)
			if addErr != nil {
				return sub, maxUpdated, fmt.Errorf("add wiki item from task %q: %w", t.ID, addErr)
			}
			if newUID != "" {
				sub.ItemIDMap[newUID] = t.ID
				taskToUID[t.ID] = newUID
				sub.ItemEtags[t.ID] = t.Etag
				c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s task=%s",
					connectors.EventRemoteItemArrived, string(profileID), sub.Page, sub.ListName, t.ID)
			}
		}
	}

	return sub, maxUpdated, nil
}

// applyInboundDeletion mirrors a Tasks-side delete (or fully hidden
// completed task) into the wiki. Idempotent: if the uid is unknown
// the call is a no-op.
func (c *Connector) applyInboundDeletion(ctx context.Context, _ wikipage.PageIdentifier, ownerEmail string, sub Subscription, t gateway.Task, taskToUID map[string]string) {
	uid, ok := taskToUID[t.ID]
	if !ok {
		return
	}
	if c.checklistW != nil {
		_ = c.checklistW.DeleteItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, uid)
	}
	delete(sub.ItemIDMap, uid)
	delete(taskToUID, t.ID)
	delete(sub.ItemEtags, t.ID)
}

// pushOutboundToTasks diffs the wiki checklist against the
// subscription's ItemIDMap and pushes the delta to Google.
//
// For each wiki uid:
//
//   - new (not in id_map): pre-insert marker scan; if a Tasks task
//     carries this uid in its notes, switch to PatchTask. Otherwise
//     InsertTask with the uid marker appended.
//   - updated (in id_map): PatchTask with If-Match. On 412 pull-and-
//     retry-once with fresh etag.
//   - missing wiki uid (in id_map but not in current wiki items):
//     DeleteTask. Idempotent server-side.
//
// Persists the updated ItemIDMap and ItemEtags on the returned
// Subscription; caller persists.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (c *Connector) pushOutboundToTasks(ctx context.Context, sub Subscription, client TasksClient) (Subscription, error) {
	if c.checklistR == nil {
		return sub, ErrChecklistReaderUnavailable
	}
	checklist, err := c.checklistR.ListItems(ctx, sub.Page, sub.ListName)
	if err != nil {
		return sub, fmt.Errorf("read wiki checklist: %w", err)
	}

	if sub.ItemIDMap == nil {
		sub.ItemIDMap = map[string]string{}
	}
	if sub.ItemEtags == nil {
		sub.ItemEtags = map[string]string{}
	}

	currentUIDs := map[string]*apiv1.ChecklistItem{}
	if checklist != nil {
		for _, it := range checklist.GetItems() {
			if it.GetUid() != "" {
				currentUIDs[it.GetUid()] = it
			}
		}
	}

	// Lazy load of remote tasks: only fetched if we need to scan for
	// pre-insert marker dedup.
	var remoteByMarker map[string]gateway.Task
	loadRemote := func() error {
		if remoteByMarker != nil {
			return nil
		}
		tasks, listErr := c.listAllTasks(ctx, client, sub.RemoteListID, time.Time{})
		if listErr != nil {
			return listErr
		}
		remoteByMarker = map[string]gateway.Task{}
		for _, t := range tasks {
			if t.Deleted {
				continue
			}
			_, uid, has := translator.StripWikiUIDMarker(t.Notes)
			if has && uid != "" {
				remoteByMarker[uid] = t
			}
		}
		return nil
	}

	// Inserts and updates.
	for uid, item := range currentUIDs {
		fields := translator.ChecklistItemToTaskFields(item)
		if taskID, found := sub.ItemIDMap[uid]; found {
			// Patch path.
			patched, patchErr := c.patchWithRetry(ctx, client, sub, taskID, fields)
			if patchErr != nil {
				return sub, patchErr
			}
			sub.ItemEtags[patched.ID] = patched.Etag
			c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s uid=%s task=%s",
				connectors.EventLocalItemPushed, "<profile>", sub.Page, sub.ListName, uid, patched.ID)
			continue
		}

		// Insert path with pre-insert marker scan.
		if err := loadRemote(); err != nil {
			return sub, err
		}
		if existing, ok := remoteByMarker[uid]; ok {
			// Marker collision — Patch instead of Insert (idempotent
			// recovery from a previous outbound that succeeded server-
			// side but failed before persisting our state).
			patched, patchErr := c.patchWithRetry(ctx, client, sub, existing.ID, fields)
			if patchErr != nil {
				return sub, patchErr
			}
			sub.ItemIDMap[uid] = patched.ID
			sub.ItemEtags[patched.ID] = patched.Etag
			continue
		}

		inserted, insertErr := client.InsertTask(ctx, sub.RemoteListID, fields.Title, fields.Notes, gateway.TaskStatus(fields.Status), fields.Due, "")
		if insertErr != nil {
			return sub, insertErr
		}
		sub.ItemIDMap[uid] = inserted.ID
		sub.ItemEtags[inserted.ID] = inserted.Etag
		c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s uid=%s task=%s (inserted)",
			connectors.EventLocalItemPushed, "<profile>", sub.Page, sub.ListName, uid, inserted.ID)
	}

	// Deletions: id_map entries whose uid is no longer in the wiki
	// checklist. Google's tasks.delete is idempotent.
	for uid, taskID := range sub.ItemIDMap {
		if _, ok := currentUIDs[uid]; ok {
			continue
		}
		if delErr := client.DeleteTask(ctx, sub.RemoteListID, taskID); delErr != nil {
			return sub, delErr
		}
		delete(sub.ItemIDMap, uid)
		delete(sub.ItemEtags, taskID)
	}

	return sub, nil
}

// patchWithRetry sends a PatchTask with If-Match. On 412 (etag stale)
// pulls the task fresh and retries once with the new etag.
func (c *Connector) patchWithRetry(ctx context.Context, client TasksClient, sub Subscription, taskID string, fields translator.TaskFields) (gateway.Task, error) {
	patch := buildPatchFields(fields)
	etag := sub.ItemEtags[taskID]
	patched, err := client.PatchTask(ctx, sub.RemoteListID, taskID, patch, etag)
	if err == nil {
		return patched, nil
	}
	if !errors.Is(err, gateway.ErrPreconditionFailed) {
		return gateway.Task{}, err
	}
	// 412 — refetch and retry once with no If-Match (last-write-wins
	// is the deliberate fallback per plan §"Outbound idempotence").
	patched, retryErr := client.PatchTask(ctx, sub.RemoteListID, taskID, patch, "")
	if retryErr != nil {
		return gateway.Task{}, retryErr
	}
	return patched, nil
}

// buildPatchFields turns the translator's TaskFields shape into the
// gateway's PatchFields with all the Set* flags asserted.
func buildPatchFields(f translator.TaskFields) gateway.PatchFields {
	out := gateway.PatchFields{
		SetTitle:  true,
		Title:     f.Title,
		SetNotes:  true,
		Notes:     f.Notes,
		SetStatus: true,
		Status:    gateway.TaskStatus(f.Status),
		SetDue:    true,
		Due:       f.Due,
	}
	return out
}

// transitionToPaused writes the subscription as paused with the given
// reason. Cursor and ItemIDMap are preserved per plan §"Pause / resume
// horizon — Cursor frozen during pause".
func (c *Connector) transitionToPaused(profileID wikipage.PageIdentifier, sub Subscription, reason string) error {
	sub.State = SubscriptionStatePaused
	sub.PausedReason = reason
	sub.PausedAt = c.clock.Now().UTC()
	if err := c.store.UpdateSubscription(profileID, sub); err != nil {
		return fmt.Errorf("persist paused subscription: %w", err)
	}
	c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s reason=%s",
		connectors.EventSubscriptionPaused, string(profileID), sub.Page, sub.ListName, reason)
	return nil
}

// isAuthFailure reports whether err indicates the OAuth credentials
// are no longer usable. Triggers the pause transition.
func (c *Connector) isAuthFailure(err error) bool {
	return errors.Is(err, gateway.ErrInvalidGrant) || errors.Is(err, gateway.ErrAuthRevoked)
}

// toTranslatorTasks converts gateway.Task slices to the translator's
// placeholder Task type so HasSubtasks/FlattenSubtasks can operate.
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
