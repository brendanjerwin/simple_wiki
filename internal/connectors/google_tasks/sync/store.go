package sync

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Errors returned by SubscriptionStore. RPC handlers map these to
// gRPC codes (NotFound, AlreadyExists, FailedPrecondition).
var (
	// ErrSubscriptionNotFound is returned by Remove/Find/Update when
	// no matching subscription exists for (page, list_name).
	ErrSubscriptionNotFound = errors.New("tasks bridge: subscription not found")

	// ErrAlreadySubscribedForChecklist is returned by AddSubscription
	// when the calling profile already owns a subscription for
	// (page, list_name). Per the plan's single-Subscription invariant
	// this is also enforced cross-profile via the LeaseTable, but the
	// per-profile check is the first line of defense.
	ErrAlreadySubscribedForChecklist = errors.New("tasks bridge: this checklist is already subscribed by you")

	// ErrConnectorNotConfigured is returned when a write requires an
	// active connector but the profile has no refresh_token.
	ErrConnectorNotConfigured = errors.New("tasks bridge: connector not configured for this user")
)

// Frontmatter path constants. The connector state lives at
//
//	wiki.connectors.google_tasks.*
//
// on the user's profile page. wiki.* is a reserved namespace
// (wikipage/reserved_namespaces.go) so generic frontmatter writes
// can't reach in here — the typed SubscriptionStore is the sole funnel.
const (
	wikiKey         = "wiki"
	connectorsKey   = "connectors"
	googleTasksKey  = "google_tasks"
	subscriptionKey = "subscriptions"

	emailField          = "email"
	refreshTokenField   = "refresh_token"
	connectedAtField    = "connected_at"
	lastVerifiedAtField = "last_verified_at"

	subPageField                 = "page"
	subListNameField             = "list_name"
	subRemoteListIDField         = "remote_list_id"
	subRemoteListTitleField      = "remote_list_title"
	subItemIDMapField            = "item_id_map"
	subItemEtagsField            = "item_etags"
	subLastUpdatedMinField       = "last_updated_min"
	subLastSuccessfulSyncAtField = "last_successful_sync_at"
	subStateField                = "state"
	subPausedReasonField         = "paused_reason"
	subPausedAtField             = "paused_at"
	subSubscribedAtField         = "subscribed_at"
)

// SubscriptionStore is the typed funnel for connector-state writes on
// profile pages. Mirrors Keep's BindingStore: per-profile mutex, all
// writes through wikipage.PageReaderMutator, no direct frontmatter
// mutation outside this package.
type SubscriptionStore struct {
	pages    wikipage.PageReaderMutator
	profilMu sync.Map // keyed by profileID; values *sync.Mutex
}

// NewSubscriptionStore constructs a SubscriptionStore.
func NewSubscriptionStore(pages wikipage.PageReaderMutator) (*SubscriptionStore, error) {
	if pages == nil {
		return nil, errors.New("tasks bridge: pages must not be nil")
	}
	return &SubscriptionStore{pages: pages}, nil
}

// LoadState reads the full connector state for the given profile
// page. Missing profile or absent connector frontmatter both return
// a zero ConnectorState (no error) so callers can render "not
// connected".
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

// SaveState overwrites the entire connector state on the profile
// page. Used by Connect (after OAuth completion) and Disconnect.
func (s *SubscriptionStore) SaveState(profileID wikipage.PageIdentifier, state ConnectorState) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	return s.writeStateLocked(profileID, state)
}

// AddSubscription appends a new subscription to the calling user's
// profile. Errors with ErrAlreadySubscribedForChecklist if the
// (page, list_name) is already in this profile's subscription list;
// callers MUST also coordinate with the LeaseTable for cross-profile
// exclusivity (per ADR-0011).
func (s *SubscriptionStore) AddSubscription(profileID wikipage.PageIdentifier, sub Subscription) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	state, err := s.loadStateLocked(profileID)
	if err != nil {
		return err
	}
	if !state.IsConfigured() {
		return ErrConnectorNotConfigured
	}
	for _, existing := range state.Subscriptions {
		if existing.Page == sub.Page && existing.ListName == sub.ListName {
			return ErrAlreadySubscribedForChecklist
		}
	}
	state.Subscriptions = append(state.Subscriptions, sub)
	return s.writeStateLocked(profileID, state)
}

// RemoveSubscription removes the calling user's subscription for
// (page, listName). Returns ErrSubscriptionNotFound if no match.
func (s *SubscriptionStore) RemoveSubscription(profileID wikipage.PageIdentifier, page, listName string) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	state, err := s.loadStateLocked(profileID)
	if err != nil {
		return err
	}
	for i, existing := range state.Subscriptions {
		if existing.Page == page && existing.ListName == listName {
			state.Subscriptions = append(state.Subscriptions[:i], state.Subscriptions[i+1:]...)
			return s.writeStateLocked(profileID, state)
		}
	}
	return ErrSubscriptionNotFound
}

// FindSubscription returns the calling user's subscription for
// (page, listName), if any. The boolean second return is "found".
func (s *SubscriptionStore) FindSubscription(profileID wikipage.PageIdentifier, page, listName string) (Subscription, bool, error) {
	state, err := s.LoadState(profileID)
	if err != nil {
		return Subscription{}, false, err
	}
	for _, sub := range state.Subscriptions {
		if sub.Page == page && sub.ListName == listName {
			return sub, true, nil
		}
	}
	return Subscription{}, false, nil
}

// UpdateSubscription replaces the existing subscription identified
// by (page, listName) with the supplied value. Errors with
// ErrSubscriptionNotFound if no match.
//
// The fields Page and ListName on the supplied Subscription MUST
// match the lookup key — this method is for updating mutable state
// (cursor, item_id_map, paused state) on an already-located record,
// not for renaming subscriptions. Mismatches are programming bugs;
// they return an error rather than silently overwriting.
func (s *SubscriptionStore) UpdateSubscription(profileID wikipage.PageIdentifier, sub Subscription) error {
	unlock := s.lockProfile(profileID)
	defer unlock()

	state, err := s.loadStateLocked(profileID)
	if err != nil {
		return err
	}
	for i, existing := range state.Subscriptions {
		if existing.Page == sub.Page && existing.ListName == sub.ListName {
			state.Subscriptions[i] = sub
			return s.writeStateLocked(profileID, state)
		}
	}
	return ErrSubscriptionNotFound
}

// WithProfileLock runs fn while holding the per-profile mutex. fn
// must use LoadStateLocked / SaveStateLocked for any state access
// (the regular LoadState / SaveState would deadlock).
func (s *SubscriptionStore) WithProfileLock(profileID wikipage.PageIdentifier, fn func() error) error {
	unlock := s.lockProfile(profileID)
	defer unlock()
	return fn()
}

// LoadStateLocked reads state without acquiring the per-profile
// mutex. Caller MUST hold the lock (typically via WithProfileLock).
func (s *SubscriptionStore) LoadStateLocked(profileID wikipage.PageIdentifier) (ConnectorState, error) {
	return s.loadStateLocked(profileID)
}

// SaveStateLocked overwrites state without acquiring the per-profile
// mutex. Caller MUST hold the lock.
func (s *SubscriptionStore) SaveStateLocked(profileID wikipage.PageIdentifier, state ConnectorState) error {
	return s.writeStateLocked(profileID, state)
}

// loadStateLocked is the unexported, lock-not-acquiring shape used
// by every public method (which all hold the per-profile mutex).
func (s *SubscriptionStore) loadStateLocked(profileID wikipage.PageIdentifier) (ConnectorState, error) {
	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConnectorState{}, nil
		}
		return ConnectorState{}, err
	}
	return decodeState(fm)
}

// writeStateLocked encodes state and writes it to the profile page.
// Caller MUST hold the per-profile mutex.
func (s *SubscriptionStore) writeStateLocked(profileID wikipage.PageIdentifier, state ConnectorState) error {
	fm, err := s.readFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	encodeState(fm, state)
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("tasks bridge: write frontmatter: %w", err)
	}
	return nil
}

// readFrontMatter reads the page's frontmatter, returning
// os.ErrNotExist for missing pages so callers can branch.
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

// lockProfile acquires the per-profile mutex.
func (s *SubscriptionStore) lockProfile(profileID wikipage.PageIdentifier) func() {
	v, _ := s.profilMu.LoadOrStore(profileID, &sync.Mutex{})
	// INVARIANT ASSERTION: every value stored in profilMu is *sync.Mutex.
	mu, ok := v.(*sync.Mutex)
	if !ok {
		panic(fmt.Sprintf("tasks bridge: profilMu held a %T, expected *sync.Mutex — programming bug", v))
	}
	mu.Lock()
	return mu.Unlock
}

// --- codec ----------------------------------------------------------------

func decodeState(fm wikipage.FrontMatter) (ConnectorState, error) {
	connector := connectorMap(fm)
	if connector == nil {
		return ConnectorState{}, nil
	}
	connectedAt, err := parseTime(getString(connector, connectedAtField))
	if err != nil {
		return ConnectorState{}, fmt.Errorf("wiki.connectors.google_tasks.connected_at: %w", err)
	}
	lastVerifiedAt, err := parseTime(getString(connector, lastVerifiedAtField))
	if err != nil {
		return ConnectorState{}, fmt.Errorf("wiki.connectors.google_tasks.last_verified_at: %w", err)
	}
	subs, err := decodeSubscriptions(connector[subscriptionKey])
	if err != nil {
		return ConnectorState{}, err
	}
	return ConnectorState{
		Email:          getString(connector, emailField),
		RefreshToken:   getString(connector, refreshTokenField),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
		Subscriptions:  subs,
	}, nil
}

func decodeSubscriptions(raw any) ([]Subscription, error) {
	if raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions is %T, expected list", raw)
	}
	out := make([]Subscription, 0, len(arr))
	for i, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d] is %T, expected map", i, entry)
		}
		idMap, err := decodeStringMap(m[subItemIDMapField])
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].item_id_map: %w", i, err)
		}
		etags, err := decodeStringMap(m[subItemEtagsField])
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].item_etags: %w", i, err)
		}
		lastUpdatedMin, err := parseTime(getString(m, subLastUpdatedMinField))
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].last_updated_min: %w", i, err)
		}
		lastSync, err := parseTime(getString(m, subLastSuccessfulSyncAtField))
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].last_successful_sync_at: %w", i, err)
		}
		pausedAt, err := parseTime(getString(m, subPausedAtField))
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].paused_at: %w", i, err)
		}
		subscribedAt, err := parseTime(getString(m, subSubscribedAtField))
		if err != nil {
			return nil, fmt.Errorf("wiki.connectors.google_tasks.subscriptions[%d].subscribed_at: %w", i, err)
		}
		state := SubscriptionState(getString(m, subStateField))
		if state == "" {
			state = SubscriptionStateActive
		}
		out = append(out, Subscription{
			Page:                 getString(m, subPageField),
			ListName:             getString(m, subListNameField),
			RemoteListID:         getString(m, subRemoteListIDField),
			RemoteListTitle:      getString(m, subRemoteListTitleField),
			ItemIDMap:            idMap,
			ItemEtags:            etags,
			LastUpdatedMin:       lastUpdatedMin,
			LastSuccessfulSyncAt: lastSync,
			State:                state,
			PausedReason:         getString(m, subPausedReasonField),
			PausedAt:             pausedAt,
			SubscribedAt:         subscribedAt,
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
	if state.RefreshToken != "" {
		connector[refreshTokenField] = state.RefreshToken
	} else {
		delete(connector, refreshTokenField)
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
	if len(state.Subscriptions) > 0 {
		connector[subscriptionKey] = encodeSubscriptions(state.Subscriptions)
	} else {
		delete(connector, subscriptionKey)
	}
}

func encodeSubscriptions(subs []Subscription) []any {
	out := make([]any, len(subs))
	for i, sub := range subs {
		entry := map[string]any{
			subPageField:         sub.Page,
			subListNameField:     sub.ListName,
			subRemoteListIDField: sub.RemoteListID,
		}
		if sub.RemoteListTitle != "" {
			entry[subRemoteListTitleField] = sub.RemoteListTitle
		}
		if len(sub.ItemIDMap) > 0 {
			entry[subItemIDMapField] = stringMapToAnyMap(sub.ItemIDMap)
		}
		if len(sub.ItemEtags) > 0 {
			entry[subItemEtagsField] = stringMapToAnyMap(sub.ItemEtags)
		}
		if !sub.LastUpdatedMin.IsZero() {
			entry[subLastUpdatedMinField] = sub.LastUpdatedMin.UTC().Format(time.RFC3339)
		}
		if !sub.LastSuccessfulSyncAt.IsZero() {
			entry[subLastSuccessfulSyncAtField] = sub.LastSuccessfulSyncAt.UTC().Format(time.RFC3339)
		}
		if sub.State != "" && sub.State != SubscriptionStateActive {
			entry[subStateField] = string(sub.State)
		}
		if sub.PausedReason != "" {
			entry[subPausedReasonField] = sub.PausedReason
		}
		if !sub.PausedAt.IsZero() {
			entry[subPausedAtField] = sub.PausedAt.UTC().Format(time.RFC3339)
		}
		if !sub.SubscribedAt.IsZero() {
			entry[subSubscribedAtField] = sub.SubscribedAt.UTC().Format(time.RFC3339)
		}
		out[i] = entry
	}
	return out
}

func stringMapToAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func decodeStringMap(raw any) (map[string]string, error) {
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

func connectorMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		return nil
	}
	conns, ok := wiki[connectorsKey].(map[string]any)
	if !ok {
		return nil
	}
	gt, ok := conns[googleTasksKey].(map[string]any)
	if !ok {
		return nil
	}
	return gt
}

func ensureConnectorMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		wiki = make(map[string]any)
		fm[wikiKey] = wiki
	}
	conns, ok := wiki[connectorsKey].(map[string]any)
	if !ok {
		conns = make(map[string]any)
		wiki[connectorsKey] = conns
	}
	gt, ok := conns[googleTasksKey].(map[string]any)
	if !ok {
		gt = make(map[string]any)
		conns[googleTasksKey] = gt
	}
	return gt
}

// getString reads a string field from a frontmatter map; non-string
// or missing entries return empty string.
//
//revive:disable-next-line:unchecked-type-assertion
func getString(m map[string]any, key string) string { s, _ := m[key].(string); return s }

// parseTime accepts an empty string (returns zero, no error —
// "absent") or an RFC3339 string.
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
