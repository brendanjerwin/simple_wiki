package sync

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// This file defines the connectors.Connector interface adapters on the
// Keep *Connector. The Connector interface is the dispatch shape the
// shared SyncScheduler and LeaseTable use; satisfying it lets the
// scheduler drive Keep without hardcoding Keep-specific knowledge.
//
// These methods are thin adapters — they preserve the existing
// SyncToKeep / state-loading / FullResync semantics while exposing
// them under the cross-connector vocabulary.

// Kind returns ConnectorKindGoogleKeep.
func (*Connector) Kind() connectors.ConnectorKind {
	return connectors.ConnectorKindGoogleKeep
}

// Sync delegates to SyncToKeep using the SubscriptionKey's fields.
func (c *Connector) Sync(ctx context.Context, key connectors.SubscriptionKey) error {
	return c.SyncToKeep(ctx, wikipage.PageIdentifier(key.ProfileID), key.Page, key.ListName)
}

// PausedReason reports whether the binding identified by key is in a
// paused state. Today the Keep bridge surfaces this through
// ConnectorState.IsConfigured() — when the master token is missing
// (e.g., post-Disconnect with bindings preserved), every binding for
// that profile is effectively paused. Returns ("", false) when the
// binding's owner profile is configured. Returns a short, user-
// facing message when paused.
func (c *Connector) PausedReason(key connectors.SubscriptionKey) (string, bool) {
	state, err := c.store.LoadState(wikipage.PageIdentifier(key.ProfileID))
	if err != nil {
		return "", false
	}
	if !state.IsConfigured() {
		return "Google Keep is not connected for this profile", true
	}
	return "", false
}

// ForceFullResync drops the per-binding KeepCursor on the calling
// user's profile so the next Sync starts a full pull. Mirrors the
// truncation-recovery path's manual handle. Returns an error if the
// binding is missing or persistence fails.
func (c *Connector) ForceFullResync(_ context.Context, key connectors.SubscriptionKey) error {
	profileID := wikipage.PageIdentifier(key.ProfileID)
	return c.store.WithProfileLock(profileID, func() error {
		state, err := c.store.LoadStateLocked(profileID)
		if err != nil {
			return err
		}
		updated := false
		for i := range state.Subscriptions {
			if state.Subscriptions[i].Page == key.Page && state.Subscriptions[i].ListName == key.ListName {
				state.Subscriptions[i].KeepCursor = ""
				state.Subscriptions[i].TruncatedTickStreak = 0
				updated = true
			}
		}
		if !updated {
			return ErrSubscriptionNotFound
		}
		return c.store.SaveStateLocked(profileID, state)
	})
}

// Compile-time check: the Keep Connector satisfies the cross-connector
// dispatch shape. Catches drift if the interface evolves.
var _ connectors.Connector = (*Connector)(nil)

// Compile-time assertion: every connector MUST implement
// BackendAdapter (per ADR-0015 + the user's directive that
// shared primitives like remote-title sync are required across
// backends). Adding a method to BackendAdapter and forgetting to
// implement it here is now a compile error rather than a parity
// gap shipped to production.
var _ connectors.BackendAdapter = (*Connector)(nil)
