// Package sync owns the wiki-side of the Google Keep bridge: per-user
// connector state on profile pages, sync orchestration, and persistence
// of bindings. Translation between Keep node shapes and wiki ChecklistItems
// lives in the sibling translator package; the wire-protocol port lives
// in the sibling gateway package.
package sync

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Subscription is a single user's link from a wiki checklist (page + list_name)
// to a Keep note in their account.
//
// ItemIDMap is the per-binding map from wiki item UID to ItemMapping —
// the structured per-item sync record. Populated at bind time by the
// bundled CreateListWithItems path and updated on every successful sync.
// See ItemMapping for the full per-item shape.
//
// KeepCursor holds the last successful pull's `to_version` (Keep's
// monotonic server-side commit cursor). Sent as `target_version` on
// the next pull so Keep returns only the delta since our last sync.
// Empty cursor → full pull (fresh client / forced full resync).
//
// TruncatedTickStreak counts consecutive truncated pulls; combined
// with the "no items observed-changed" condition, drives the chronic-
// truncation escape hatch (force full resync after 5 such ticks).
//
// MigratedFingerprints is the eager-migration flag: false on legacy
// bindings until the migration job runs, then permanently true. The
// SyncToKeep gate refuses to sync until this is true (prevents acting
// on stale id_map state without per-item fingerprints).
//
// SubscribedAt is informational-only — used by the KeepConnect macro to
// render "bound on YYYY-MM-DD"; not consulted by sync logic.
type Subscription struct {
	Page          string
	ListName      string
	KeepNoteID    string
	KeepNoteTitle string
	// KeepNoteClientID is the LIST node's client-generated stable
	// identifier (the `id` field on Keep's wire format), distinct from
	// KeepNoteID which is the server-assigned `serverId`. Outbound LIST
	// node updates MUST send `id != serverId` — Keep returns stage3
	// HTTP 500 "Unknown Error" on `id == serverId`. Captured at:
	//   1. Bind time, from seedIDMapFromExistingList's pull walking the
	//      LIST node itself.
	//   2. Bootstrap time, from CreateListWithItems's generated client_id.
	//   3. Migration time, from MigrateSubscriptionFingerprints's full pull.
	// Empty for legacy bindings until first observed; a self-heal in
	// SyncToKeep clears KeepCursor on the empty case to force a full
	// pull whose LIST node populates this field.
	KeepNoteClientID     string
	SubscribedAt              time.Time
	KeepCursor           string
	TruncatedTickStreak  int
	MigratedFingerprints bool
	ItemIDMap            map[string]ItemMapping
	// LabelIDs persists the per-binding mapping from label name to
	// Keep label MainID. Captured from every pull that carries a
	// non-empty Labels slice and consulted as the PRIMARY lookup in
	// translator.MergeKeepLabels so incremental pulls (which usually
	// return no labels at all) don't cause the connector to emit
	// fresh label CRUD entries — and a corresponding new MainID —
	// every tick for labels Keep already knows about. The per-pull
	// Labels slice is the SECONDARY source: it only updates this map
	// when a label appears or its MainID changes.
	//
	// Tombstoned labels (LabelEntry.Deleted != 0) are removed from
	// the map so the next sync re-creates them rather than reusing a
	// dead MainID.
	//
	// Empty/nil for legacy bindings until the first pull that carries
	// a non-empty Labels slice — including the migration job's full
	// pull and Bind's seed-time full pull. translator.MergeKeepLabels
	// falls back to the per-pull byName index until the persisted map
	// is populated.
	LabelIDs map[string]string
}

// ItemMapping is the per-item sync record for one wiki UID inside a
// binding. It carries the Keep ServerID and the fingerprint baseline
// used by the divergence rule.
//
// The "synced fingerprint" (Synced{Text,Checked,SortValue}) is the
// post-successful-sync content baseline — the merge-base in the
// three-way merge. Each tick computes:
//
//	wiki_diverged := wiki_fp != synced_fp
//	keep_diverged := keep_fp != synced_fp
//
// to decide direction (push wiki / apply Keep / no-op / conflict).
//
// LastObservedWiki* records the wiki content as observed at the END
// of the prior tick. Used to detect "user re-edited locally since
// our last push attempt" — when current wiki_fp differs from this
// triple, we reset PushFailureCount to 0 (the obvious user fix path
// after a dead-letter).
//
// PushFailureCount counts consecutive per-item push failures; backoff
// is min(60 * 2^(n-1), 3600) seconds. After 10 failures the item is
// dead-lettered: surfaced via gRPC ListDeadLetters and skipped on
// subsequent pushes until the user clears it (ClearDeadLetter) or
// re-edits the wiki side. LastFailureCode is Keep's per-node status
// code (or our internal classifier) for the most recent failure.
//
// NextAttemptAt is the earliest wall-clock time at which the connector
// should retry pushing this item after a failure. Set on every push
// failure to `now + min(60 * 2^(n-1), 3600) seconds` where n is the
// post-increment PushFailureCount. The diff loop skips items whose
// NextAttemptAt is in the future. Zero value (always-eligible) is the
// normal steady state — only failed items carry a non-zero value, and
// successful pushes reset it back to zero alongside the failure count.
type ItemMapping struct {
	ServerID                  string
	SyncedText                string
	SyncedChecked             bool
	SyncedSortValue           string
	LastObservedWikiText      string
	LastObservedWikiChecked   bool
	LastObservedWikiSortValue string
	PushFailureCount          int
	LastFailureCode           string
	NextAttemptAt             time.Time
	// BaseVersion is Keep's optimistic-concurrency-control token for
	// this LIST_ITEM. Captured from every pull that includes the
	// node, persisted across ticks so incremental pulls (which only
	// return CHANGED nodes) don't strip it from the in-memory map.
	// Included as `baseVersion` on every outbound LIST_ITEM update —
	// Keep returns stage3 HTTP 500 "Unknown Error" if it's missing
	// or stale. Source: gkeepapi node.py self._base_version handling.
	BaseVersion string
	// ClientID is Keep's client-generated stable identifier (the
	// `id` field, distinct from `serverId`). Captured from pulls so
	// outbound updates carry the same `id` across ticks. Like
	// BaseVersion, must survive incremental pulls that don't echo
	// the node back. Empty for legacy bindings until first observed
	// in a pull.
	ClientID string
}

// ConnectorState is the per-user connector configuration stored on the
// profile page under wiki.connectors.google_keep.*.
//
// The MasterToken is plaintext per design — see plan Phase A and the help
// page for the trust-model rationale (Tailscale-fronted, profile pages
// read-restricted).
type ConnectorState struct {
	Email               string
	MasterToken         string
	ConnectedAt         time.Time
	LastVerifiedAt      time.Time
	PollIntervalSeconds int64
	Subscriptions            []Subscription
}

// IsConfigured reports whether the connector has a master token (i.e. the
// user has completed the connect flow). LoadState's zero return reads as
// "not configured" via this check.
func (s ConnectorState) IsConfigured() bool { return s.MasterToken != "" }

// Errors returned by SubscriptionStore. RPC handlers map these to gRPC codes.
var (
	// ErrAlreadySubscribedForChecklist is returned by Add when the calling
	// user already has a subscription for (page, list_name). Per the plan's
	// per-user collision matrix.
	ErrAlreadySubscribedForChecklist = errors.New("keep bridge: this checklist is already subscribed by you")

	// ErrKeepNoteAlreadyClaimed is returned by Add when the calling user
	// already has a subscription to the same Keep note (different checklist).
	ErrKeepNoteAlreadyClaimed = errors.New("keep bridge: this Keep note is already subscribed by you")

	// ErrSubscriptionNotFound is returned by Remove when no subscription matches.
	ErrSubscriptionNotFound = errors.New("keep bridge: subscription not found")

	// ErrConnectorNotConfigured is returned by methods that require an
	// active connector when the profile has no master_token.
	ErrConnectorNotConfigured = errors.New("keep bridge: connector not configured for this user")

	// ErrDeadLetterItemNotFound is returned by ClearDeadLetter when no
	// ItemMapping exists for the given (page, listName, itemUID). The
	// gRPC layer maps this to NotFound so the macro can surface a
	// "this item no longer exists" message.
	ErrDeadLetterItemNotFound = errors.New("keep bridge: dead-letter item not found")
)

// Frontmatter path constants. The connector state lives at
//   wiki.connectors.google_keep.*
// on the user's profile page. wiki.* is a reserved top-level namespace
// (wikipage/reserved_namespaces.go) so generic MergeFrontmatter rejects
// writes here — the typed SubscriptionStore is the sole funnel.
const (
	wikiKey       = "wiki"
	connectorsKey = "connectors"
	googleKeepKey = "google_keep"

	emailField               = "email"
	masterTokenField         = "master_token"
	connectedAtField         = "connected_at"
	lastVerifiedAtField      = "last_verified_at"
	pollIntervalSecondsField = "poll_interval_seconds"
	bindingsField            = "bindings"

	bindingPageField             = "page"
	bindingListNameField         = "list_name"
	bindingKeepNoteIDField       = "keep_note_id"
	bindingKeepNoteTitleField    = "keep_note_title"
	bindingKeepNoteClientIDField = "keep_note_client_id"
	bindingBoundAtField              = "bound_at"
	bindingKeepCursorField           = "keep_cursor"
	bindingTruncatedTickStreakField  = "truncated_tick_streak"
	bindingMigratedFingerprintsField = "migrated_fingerprints"
	bindingItemIDMapField            = "item_id_map"
	bindingLabelIDsField             = "label_ids"

	itemBindingServerIDField                  = "server_id"
	itemBindingSyncedTextField                = "synced_text"
	itemBindingSyncedCheckedField             = "synced_checked"
	itemBindingSyncedSortValueField           = "synced_sort_value"
	itemBindingLastObservedWikiTextField      = "last_observed_wiki_text"
	itemBindingLastObservedWikiCheckedField   = "last_observed_wiki_checked"
	itemBindingLastObservedWikiSortValueField = "last_observed_wiki_sort_value"
	itemBindingPushFailureCountField          = "push_failure_count"
	itemBindingLastFailureCodeField           = "last_failure_code"
	itemBindingNextAttemptAtField             = "next_attempt_at"
	itemBindingBaseVersionField               = "base_version"
	itemBindingClientIDField                  = "client_id"
)

// SubscriptionStore is the typed funnel for connector-state writes on profile
// pages. Mirrors the checklistmutator pattern: per-page mutex, all writes
// through wikipage.PageReaderMutator, no direct frontmatter mutation
// outside this package.
type SubscriptionStore struct {
	pages    wikipage.PageReaderMutator
	profilMu sync.Map // keyed by profileID; values *sync.Mutex
}

// NewSubscriptionStore constructs a SubscriptionStore.
func NewSubscriptionStore(pages wikipage.PageReaderMutator) *SubscriptionStore {
	return &SubscriptionStore{pages: pages}
}

// LoadState reads the full connector state for the given profile page.
// Missing profile or absent connector frontmatter both return a zero
// ConnectorState (no error) so callers can render "not connected".
func (s *SubscriptionStore) LoadState(profileID wikipage.PageIdentifier) (ConnectorState, error) {
	unlock := s.lockProfile(profileID)
	defer unlock()

	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConnectorState{}, nil
		}
		return ConnectorState{}, err
	}
	return decodeState(fm)
}

// SaveState overwrites the entire connector state on the profile page.
// Used by ExchangeAndStore (after a verified token exchange) and by
// Disconnect (to clear). Preserves all other top-level frontmatter.
func (s *SubscriptionStore) SaveState(profileID wikipage.PageIdentifier, state ConnectorState) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	fm, err := s.readFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	encodeState(fm, state)
	return s.writeFrontMatter(profileID, fm)
}

// AddSubscription appends a new binding to the calling user's profile after
// running the per-user collision matrix.
func (s *SubscriptionStore) AddSubscription(profileID wikipage.PageIdentifier, b Subscription) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		return err
	}
	state, err := decodeState(fm)
	if err != nil {
		return err
	}
	if !state.IsConfigured() {
		return ErrConnectorNotConfigured
	}

	for _, existing := range state.Subscriptions {
		if existing.Page == b.Page && existing.ListName == b.ListName {
			return ErrAlreadySubscribedForChecklist
		}
		if existing.KeepNoteID == b.KeepNoteID {
			return ErrKeepNoteAlreadyClaimed
		}
	}
	state.Subscriptions = append(state.Subscriptions, b)

	encodeState(fm, state)
	return s.writeFrontMatter(profileID, fm)
}

// RemoveSubscription removes the calling user's binding for (page, listName).
// Returns ErrSubscriptionNotFound if no match.
func (s *SubscriptionStore) RemoveSubscription(profileID wikipage.PageIdentifier, page, listName string) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		return err
	}
	state, err := decodeState(fm)
	if err != nil {
		return err
	}

	for i, existing := range state.Subscriptions {
		if existing.Page == page && existing.ListName == listName {
			state.Subscriptions = append(state.Subscriptions[:i], state.Subscriptions[i+1:]...)
			encodeState(fm, state)
			return s.writeFrontMatter(profileID, fm)
		}
	}
	return ErrSubscriptionNotFound
}

// FindSubscription returns the calling user's binding for (page, listName), if
// any. The boolean second return is "found".
func (s *SubscriptionStore) FindSubscription(profileID wikipage.PageIdentifier, page, listName string) (Subscription, bool, error) {
	state, err := s.LoadState(profileID)
	if err != nil {
		return Subscription{}, false, err
	}
	for _, b := range state.Subscriptions {
		if b.Page == page && b.ListName == listName {
			return b, true, nil
		}
	}
	return Subscription{}, false, nil
}

// readFrontMatter reads the page's frontmatter, returning os.ErrNotExist
// for missing pages so callers can branch.
func (s *SubscriptionStore) readFrontMatter(profileID wikipage.PageIdentifier) (wikipage.FrontMatter, error) {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	return fm, nil
}

func (s *SubscriptionStore) writeFrontMatter(profileID wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("keep bridge: write frontmatter: %w", err)
	}
	return nil
}

// lockProfile acquires the per-profile mutex.
func (s *SubscriptionStore) lockProfile(profileID wikipage.PageIdentifier) func() {
	v, _ := s.profilMu.LoadOrStore(profileID, &sync.Mutex{})
	// INVARIANT ASSERTION: every value stored in profilMu is *sync.Mutex.
	// Anything else here is a programming bug — falling back to a fresh
	// mutex would silently break the per-profile serialization invariant
	// (two writers could race). Panic loudly so the bug gets fixed.
	mu, ok := v.(*sync.Mutex)
	if !ok {
		panic(fmt.Sprintf("keep bridge: profilMu held a %T, expected *sync.Mutex — programming bug", v))
	}
	mu.Lock()
	return mu.Unlock
}

// LoadStateLocked reads state without acquiring the per-profile mutex.
// Caller MUST hold the lock (typically via WithProfileLock). Used by
// the eager migration job to read-then-write under one lock window.
// INVARIANT ASSERTION: this is documentation, not enforced; misuse
// races against AddSubscription/RemoveSubscription/SaveState.
func (s *SubscriptionStore) LoadStateLocked(profileID wikipage.PageIdentifier) (ConnectorState, error) {
	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConnectorState{}, nil
		}
		return ConnectorState{}, err
	}
	return decodeState(fm)
}

// SaveStateLocked overwrites state without acquiring the per-profile
// mutex. Caller MUST hold the lock. Pairs with LoadStateLocked for
// the eager migration job's read-rebaseline-write cycle.
func (s *SubscriptionStore) SaveStateLocked(profileID wikipage.PageIdentifier, state ConnectorState) error {
	fm, err := s.readFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	encodeState(fm, state)
	return s.writeFrontMatter(profileID, fm)
}

// WithProfileLock runs fn while holding the per-profile mutex. fn
// must use LoadStateLocked / SaveStateLocked for any state access
// (the regular LoadState / SaveState would deadlock). Used by the
// eager migration job to span its read-pull-rebaseline-write cycle
// inside one lock window so concurrent macro actions, cron ticks,
// and Bind/Unbind calls serialize against migration cleanly.
func (s *SubscriptionStore) WithProfileLock(profileID wikipage.PageIdentifier, fn func() error) error {
	unlock := s.lockProfile(profileID)
	defer unlock()
	return fn()
}

// --- codec ----------------------------------------------------------------

func decodeState(fm wikipage.FrontMatter) (ConnectorState, error) {
	connector := connectorMap(fm)
	if connector == nil {
		return ConnectorState{}, nil
	}
	connectedAt, err := parseTime(getString(connector, connectedAtField))
	if err != nil {
		return ConnectorState{}, fmt.Errorf("wiki.connectors.google_keep.connected_at: %w", err)
	}
	lastVerifiedAt, err := parseTime(getString(connector, lastVerifiedAtField))
	if err != nil {
		return ConnectorState{}, fmt.Errorf("wiki.connectors.google_keep.last_verified_at: %w", err)
	}
	bindings, err := decodeBindings(connector[bindingsField])
	if err != nil {
		return ConnectorState{}, err
	}
	return ConnectorState{
		Email:               getString(connector, emailField),
		MasterToken:         getString(connector, masterTokenField),
		ConnectedAt:         connectedAt,
		LastVerifiedAt:      lastVerifiedAt,
		PollIntervalSeconds: getInt64(connector, pollIntervalSecondsField),
		Subscriptions:            bindings,
	}, nil
}

func decodeBindings(raw any) ([]Subscription, error) {
	if raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("wiki.connectors.google_keep.bindings is %T, expected list", raw)
	}
	out := make([]Subscription, 0, len(arr))
	for i, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("wiki.connectors.google_keep.bindings[%d] is %T, expected map", i, entry)
		}
		boundAt, err := parseTime(getString(m, bindingBoundAtField))
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_keep.bindings[%d].bound_at: %w", i, err)
		}
		idMap, err := decodeItemIDMap(m[bindingItemIDMapField])
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_keep.bindings[%d].item_id_map: %w", i, err)
		}
		labelIDs, err := decodeLabelIDs(m[bindingLabelIDsField])
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_keep.bindings[%d].label_ids: %w", i, err)
		}
		out = append(out, Subscription{
			Page:                 getString(m, bindingPageField),
			ListName:             getString(m, bindingListNameField),
			KeepNoteID:           getString(m, bindingKeepNoteIDField),
			KeepNoteTitle:        getString(m, bindingKeepNoteTitleField),
			KeepNoteClientID:     getString(m, bindingKeepNoteClientIDField),
			SubscribedAt:              boundAt,
			KeepCursor:           getString(m, bindingKeepCursorField),
			TruncatedTickStreak:  getInt(m, bindingTruncatedTickStreakField),
			MigratedFingerprints: getBool(m, bindingMigratedFingerprintsField),
			ItemIDMap:            idMap,
			LabelIDs:             labelIDs,
		})
	}
	return out, nil
}

func encodeState(fm wikipage.FrontMatter, state ConnectorState) {
	connector := ensureConnectorMap(fm)
	if state.Email != "" {
		connector[emailField] = state.Email
	} else {
		delete(connector, emailField)
	}
	if state.MasterToken != "" {
		connector[masterTokenField] = state.MasterToken
	} else {
		delete(connector, masterTokenField)
	}
	if !state.ConnectedAt.IsZero() {
		connector[connectedAtField] = state.ConnectedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, connectedAtField)
	}
	if !state.LastVerifiedAt.IsZero() {
		connector[lastVerifiedAtField] = state.LastVerifiedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, lastVerifiedAtField)
	}
	if state.PollIntervalSeconds > 0 {
		connector[pollIntervalSecondsField] = state.PollIntervalSeconds
	} else {
		delete(connector, pollIntervalSecondsField)
	}
	if len(state.Subscriptions) > 0 {
		connector[bindingsField] = encodeBindings(state.Subscriptions)
	} else {
		delete(connector, bindingsField)
	}
}

func encodeBindings(bindings []Subscription) []any {
	out := make([]any, len(bindings))
	for i, b := range bindings {
		entry := map[string]any{
			bindingPageField:       b.Page,
			bindingListNameField:   b.ListName,
			bindingKeepNoteIDField: b.KeepNoteID,
		}
		if b.KeepNoteTitle != "" {
			entry[bindingKeepNoteTitleField] = b.KeepNoteTitle
		}
		if b.KeepNoteClientID != "" {
			entry[bindingKeepNoteClientIDField] = b.KeepNoteClientID
		}
		if !b.SubscribedAt.IsZero() {
			entry[bindingBoundAtField] = b.SubscribedAt.UTC().Format(time.RFC3339)
		}
		if b.KeepCursor != "" {
			entry[bindingKeepCursorField] = b.KeepCursor
		}
		if b.TruncatedTickStreak > 0 {
			entry[bindingTruncatedTickStreakField] = b.TruncatedTickStreak
		}
		if b.MigratedFingerprints {
			entry[bindingMigratedFingerprintsField] = true
		}
		if len(b.ItemIDMap) > 0 {
			m := make(map[string]any, len(b.ItemIDMap))
			for uid, ib := range b.ItemIDMap {
				m[uid] = encodeItemBinding(ib)
			}
			entry[bindingItemIDMapField] = m
		}
		if len(b.LabelIDs) > 0 {
			m := make(map[string]any, len(b.LabelIDs))
			for name, mainID := range b.LabelIDs {
				m[name] = mainID
			}
			entry[bindingLabelIDsField] = m
		}
		out[i] = entry
	}
	return out
}

// encodeItemBinding writes one ItemMapping as a frontmatter map. Always
// uses the new structured shape; legacy decoding still accepts the flat
// string shape for backwards compatibility with files written before
// this rewrite.
func encodeItemBinding(ib ItemMapping) map[string]any {
	out := map[string]any{
		itemBindingServerIDField: ib.ServerID,
	}
	if ib.SyncedText != "" {
		out[itemBindingSyncedTextField] = ib.SyncedText
	}
	if ib.SyncedChecked {
		out[itemBindingSyncedCheckedField] = true
	}
	if ib.SyncedSortValue != "" {
		out[itemBindingSyncedSortValueField] = ib.SyncedSortValue
	}
	if ib.LastObservedWikiText != "" {
		out[itemBindingLastObservedWikiTextField] = ib.LastObservedWikiText
	}
	if ib.LastObservedWikiChecked {
		out[itemBindingLastObservedWikiCheckedField] = true
	}
	if ib.LastObservedWikiSortValue != "" {
		out[itemBindingLastObservedWikiSortValueField] = ib.LastObservedWikiSortValue
	}
	if ib.PushFailureCount > 0 {
		out[itemBindingPushFailureCountField] = ib.PushFailureCount
	}
	if ib.LastFailureCode != "" {
		out[itemBindingLastFailureCodeField] = ib.LastFailureCode
	}
	if !ib.NextAttemptAt.IsZero() {
		out[itemBindingNextAttemptAtField] = ib.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	if ib.BaseVersion != "" {
		out[itemBindingBaseVersionField] = ib.BaseVersion
	}
	if ib.ClientID != "" {
		out[itemBindingClientIDField] = ib.ClientID
	}
	return out
}

// decodeItemIDMap reads the per-binding wiki-uid → ItemMapping map.
// Accepts BOTH the old flat shape (map[uid]string of serverID) for
// backwards compatibility with bindings persisted before this rewrite,
// AND the new structured shape (map[uid]map[field]any with synced_*,
// last_observed_wiki_*, push_failure_count, last_failure_code).
//
// Old-shape entries decode as ItemMapping{ServerID: v, …zero…} —
// the eager migration job populates the rest.
func decodeItemIDMap(raw any) (map[string]ItemMapping, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map, got %T", raw)
	}
	out := make(map[string]ItemMapping, len(m))
	for k, v := range m {
		switch typed := v.(type) {
		case string:
			// Legacy flat shape: just the serverID.
			out[k] = ItemMapping{ServerID: typed}
		case map[string]any:
			// New structured shape.
			nextAttemptAt, err := parseTime(getString(typed, itemBindingNextAttemptAtField))
			if err != nil {
				return nil, fmt.Errorf("key %q next_attempt_at: %w", k, err)
			}
			out[k] = ItemMapping{
				ServerID:                  getString(typed, itemBindingServerIDField),
				SyncedText:                getString(typed, itemBindingSyncedTextField),
				SyncedChecked:             getBool(typed, itemBindingSyncedCheckedField),
				SyncedSortValue:           getString(typed, itemBindingSyncedSortValueField),
				LastObservedWikiText:      getString(typed, itemBindingLastObservedWikiTextField),
				LastObservedWikiChecked:   getBool(typed, itemBindingLastObservedWikiCheckedField),
				LastObservedWikiSortValue: getString(typed, itemBindingLastObservedWikiSortValueField),
				PushFailureCount:          getInt(typed, itemBindingPushFailureCountField),
				LastFailureCode:           getString(typed, itemBindingLastFailureCodeField),
				NextAttemptAt:             nextAttemptAt,
				BaseVersion:               getString(typed, itemBindingBaseVersionField),
				ClientID:                  getString(typed, itemBindingClientIDField),
			}
		default:
			return nil, fmt.Errorf("key %q value is %T, expected string or map", k, v)
		}
	}
	return out, nil
}

// decodeLabelIDs reads the per-binding label-name → MainID map.
// Legacy bindings without a label_ids key decode as a nil map; the
// next sync's pull will populate it.
func decodeLabelIDs(raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map, got %T", raw)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("key %q value is %T, expected string", k, v)
		}
		out[k] = s
	}
	return out, nil
}

// connectorMap returns the wiki.connectors.google_keep submap, or nil if
// any link is missing.
func connectorMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		return nil
	}
	connectors, ok := wiki[connectorsKey].(map[string]any)
	if !ok {
		return nil
	}
	gk, ok := connectors[googleKeepKey].(map[string]any)
	if !ok {
		return nil
	}
	return gk
}

// ensureConnectorMap returns the wiki.connectors.google_keep submap,
// creating any missing parent maps.
func ensureConnectorMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		wiki = make(map[string]any)
		fm[wikiKey] = wiki
	}
	connectors, ok := wiki[connectorsKey].(map[string]any)
	if !ok {
		connectors = make(map[string]any)
		wiki[connectorsKey] = connectors
	}
	gk, ok := connectors[googleKeepKey].(map[string]any)
	if !ok {
		gk = make(map[string]any)
		connectors[googleKeepKey] = gk
	}
	return gk
}

// getString reads a string field from a frontmatter map; non-string or
// missing entries return empty string.
//
//revive:disable-next-line:unchecked-type-assertion
func getString(m map[string]any, key string) string { s, _ := m[key].(string); return s }

func getInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func getInt(m map[string]any, key string) int {
	return int(getInt64(m, key))
}

func getBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// parseTime accepts an empty string (returns zero, no error — "absent")
// or an RFC3339 string. A non-empty unparseable input is an error: not
// the same thing as "absent", and silently collapsing the two would
// hide profile-page corruption (a recently-connected user would render
// as "never verified", etc.).
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a valid RFC3339 timestamp: %w", err)
	}
	return t.UTC(), nil
}
