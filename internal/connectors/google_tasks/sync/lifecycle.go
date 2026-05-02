package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

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
//  4. Refuse-to-subscribe if the Tasks list contains subtasks.
//  5. Subscribe-ceremony seed: text-match wiki items against
//     existing tasks, build initial ItemIDMap, stamp wiki:uid markers
//     on existing matched tasks (deferred to first outbound push).
//  6. Persist the Subscription on the profile.
//  7. Take the lease.
//  8. Release mutex; emit EventSubscriptionEstablished.
//
// Returns the persisted Subscription on success.
func (c *Connector) Subscribe(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, remoteListID string) (Subscription, error) {
	if page == "" {
		return Subscription{}, errors.New("tasks bridge: page is required")
	}
	if listName == "" {
		return Subscription{}, errors.New("tasks bridge: list_name is required")
	}
	if remoteListID == "" {
		return Subscription{}, errors.New("tasks bridge: remote_list_id is required")
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
		// Cross-profile/cross-connector existence check via the
		// LeaseTable — the boot-rebuild + the contract that every
		// successful Subscribe Takes a lease guarantees this is the
		// authoritative cross-profile view at this moment.
		if existing, exists := c.leaseTable.LookupOwner(checklistKey); exists {
			return fmt.Errorf("%w: %s/%s held by %s/%s",
				connectors.ErrChecklistAlreadyLeased, page, listName,
				existing.Kind, existing.ProfileID)
		}

		// Refuse-to-subscribe if the chosen Tasks list contains a
		// parent-child hierarchy.
		tasks, listErr := c.listAllTasks(ctx, client, remoteListID, time.Time{})
		if listErr != nil {
			return fmt.Errorf("inspect tasks list: %w", listErr)
		}
		if translator.HasSubtasks(toTranslatorTasks(tasks)) {
			return ErrTasksListHasSubtasks
		}

		// Subscribe-ceremony seed: text-match wiki items against
		// existing tasks; build the initial ItemIDMap. The wiki:uid
		// marker stamping happens on the first outbound push (each
		// matched task will be Patched and the marker appended via
		// the translator); doing it here would require an extra
		// PatchTask call per match.
		idMap, etags, err := c.seedIDMapForSubscribe(ctx, page, listName, tasks)
		if err != nil {
			return fmt.Errorf("seed id_map: %w", err)
		}

		now := c.clock.Now().UTC()

		// Pull the friendly title for display.
		remoteTitle := ""
		taskLists, lookupErr := client.ListTaskLists(ctx)
		if lookupErr == nil {
			for _, tl := range taskLists {
				if tl.ID == remoteListID {
					remoteTitle = tl.Title
					break
				}
			}
		}

		sub := Subscription{
			Page:            page,
			ListName:        listName,
			RemoteListID:    remoteListID,
			RemoteListTitle: remoteTitle,
			ItemIDMap:       idMap,
			ItemEtags:       etags,
			State:           SubscriptionStateActive,
			SubscribedAt:    now,
		}

		if err := c.store.AddSubscription(profileID, sub); err != nil {
			return err
		}
		if err := c.leaseTable.Take(checklistKey, owner); err != nil {
			// Profile already updated — best-effort rollback. The
			// next subscribe on the same checklist will re-check
			// the LeaseTable.
			_ = c.store.RemoveSubscription(profileID, page, listName)
			return err
		}
		subscribed = sub
		return nil
	})
	if cerErr != nil {
		return Subscription{}, cerErr
	}

	c.logger.Info("tasks bridge: %s profile=%s page=%s list=%s remote=%s",
		connectors.EventSubscriptionEstablished, string(profileID), page, listName, remoteListID)
	return subscribed, nil
}

// seedIDMapForSubscribe walks the existing tasks list and builds an
// initial ItemIDMap by matching task titles against the current wiki
// checklist's items. Tasks already carrying a wiki:uid marker are
// preferred (a re-subscribe after Disconnect/Reconnect should rebind
// without relying on text equality).
func (c *Connector) seedIDMapForSubscribe(ctx context.Context, page, listName string, tasks []gateway.Task) (map[string]string, map[string]string, error) {
	if c.checklistR == nil {
		return nil, nil, ErrChecklistReaderUnavailable
	}
	checklist, err := c.checklistR.ListItems(ctx, page, listName)
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

	idMap := map[string]string{}
	etags := map[string]string{}
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
