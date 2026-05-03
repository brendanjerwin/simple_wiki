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
//  1. Load the full profile state once (derives refreshToken, email,
//     and the matching Subscription in a single read).
//  2. Return early if no subscription found or if it is paused.
//  3. Per-connector rate-limit choke: skip if last successful sync
//     was within rateLimitChokeSeconds of now.
//  4. Build a TasksClient bound to this profile's refresh token.
//  5. Inbound: walk all pages of ListTasks(updatedMin=cursor); apply
//     to wiki via mutator (suppressed); flatten subtasks if any
//     observed. Capture max(Task.updated) for cursor advance.
//  6. Outbound: diff current wiki items against ItemIDMap; insert
//     new uids (with pre-insert marker scan), patch updated uids
//     (with If-Match etag), delete missing uids.
//  7. Apply-then-advance: persist updated ItemIDMap + cursor on the
//     subscription record AFTER both directions succeed.
//
// On gateway.ErrInvalidGrant or gateway.ErrAuthRevoked (after
// retry-once exhaustion), transition the subscription to paused
// (PausedReason=auth_failed) and return nil — paused subscriptions
// are a steady-state condition, not an error to propagate to the
// scheduler. The user reconnects via the OAuth flow.
//
func (c *Connector) Sync(ctx context.Context, key connectors.SubscriptionKey) error {
	profileID := wikipage.PageIdentifier(key.ProfileID)

	// Single profile read: derive refreshToken, email, and the
	// matching Subscription from this one result rather than calling
	// FindSubscription (which internally calls LoadState) and then
	// LoadState again — that was two round-trips through the same
	// frontmatter for every Sync tick.
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return fmt.Errorf("load profile state: %w", err)
	}

	sub, found := findSubscriptionInState(state, key.Page, key.ListName)
	if !found {
		return nil
	}
	if sub.IsPaused() {
		return nil
	}

	now := c.clock.Now().UTC()
	if !sub.LastSuccessfulSyncAt.IsZero() && now.Sub(sub.LastSuccessfulSyncAt) < rateLimitChokeSeconds*time.Second {
		return nil
	}

	if !state.IsConfigured() {
		return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
	}

	client, _, err := c.clientFactory(profileID, state.RefreshToken)
	if err != nil {
		return fmt.Errorf("build tasks client: %w", err)
	}
	if c.checklistR == nil {
		return ErrChecklistReaderUnavailable
	}

	return c.runSyncPasses(ctx, profileID, state.Email, sub, client, now)
}

// runSyncPasses executes the inbound-then-outbound sync for one subscription:
// applies Tasks changes to the wiki, pushes wiki changes to Tasks, then
// advances the cursor and persists.
//
//revive:disable-next-line:cyclomatic
func (c *Connector) runSyncPasses(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, sub Subscription, client TasksClient, now time.Time) error {
	updatedSub, maxUpdated, err := c.applyInboundFromTasks(ctx, profileID, ownerEmail, sub, client)
	if err != nil {
		if c.isAuthFailure(err) {
			return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
		}
		return fmt.Errorf("inbound apply: %w", err)
	}
	sub = updatedSub

	updatedSub2, err := c.pushOutboundToTasks(ctx, profileID, ownerEmail, sub, client)
	if err != nil {
		if c.isAuthFailure(err) {
			return c.transitionToPaused(profileID, sub, PausedReasonAuthFailed)
		}
		return fmt.Errorf("outbound push: %w", err)
	}
	sub = updatedSub2

	// Apply-then-advance the cursor (per plan §"Cursor — Boundary semantics").
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
func (c *Connector) rebuildIDMapByTextMatch(ctx context.Context, sub Subscription, client TasksClient) (idMap map[string]string, etags map[string]string, err error) {
	var tasks []gateway.Task
	tasks, err = c.listAllTasks(ctx, client, sub.RemoteListID, time.Time{})
	if err != nil {
		return nil, nil, err
	}

	var wikiItems *apiv1.Checklist
	wikiItems, err = c.checklistR.ListItems(ctx, sub.Page, sub.ListName)
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

	idMap = map[string]string{}
	etags = map[string]string{}
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
func (*Connector) listAllTasks(ctx context.Context, client TasksClient, remoteListID string, updatedMin time.Time) ([]gateway.Task, error) {
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

	// Reverse map: tasks_id → wiki_uid.
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
	if sub.SyncedItems == nil {
		sub.SyncedItems = map[string]ItemSyncState{}
	}

	// Lazy loader for wiki-text lookup (marker-loss recovery).
	resolveWikiByText := c.buildWikiByTextResolver(ctx, sub)

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
		if err := c.applyOneInboundTask(ctx, profileID, ownerEmail, sub, taskToUID, t, resolveWikiByText); err != nil {
			return sub, maxUpdated, err
		}
	}
	return sub, maxUpdated, nil
}

// buildWikiByTextResolver returns a lazy loader that reads the wiki
// checklist once and builds a normalizedText → uid map. Calling it
// multiple times is idempotent (result cached after first call).
func (c *Connector) buildWikiByTextResolver(ctx context.Context, sub Subscription) func() (map[string]string, error) {
	var cached map[string]string
	return func() (map[string]string, error) {
		if cached != nil {
			return cached, nil
		}
		items, err := c.checklistR.ListItems(ctx, sub.Page, sub.ListName)
		if err != nil {
			return nil, fmt.Errorf("read wiki checklist: %w", err)
		}
		cached = map[string]string{}
		if items != nil {
			for _, it := range items.GetItems() {
				text := normalizeText(it.GetText())
				if text != "" {
					cached[text] = it.GetUid()
				}
			}
		}
		return cached, nil
	}
}

// applyOneInboundTask processes a single non-deleted inbound task:
// resolves the wiki uid (via id_map, marker, or text-match), then
// updates an existing wiki item or adds a new one.
// sub.ItemIDMap and sub.ItemEtags are maps — mutations here propagate
// back to the caller because map values are reference types.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (c *Connector) applyOneInboundTask(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, sub Subscription, taskToUID map[string]string, t gateway.Task, resolveWikiByText func() (map[string]string, error)) error {
	uid, hasUID := taskToUID[t.ID]
	if !hasUID {
		_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
		if hasMarker && markerUID != "" {
			uid = markerUID
			hasUID = true
			sub.ItemIDMap[uid] = t.ID
			taskToUID[t.ID] = uid
		} else {
			wikiByText, err := resolveWikiByText()
			if err != nil {
				return err
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
		return fmt.Errorf("translate task %q: %w", t.ID, err)
	}

	if hasUID {
		if c.checklistW != nil {
			description := item.GetDescription()
			if updateErr := c.checklistW.UpdateItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, uid, item.GetText(), item.GetChecked(), item.GetTags(), description); updateErr != nil {
				return fmt.Errorf("update wiki item %q: %w", uid, updateErr)
			}
		}
		sub.ItemEtags[t.ID] = t.Etag
		stampSyncedFromWikiItem(sub.SyncedItems, uid, item)
		return nil
	}

	// Genuinely new from Tasks: AddItemForSync.
	if c.checklistW != nil {
		description := item.GetDescription()
		newUID, addErr := c.checklistW.AddItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, item.GetText(), item.GetChecked(), item.GetTags(), description, t.Position)
		if addErr != nil {
			return fmt.Errorf("add wiki item from task %q: %w", t.ID, addErr)
		}
		if newUID != "" {
			sub.ItemIDMap[newUID] = t.ID
			taskToUID[t.ID] = newUID
			sub.ItemEtags[t.ID] = t.Etag
			stampSyncedFromWikiItem(sub.SyncedItems, newUID, item)
			c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s task=%s",
				connectors.EventRemoteItemArrived, string(profileID), sub.Page, sub.ListName, t.ID)
		}
	}
	return nil
}

// stampSyncedFromWikiItem advances the SyncedItems entry for uid to
// the wiki-side translation of item. Used by inbound apply paths so
// the next outbound diff sees "wiki == synced" (marker included) and
// skips a redundant patch. Stamping from raw remote fields would miss
// the wiki:uid marker that the outbound encoder appends, causing
// every tick to re-push the just-applied state.
func stampSyncedFromWikiItem(synced map[string]ItemSyncState, uid string, item *apiv1.ChecklistItem) {
	item.Uid = uid
	fields := translator.ChecklistItemToTaskFields(item)
	synced[uid] = ItemSyncState{
		SyncedTitle:  fields.Title,
		SyncedNotes:  fields.Notes,
		SyncedStatus: fields.Status,
		SyncedDue:    fields.Due,
	}
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
	delete(sub.SyncedItems, uid)
}

// pushOutboundToTasks diffs the wiki checklist against the
// subscription's ItemIDMap and pushes the delta to Google.
//
// For each wiki uid:
//
//   - new (not in id_map): pre-insert marker scan; if a Tasks task
//     carries this uid in its notes, switch to PatchTask. Otherwise
//     InsertTask with the uid marker appended.
//   - updated (in id_map): PatchTask with If-Match BUT ONLY when the
//     current wiki state differs from the SyncedItems baseline. If
//     it matches, skip the patch entirely — there is no local change
//     to push, and patching anyway would overwrite remote changes
//     that arrived between ticks (this was a real production bug).
//     On 412 (etag stale): pull the remote item and apply its state
//     back into the wiki via the mutator. We do NOT blind-retry the
//     patch — that path is last-write-wins and silently destroys
//     user changes from the phone.
//   - missing wiki uid (in id_map but not in current wiki items):
//     DeleteTask. Idempotent server-side.
//
// Persists the updated ItemIDMap, ItemEtags, and SyncedItems on the
// returned Subscription; caller persists.
func (c *Connector) pushOutboundToTasks(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, sub Subscription, client TasksClient) (Subscription, error) {
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
	if sub.SyncedItems == nil {
		sub.SyncedItems = map[string]ItemSyncState{}
	}

	currentUIDs := map[string]*apiv1.ChecklistItem{}
	if checklist != nil {
		for _, it := range checklist.GetItems() {
			if it.GetUid() != "" {
				currentUIDs[it.GetUid()] = it
			}
		}
	}

	sub, err = c.pushOutboundUpserts(ctx, profileID, ownerEmail, client, sub, currentUIDs)
	if err != nil {
		return sub, err
	}
	sub, err = c.pushOutboundDeletions(ctx, client, sub, currentUIDs)
	if err != nil {
		return sub, err
	}
	stampLastObservedWiki(&sub, currentUIDs)
	return sub, nil
}

// pushOutboundUpserts handles the insert-or-patch path for each wiki
// item that is currently in the checklist.
//
// Diff-before-push (the load-bearing rule): if a wiki uid is already
// in the id_map AND the current wiki fields equal the SyncedItems
// baseline, the patch is SKIPPED — the wiki has not changed since
// the last successful push, so re-patching would be a wasted call
// that overwrites whatever the user changed on Google's side
// (phone) since our last tick.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity,function-length
func (c *Connector) pushOutboundUpserts(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, client TasksClient, sub Subscription, currentUIDs map[string]*apiv1.ChecklistItem) (Subscription, error) {
	// Lazy load of remote tasks for pre-insert marker dedup.
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

	for uid, item := range currentUIDs {
		fields := translator.ChecklistItemToTaskFields(item)
		if taskID, found := sub.ItemIDMap[uid]; found {
			// DIFF-BEFORE-PUSH: skip the patch when the wiki state
			// matches the last successfully-pushed snapshot. This is
			// the fix for the every-tick-overwrites-phone bug — see
			// the function comment.
			if synced, ok := sub.SyncedItems[uid]; ok && syncedMatchesFields(synced, fields) {
				continue
			}
			patched, applied, patchErr := c.patchOrApplyRemote(ctx, profileID, ownerEmail, client, sub, uid, taskID, fields)
			if patchErr != nil {
				return sub, patchErr
			}
			if applied {
				// 412 → remote authoritative; we re-applied the
				// remote state to the wiki and did NOT push. The
				// next tick will re-evaluate after the wiki side is
				// caught up.
				continue
			}
			sub.ItemEtags[patched.ID] = patched.Etag
			sub.SyncedItems[uid] = stampSyncedFromFields(sub.SyncedItems[uid], fields)
			c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s uid=%s task=%s",
				connectors.EventLocalItemPushed, string(profileID), sub.Page, sub.ListName, uid, patched.ID)
			continue
		}

		// Insert path with pre-insert marker scan.
		if err := loadRemote(); err != nil {
			return sub, err
		}
		if existing, ok := remoteByMarker[uid]; ok {
			patched, applied, patchErr := c.patchOrApplyRemote(ctx, profileID, ownerEmail, client, sub, uid, existing.ID, fields)
			if patchErr != nil {
				return sub, patchErr
			}
			if applied {
				sub.ItemIDMap[uid] = existing.ID
				continue
			}
			sub.ItemIDMap[uid] = patched.ID
			sub.ItemEtags[patched.ID] = patched.Etag
			sub.SyncedItems[uid] = stampSyncedFromFields(sub.SyncedItems[uid], fields)
			continue
		}

		inserted, insertErr := client.InsertTask(ctx, sub.RemoteListID, fields.Title, fields.Notes, gateway.TaskStatus(fields.Status), fields.Due, "")
		if insertErr != nil {
			return sub, insertErr
		}
		sub.ItemIDMap[uid] = inserted.ID
		sub.ItemEtags[inserted.ID] = inserted.Etag
		sub.SyncedItems[uid] = stampSyncedFromFields(sub.SyncedItems[uid], fields)
		c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s uid=%s task=%s (inserted)",
			connectors.EventLocalItemPushed, string(profileID), sub.Page, sub.ListName, uid, inserted.ID)
	}
	return sub, nil
}

// pushOutboundDeletions removes remote Tasks whose wiki uid is no
// longer present in the checklist. tasks.delete is idempotent.
func (*Connector) pushOutboundDeletions(ctx context.Context, client TasksClient, sub Subscription, currentUIDs map[string]*apiv1.ChecklistItem) (Subscription, error) {
	for uid, taskID := range sub.ItemIDMap {
		if _, ok := currentUIDs[uid]; ok {
			continue
		}
		if delErr := client.DeleteTask(ctx, sub.RemoteListID, taskID); delErr != nil {
			return sub, delErr
		}
		delete(sub.ItemIDMap, uid)
		delete(sub.ItemEtags, taskID)
		delete(sub.SyncedItems, uid)
	}
	return sub, nil
}

// patchOrApplyRemote sends a PatchTask with If-Match. On 412 (etag
// stale) it does NOT blind-retry the patch — that would be
// last-write-wins and silently destroy whatever the user changed on
// Google's side (typically from their phone) since our last
// observation. Instead, it pulls the current remote state and folds
// it into the wiki via the inbound apply path; the wiki is now
// caught up, and the next tick (after a real wiki edit) will re-push
// cleanly.
//
// Returns:
//
//   - patched: the gateway.Task returned by a successful PatchTask
//     (when applied is false).
//   - applied: true when the 412 path ran — caller must NOT update
//     SyncedItems/etags from a phantom patch result.
//   - err: any non-412 error from the gateway, or any error from the
//     remote pull / mutator apply during the 412 path.
//
// On a 412 with no remote item visible (the task got deleted between
// our patch and our follow-up list), the remote-side delete is mirrored
// into the wiki via the mutator and the id_map / etags / synced state
// for this uid is cleaned up.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (c *Connector) patchOrApplyRemote(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, client TasksClient, sub Subscription, uid, taskID string, fields translator.TaskFields) (patched gateway.Task, applied bool, err error) {
	patch := buildPatchFields(fields)
	etag := sub.ItemEtags[taskID]
	patched, err = client.PatchTask(ctx, sub.RemoteListID, taskID, patch, etag)
	if err == nil {
		return patched, false, nil
	}
	if !errors.Is(err, gateway.ErrPreconditionFailed) {
		return gateway.Task{}, false, err
	}
	// 412 — remote moved under us. Pull the current authoritative
	// remote state and apply it to the wiki; do NOT push.
	c.logger.Info("tasks bridge: precondition_failed_on_patch profile=%s page=%s list=%s uid=%s task=%s",
		string(profileID), sub.Page, sub.ListName, uid, taskID)
	if applyErr := c.applyRemoteAuthoritative(ctx, profileID, ownerEmail, sub, client, uid, taskID); applyErr != nil {
		return gateway.Task{}, false, fmt.Errorf("apply remote authoritative on 412: %w", applyErr)
	}
	return gateway.Task{}, true, nil
}

// applyRemoteAuthoritative pulls the current remote state for
// taskID, then folds it back into the wiki via the inbound mutator.
// Used by the 412-on-patch path so a remote-side change wins over a
// stale local change rather than being silently overwritten.
//
// Implementation note: the gateway exposes ListTasks (paginated) but
// not single-task GET, so we list with updatedMin=zero and find the
// matching id. For typical household-scale tasklists (<200 tasks)
// this is a single page; the pagination loop handles the rest.
func (c *Connector) applyRemoteAuthoritative(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, sub Subscription, client TasksClient, uid, taskID string) error {
	tasks, err := c.listAllTasks(ctx, client, sub.RemoteListID, time.Time{})
	if err != nil {
		return fmt.Errorf("list tasks for remote authoritative pull: %w", err)
	}
	var found *gateway.Task
	for i := range tasks {
		if tasks[i].ID == taskID {
			found = &tasks[i]
			break
		}
	}
	if found == nil || found.Deleted {
		// Remote is gone. Mirror the delete into the wiki via the
		// suppressed mutator path so the next outbound diff doesn't
		// re-push a stale local item.
		if c.checklistW != nil {
			if c.suppressor != nil {
				c.suppressor.Suppress(profileID, sub.Page, sub.ListName)
				defer c.suppressor.Unsuppress(profileID, sub.Page, sub.ListName)
			}
			_ = c.checklistW.DeleteItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, uid)
		}
		delete(sub.ItemIDMap, uid)
		delete(sub.ItemEtags, taskID)
		delete(sub.SyncedItems, uid)
		return nil
	}
	// Translate the remote task to a wiki item and apply.
	item, err := translator.TaskToChecklistItem(translator.Task{
		ID:        found.ID,
		ETag:      found.Etag,
		Title:     found.Title,
		Notes:     found.Notes,
		Status:    string(found.Status),
		Position:  found.Position,
		Updated:   found.Updated,
		Due:       found.Due,
		Completed: found.Completed,
	})
	if err != nil {
		return fmt.Errorf("translate remote task on 412: %w", err)
	}
	if c.checklistW != nil {
		if c.suppressor != nil {
			c.suppressor.Suppress(profileID, sub.Page, sub.ListName)
			defer c.suppressor.Unsuppress(profileID, sub.Page, sub.ListName)
		}
		if updateErr := c.checklistW.UpdateItemForSync(ctx, sub.Page, sub.ListName, ownerEmail, uid, item.GetText(), item.GetChecked(), item.GetTags(), item.GetDescription()); updateErr != nil {
			return fmt.Errorf("update wiki item on 412: %w", updateErr)
		}
	}
	// Refresh etag + Synced* baseline from the wiki-side translation
	// of the just-applied item — the next outbound diff compares
	// wiki-derived fields, so the synced floor must match those.
	sub.ItemEtags[taskID] = found.Etag
	stampSyncedFromWikiItem(sub.SyncedItems, uid, item)
	return nil
}

// syncedMatchesFields reports whether the SyncedItems baseline for a
// uid equals the wiki fields we'd otherwise send in a patch. When
// true, the diff loop SKIPS the patch — there is no local change.
//
// Due is compared at date-only resolution because Google Tasks
// stores `due` as date-only (the time-of-day is always 00:00 on the
// wire round-trip). Comparing at full-timestamp resolution causes a
// permanent mismatch when the wiki keeps a time-of-day on its Due
// proto: the wiki side is "21:30 today," Google's server-side is
// "00:00 today," and the diff would re-fire every tick. Aligning at
// date-only resolution matches Tasks's actual semantics.
func syncedMatchesFields(s ItemSyncState, fields translator.TaskFields) bool {
	if s.SyncedTitle != fields.Title {
		return false
	}
	if s.SyncedNotes != fields.Notes {
		return false
	}
	if s.SyncedStatus != fields.Status {
		return false
	}
	if !sameDueDate(s.SyncedDue, fields.Due) {
		return false
	}
	return true
}

// sameDueDate reports whether two Due timestamps refer to the same
// calendar date in UTC. See syncedMatchesFields for the rationale.
func sameDueDate(a, b time.Time) bool {
	if a.IsZero() != b.IsZero() {
		return false
	}
	if a.IsZero() {
		return true
	}
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// stampSyncedFromFields returns a new ItemSyncState with the Synced*
// fields advanced to reflect a successful push. LastObservedWiki* is
// preserved so the next tick can detect "wiki re-edited locally
// after our push."
func stampSyncedFromFields(prev ItemSyncState, fields translator.TaskFields) ItemSyncState {
	prev.SyncedTitle = fields.Title
	prev.SyncedNotes = fields.Notes
	prev.SyncedStatus = fields.Status
	prev.SyncedDue = fields.Due
	return prev
}

// stampLastObservedWiki updates the LastObservedWiki* fields on every
// SyncedItems entry to match the current wiki state. Called at the
// end of every outbound pass so the next tick can compare against
// "what the wiki looked like after we last observed it."
func stampLastObservedWiki(sub *Subscription, currentUIDs map[string]*apiv1.ChecklistItem) {
	if sub.SyncedItems == nil {
		sub.SyncedItems = map[string]ItemSyncState{}
	}
	for uid, item := range currentUIDs {
		fields := translator.ChecklistItemToTaskFields(item)
		entry := sub.SyncedItems[uid]
		entry.LastObservedWikiTitle = fields.Title
		entry.LastObservedWikiNotes = fields.Notes
		entry.LastObservedWikiStatus = fields.Status
		entry.LastObservedWikiDue = fields.Due
		sub.SyncedItems[uid] = entry
	}
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
func (*Connector) isAuthFailure(err error) bool {
	return errors.Is(err, gateway.ErrInvalidGrant) || errors.Is(err, gateway.ErrAuthRevoked)
}

// findSubscriptionInState locates the subscription for (page, listName)
// within an already-loaded ConnectorState. Avoids a second LoadState
// call inside FindSubscription.
func findSubscriptionInState(state ConnectorState, page, listName string) (Subscription, bool) {
	for _, sub := range state.Subscriptions {
		if sub.Page == page && sub.ListName == listName {
			return sub, true
		}
	}
	return Subscription{}, false
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
