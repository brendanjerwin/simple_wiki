package sync

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

// RemoteList is the shape ListRemoteLists returns for the subscribe
// picker — Google Tasks tasklist id + display title.
type RemoteList struct {
	ID    string
	Title string
}

// Connect persists a fresh OAuth result on the user's profile after
// the OAuth callback handler (Phase 6) has completed the auth-code
// exchange and obtained a refresh token.
//
// The handler verifies state/iss/PKCE before calling Connect; this
// method is just persistence. Existing subscriptions are preserved
// (a reconnect keeps the user's checklist subscriptions intact and
// resumes them automatically on the next Sync).
func (c *Connector) Connect(_ context.Context, profileID wikipage.PageIdentifier, email, refreshToken string) (ConnectorState, error) {
	if email == "" {
		return ConnectorState{}, errors.New("tasks bridge: email is required")
	}
	if refreshToken == "" {
		return ConnectorState{}, errors.New("tasks bridge: refresh_token is required")
	}

	now := c.clock.Now().UTC()
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return ConnectorState{}, err
	}
	state.Email = email
	state.RefreshToken = refreshToken
	if state.ConnectedAt.IsZero() {
		state.ConnectedAt = now
	}
	state.LastVerifiedAt = now

	if err := c.store.SaveState(profileID, state); err != nil {
		return ConnectorState{}, err
	}
	return state, nil
}

// PersistRefreshToken stores a fresh refresh token on the user's profile
// after the OAuth callback handler has completed the auth-code exchange.
// Implements server.RefreshTokenPersister — Phase 6's handler invokes
// this on a successful callback (after iss/state/PKCE/scope checks
// pass).
//
// Differs from Connect in two ways:
//
//   - Email is optional. The OAuth callback handler does not parse the
//     id_token, so it has no email to forward; Connect REQUIRES an
//     email and would reject. PersistRefreshToken stores the token on
//     whatever email already exists on the profile (typically empty;
//     the user can fill it in via a separate UI flow if desired). The
//     refresh token is the load-bearing artifact, not the email.
//   - Suitable for headless boundary callers that don't want to reason
//     about the per-profile state shape.
//
// Auto-resume on reconnect (per plan §"Auth-failed UX"): after persisting
// the fresh token, every paused subscription on the profile is walked
// and Resume() is called on each. Resume is idempotent — calling it on
// an already-active subscription is a no-op — so the walk is safe to
// run unconditionally. A best-effort failure to resume a single
// subscription is logged but does not roll back the token persistence;
// the next scheduler tick re-attempts naturally.
func (c *Connector) PersistRefreshToken(ctx context.Context, profileID, accountEmail, refreshToken string) error {
	if profileID == "" {
		return errors.New("tasks bridge: profileID is required")
	}
	if refreshToken == "" {
		return errors.New("tasks bridge: refresh_token is required")
	}
	now := c.clock.Now().UTC()
	pid := wikipage.PageIdentifier(profileID)
	state, err := c.store.LoadState(pid)
	if err != nil {
		return fmt.Errorf("load profile state: %w", err)
	}
	if accountEmail != "" {
		state.Email = accountEmail
	}
	state.RefreshToken = refreshToken
	if state.ConnectedAt.IsZero() {
		state.ConnectedAt = now
	}
	state.LastVerifiedAt = now
	if err := c.store.SaveState(pid, state); err != nil {
		return fmt.Errorf("persist profile state: %w", err)
	}

	// Auto-resume paused subscriptions. Snapshot the (page, list)
	// pairs from the freshly-saved state — Resume() takes its own
	// lock on the profile, so we cannot iterate state.Subscriptions
	// while holding any mutex.
	type pausedKey struct {
		page, listName string
	}
	paused := make([]pausedKey, 0, len(state.Subscriptions))
	for _, sub := range state.Subscriptions {
		if sub.IsPaused() {
			paused = append(paused, pausedKey{page: sub.Page, listName: sub.ListName})
		}
	}
	for _, k := range paused {
		if resumeErr := c.Resume(ctx, pid, k.page, k.listName); resumeErr != nil {
			// Best-effort: a single failed Resume should not roll back
			// the token persistence. The next scheduler tick will
			// observe the still-paused subscription and try again
			// once the connector tries to sync against the fresh token.
			c.logger.Error("tasks bridge: auto-resume failed profile=%s page=%s list=%s err=%v",
				profileID, k.page, k.listName, resumeErr)
		}
	}
	return nil
}

// Disconnect wipes the refresh token from the calling user's profile
// but preserves the subscription list (paused). Reconnect resumes
// them.
func (c *Connector) Disconnect(_ context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return ConnectorState{}, err
	}
	state.RefreshToken = ""
	state.LastVerifiedAt = time.Time{}
	// Mark every subscription paused so PausedReason() reports the
	// auth-failed signal even before the next Sync runs.
	for i := range state.Subscriptions {
		if state.Subscriptions[i].State != SubscriptionStatePaused {
			state.Subscriptions[i].State = SubscriptionStatePaused
			state.Subscriptions[i].PausedReason = PausedReasonAuthFailed
			state.Subscriptions[i].PausedAt = c.clock.Now().UTC()
		}
	}
	if err := c.store.SaveState(profileID, state); err != nil {
		return ConnectorState{}, err
	}
	return state, nil
}

// GetState returns the calling user's connector state.
func (c *Connector) GetState(_ context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
	return c.store.LoadState(profileID)
}

// ListRemoteLists enumerates the calling user's Google Tasks
// tasklists. Used to populate the subscribe picker UI.
func (c *Connector) ListRemoteLists(ctx context.Context, profileID wikipage.PageIdentifier) ([]RemoteList, error) {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return nil, err
	}
	if !state.IsConfigured() {
		return nil, ErrConnectorNotConfigured
	}

	client, _, err := c.clientFactory(profileID, state.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("build tasks client: %w", err)
	}

	lists, err := client.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RemoteList, 0, len(lists))
	for _, l := range lists {
		out = append(out, RemoteList{ID: l.ID, Title: l.Title})
	}
	return out, nil
}

// Subscribe wires a wiki checklist (page, listName) to a Google
// Tasks tasklist. Implements the subscribe-ceremony per plan
// §"Single-Subscription invariant via LeaseTable":
//
//  1. Block until the LeaseTable has finished its boot rebuild.
//  2. Acquire the per-checklist mutex.
//  3. Fan-out re-read: verify no Subscription exists for
//     (page, listName) on any profile / connector.
//  4. If remoteListID is empty, create a fresh Tasks list named
//     after the wiki listName (the "Bind to a new list" path).
//  5. Refuse-to-subscribe if the (existing) Tasks list contains subtasks.
//  6. Subscribe-ceremony seed: text-match wiki items against
//     existing tasks, build initial ItemIDMap, stamp wiki:uid markers
//     on existing matched tasks (deferred to first outbound push).
//  7. Persist the Subscription on the profile.
//  8. Take the lease.
//  9. Release mutex; emit EventSubscriptionEstablished.
//
// Empty remoteListID means "create a new Tasks list named <listName>"
// — mirrors the Keep bridge's empty-keepNoteID semantics. The first
// outbound sync (cron tick or save-debounce) populates the new list
// with whatever wiki items already exist.
//
// Returns the persisted Subscription on success.
func (c *Connector) Subscribe(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, remoteListID string) (Subscription, error) {
	if page == "" {
		return Subscription{}, errors.New("tasks bridge: page is required")
	}
	if listName == "" {
		return Subscription{}, errors.New("tasks bridge: list_name is required")
	}

	if err := c.leaseTable.WaitReady(ctx); err != nil {
		return Subscription{}, fmt.Errorf("await lease-table ready: %w", err)
	}

	state, err := c.store.LoadState(profileID)
	if err != nil {
		return Subscription{}, err
	}
	if !state.IsConfigured() {
		return Subscription{}, ErrConnectorNotConfigured
	}

	client, _, err := c.clientFactory(profileID, state.RefreshToken)
	if err != nil {
		return Subscription{}, fmt.Errorf("build tasks client: %w", err)
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	owner := connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(profileID)}

	var subscribed Subscription
	cerErr := c.leaseTable.WithChecklistLock(checklistKey, func() error {
		var lockErr error
		subscribed, lockErr = c.subscribeWithLock(ctx, profileID, page, listName, remoteListID, client, checklistKey, owner)
		return lockErr
	})
	if cerErr != nil {
		return Subscription{}, cerErr
	}

	c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s remote=%s",
		connectors.EventSubscriptionEstablished, string(profileID), subscribed.Page, subscribed.ListName, subscribed.RemoteListID)
	return subscribed, nil
}

// subscribeWithLock performs the subscribe ceremony while the caller
// holds the per-checklist mutex. Cross-connector existence check,
// optional create-new tasklist, subtask guard, ID-map seed, and lease
// acquisition all happen here.
//
// Empty remoteListID means "create a fresh Tasks list named after
// listName" — mirrors the Keep bridge's empty-keepNoteID semantics.
// A create failure surfaces directly because nothing has been
// persisted yet.
func (c *Connector) subscribeWithLock(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, remoteListID string, client TasksClient, checklistKey connectors.ChecklistKey, owner connectors.LeaseOwner) (Subscription, error) {
	// Cross-profile/cross-connector existence check via the
	// LeaseTable — the boot-rebuild + the contract that every
	// successful Subscribe Takes a lease guarantees this is the
	// authoritative cross-profile view at this moment.
	if existing, exists := c.leaseTable.LookupOwner(checklistKey); exists {
		return Subscription{}, fmt.Errorf("%w: %s/%s held by %s/%s",
			connectors.ErrChecklistAlreadyLeased, page, listName,
			existing.Kind, existing.ProfileID)
	}

	effectiveRemoteID := remoteListID
	effectiveRemoteTitle := ""
	createdNewList := false
	if effectiveRemoteID == "" {
		created, createErr := client.CreateTaskList(ctx, listName)
		if createErr != nil {
			return Subscription{}, fmt.Errorf("create tasks list: %w", createErr)
		}
		effectiveRemoteID = created.ID
		effectiveRemoteTitle = created.Title
		createdNewList = true
	}

	tasks, listErr := c.listAllTasks(ctx, client, effectiveRemoteID, time.Time{})
	if listErr != nil {
		return Subscription{}, fmt.Errorf("inspect tasks list: %w", listErr)
	}
	if translator.HasSubtasks(toTranslatorTasks(tasks)) {
		return Subscription{}, ErrTasksListHasSubtasks
	}

	idMap, etags, err := c.seedIDMapForSubscribe(ctx, page, listName, tasks)
	if err != nil {
		return Subscription{}, fmt.Errorf("seed id_map: %w", err)
	}

	// Push existing wiki items into the freshly-created tasklist so
	// the user sees their checklist contents in Google Tasks
	// immediately — without waiting for the next scheduler tick (which
	// would also be the FIRST tick after persist, but the new list
	// would be empty until then). Mirrors the Keep bridge's create-new
	// seed path. Only runs when we just created the list — for the
	// bind-to-existing path the next scheduler tick's outbound pass
	// pushes any wiki-only items (idMap-miss → insert).
	if createdNewList {
		seededMap, seededEtags, seedErr := c.seedNewTasklistFromWiki(ctx, page, listName, effectiveRemoteID, client)
		if seedErr != nil {
			return Subscription{}, fmt.Errorf("seed new tasklist from wiki: %w", seedErr)
		}
		idMap = seededMap
		etags = seededEtags
	}

	if effectiveRemoteTitle == "" {
		effectiveRemoteTitle = resolveRemoteTitle(ctx, client, effectiveRemoteID)
	}
	sub := Subscription{
		Page:            page,
		ListName:        listName,
		RemoteListID:    effectiveRemoteID,
		RemoteListTitle: effectiveRemoteTitle,
		ItemIDMap:       idMap,
		ItemEtags:       etags,
		State:           SubscriptionStateActive,
		SubscribedAt:    c.clock.Now().UTC(),
	}

	if err := c.store.AddSubscription(profileID, sub); err != nil {
		return Subscription{}, err
	}
	if err := c.leaseTable.Take(checklistKey, owner); err != nil {
		_ = c.store.RemoveSubscription(profileID, page, listName)
		return Subscription{}, err
	}
	return sub, nil
}

// resolveRemoteTitle fetches the friendly display title for remoteListID.
// Returns "" on any error — the title is cosmetic; failure must not
// block the subscribe ceremony.
func resolveRemoteTitle(ctx context.Context, client TasksClient, remoteListID string) string {
	taskLists, err := client.ListTaskLists(ctx)
	if err != nil {
		return ""
	}
	for _, tl := range taskLists {
		if tl.ID == remoteListID {
			return tl.Title
		}
	}
	return ""
}

// seedNewTasklistFromWiki pushes every wiki checklist item into the
// freshly-created tasklist via InsertTask, then returns the resulting
// ItemIDMap (wiki uid → tasks id) and ItemEtags (tasks id → etag).
// Each insert's notes carry the wiki:uid marker so subsequent
// inbound pulls can recover the binding without text-match.
//
// Called only by the create-new path of subscribeWithLock — for the
// bind-to-existing path, the seed runs via text-match against the
// existing remote tasks (see seedIDMapForSubscribe) and the next
// scheduler tick's outbound pass handles wiki items missing from the
// remote.
func (c *Connector) seedNewTasklistFromWiki(ctx context.Context, page, listName, remoteListID string, client TasksClient) (idMap map[string]string, etags map[string]string, err error) {
	if c.checklistR == nil {
		return nil, nil, ErrChecklistReaderUnavailable
	}
	checklist, err := c.checklistR.ListItems(ctx, page, listName)
	if err != nil {
		return nil, nil, fmt.Errorf("read wiki checklist: %w", err)
	}
	idMap = map[string]string{}
	etags = map[string]string{}
	if checklist == nil {
		return idMap, etags, nil
	}
	for _, item := range checklist.GetItems() {
		uid := item.GetUid()
		if uid == "" {
			// Items without a uid can't be re-bound on the inbound
			// pass — skip them. The outbound sync's upsert path
			// will pick them up after the wiki stamps a uid.
			continue
		}
		fields := translator.ChecklistItemToTaskFields(item)
		inserted, insertErr := client.InsertTask(ctx, remoteListID, fields.Title, fields.Notes, gateway.TaskStatus(fields.Status), fields.Due, "")
		if insertErr != nil {
			return nil, nil, fmt.Errorf("insert wiki item uid=%s: %w", uid, insertErr)
		}
		idMap[uid] = inserted.ID
		if inserted.Etag != "" {
			etags[inserted.ID] = inserted.Etag
		}
	}
	return idMap, etags, nil
}

// seedIDMapForSubscribe walks the existing tasks list and builds an
// initial ItemIDMap by matching task titles against the current wiki
// checklist's items. Tasks already carrying a wiki:uid marker are
// preferred (a re-subscribe after Disconnect/Reconnect should rebind
// without relying on text equality).
func (c *Connector) seedIDMapForSubscribe(ctx context.Context, page, listName string, tasks []gateway.Task) (idMap map[string]string, etags map[string]string, err error) {
	if c.checklistR == nil {
		return nil, nil, ErrChecklistReaderUnavailable
	}
	var checklist *apiv1.Checklist
	checklist, err = c.checklistR.ListItems(ctx, page, listName)
	if err != nil {
		return nil, nil, fmt.Errorf("read wiki checklist: %w", err)
	}
	wikiByText := map[string]string{}
	if checklist != nil {
		for _, it := range checklist.GetItems() {
			text := normalizeText(it.GetText())
			if text != "" {
				wikiByText[text] = it.GetUid()
			}
		}
	}

	idMap = map[string]string{}
	etags = map[string]string{}
	for _, t := range tasks {
		if t.Deleted {
			continue
		}
		_, markerUID, hasMarker := translator.StripWikiUIDMarker(t.Notes)
		if hasMarker && markerUID != "" {
			idMap[markerUID] = t.ID
			if t.Etag != "" {
				etags[t.ID] = t.Etag
			}
			continue
		}
		uid, ok := wikiByText[normalizeText(t.Title)]
		if !ok {
			continue
		}
		if _, already := idMap[uid]; already {
			continue
		}
		idMap[uid] = t.ID
		if t.Etag != "" {
			etags[t.ID] = t.Etag
		}
	}
	return idMap, etags, nil
}

// Unsubscribe removes the subscription for (page, listName) and
// releases the lease. Per plan §"Unsubscribe operation contract":
// per-checklist mutex acquire → write profile → release lease →
// release mutex.
func (c *Connector) Unsubscribe(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if err := c.leaseTable.WaitReady(ctx); err != nil {
		return fmt.Errorf("await lease-table ready: %w", err)
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	return c.leaseTable.WithChecklistLock(checklistKey, func() error {
		if err := c.store.RemoveSubscription(profileID, page, listName); err != nil {
			return err
		}
		c.leaseTable.Release(checklistKey)
		c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s",
			connectors.EventSubscriptionRevoked, string(profileID), page, listName)
		return nil
	})
}

// IsChecklistPaused reports whether any subscription on the given
// checklist is currently paused. Provided for the wiki's tombstone
// GC walker per plan §"Tombstone retention extension while paused" —
// the GC retains tombstones beyond the default 7-day window when
// any active subscription on the checklist is paused, so that the
// inbound deletion replay on resume isn't undone by tombstone
// collection.
//
// Returns false when no subscription exists for (page, listName) or
// when the lease isn't held by Tasks.
func (c *Connector) IsChecklistPaused(profileID wikipage.PageIdentifier, page, listName string) bool {
	sub, found, err := c.store.FindSubscription(profileID, page, listName)
	if err != nil || !found {
		return false
	}
	return sub.IsPaused()
}

// Resume transitions a paused subscription back to active. Per plan
// §"Pause / resume horizon":
//
//   - If pause duration < 7 days: incremental fetch from frozen
//     cursor on next Sync (this method clears the paused flags so
//     the next Sync proceeds normally).
//   - If pause duration ≥ 7 days: ForceFullResync runs synchronously
//     here, rebuilding the id_map by text-match. Cursor reset.
//
// Called by the OAuth callback handler (Phase 6) after a successful
// reconnect re-issues the refresh token. Idempotent — calling on a
// non-paused subscription is a no-op.
func (c *Connector) Resume(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	sub, found, err := c.store.FindSubscription(profileID, page, listName)
	if err != nil {
		return fmt.Errorf("load subscription: %w", err)
	}
	if !found {
		return ErrSubscriptionNotFound
	}
	if !sub.IsPaused() {
		return nil
	}

	pauseDuration := time.Duration(0)
	if !sub.PausedAt.IsZero() {
		pauseDuration = c.clock.Now().UTC().Sub(sub.PausedAt)
	}

	if pauseDuration >= resumeFullResyncHorizon {
		// ForceFullResync clears paused flags + resets cursor +
		// rebuilds id_map.
		key := connectors.SubscriptionKey{ProfileID: string(profileID), Page: page, ListName: listName}
		if err := c.ForceFullResync(ctx, key); err != nil {
			return err
		}
		c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s reason=resume_after_horizon pause_hours=%.1f",
			connectors.EventSubscriptionResumed, string(profileID), page, listName, pauseDuration.Hours())
		return nil
	}

	// <7d: incremental from frozen cursor.
	sub.State = SubscriptionStateActive
	sub.PausedReason = ""
	sub.PausedAt = time.Time{}
	if err := c.store.UpdateSubscription(profileID, sub); err != nil {
		return fmt.Errorf("persist resumed subscription: %w", err)
	}
	c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s",
		connectors.EventSubscriptionResumed, string(profileID), page, listName)
	return nil
}
