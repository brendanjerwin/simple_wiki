// Package bridge owns the wiki-side of the Google Keep bridge: per-user
// connector state on profile pages, sync logic, and field mapping. The
// wire-protocol port lives one directory up under internal/keep/protocol.
package bridge

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Binding is a single user's link from a wiki checklist (page + list_name)
// to a Keep note in their account.
//
// ItemIDMap is the per-binding mapping from wiki item UID to the Keep
// node's serverID. Populated at bind time by the bundled CreateListWithItems
// path and updated on every outbound sync — the sync engine uses it to
// decide "is this wiki item a fresh push (uid not in map) or an update
// (uid already in map, push with parent_server_id)?" Without this map
// we'd have no identity correlation between sides and would have to
// recreate the whole list on every change.
type Binding struct {
	Page          string
	ListName      string
	KeepNoteID    string
	KeepNoteTitle string
	BoundAt       time.Time
	ItemIDMap     map[string]string
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
	Bindings            []Binding
}

// IsConfigured reports whether the connector has a master token (i.e. the
// user has completed the connect flow). LoadState's zero return reads as
// "not configured" via this check.
func (s ConnectorState) IsConfigured() bool { return s.MasterToken != "" }

// Errors returned by BindingStore. RPC handlers map these to gRPC codes.
var (
	// ErrAlreadyBoundForChecklist is returned by Add when the calling
	// user already has a binding for (page, list_name). Per the plan's
	// per-user collision matrix.
	ErrAlreadyBoundForChecklist = errors.New("keep bridge: this checklist is already bound by you")

	// ErrAlreadyBoundToKeepNote is returned by Add when the calling user
	// already has a binding to the same Keep note (different checklist).
	ErrAlreadyBoundToKeepNote = errors.New("keep bridge: this Keep note is already bound by you")

	// ErrBindingNotFound is returned by Remove when no binding matches.
	ErrBindingNotFound = errors.New("keep bridge: binding not found")

	// ErrConnectorNotConfigured is returned by methods that require an
	// active connector when the profile has no master_token.
	ErrConnectorNotConfigured = errors.New("keep bridge: connector not configured for this user")
)

// Frontmatter path constants. The connector state lives at
//   wiki.connectors.google_keep.*
// on the user's profile page. wiki.* is a reserved top-level namespace
// (wikipage/reserved_namespaces.go) so generic MergeFrontmatter rejects
// writes here — the typed BindingStore is the sole funnel.
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

	bindingPageField          = "page"
	bindingListNameField      = "list_name"
	bindingKeepNoteIDField    = "keep_note_id"
	bindingKeepNoteTitleField = "keep_note_title"
	bindingBoundAtField       = "bound_at"
	bindingItemIDMapField     = "item_id_map"
)

// BindingStore is the typed funnel for connector-state writes on profile
// pages. Mirrors the checklistmutator pattern: per-page mutex, all writes
// through wikipage.PageReaderMutator, no direct frontmatter mutation
// outside this package.
type BindingStore struct {
	pages    wikipage.PageReaderMutator
	profilMu sync.Map // keyed by profileID; values *sync.Mutex
}

// NewBindingStore constructs a BindingStore.
func NewBindingStore(pages wikipage.PageReaderMutator) *BindingStore {
	return &BindingStore{pages: pages}
}

// LoadState reads the full connector state for the given profile page.
// Missing profile or absent connector frontmatter both return a zero
// ConnectorState (no error) so callers can render "not connected".
func (s *BindingStore) LoadState(profileID wikipage.PageIdentifier) (ConnectorState, error) {
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
func (s *BindingStore) SaveState(profileID wikipage.PageIdentifier, state ConnectorState) error {
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

// AddBinding appends a new binding to the calling user's profile after
// running the per-user collision matrix.
func (s *BindingStore) AddBinding(profileID wikipage.PageIdentifier, b Binding) error {
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

	for _, existing := range state.Bindings {
		if existing.Page == b.Page && existing.ListName == b.ListName {
			return ErrAlreadyBoundForChecklist
		}
		if existing.KeepNoteID == b.KeepNoteID {
			return ErrAlreadyBoundToKeepNote
		}
	}
	state.Bindings = append(state.Bindings, b)

	encodeState(fm, state)
	return s.writeFrontMatter(profileID, fm)
}

// RemoveBinding removes the calling user's binding for (page, listName).
// Returns ErrBindingNotFound if no match.
func (s *BindingStore) RemoveBinding(profileID wikipage.PageIdentifier, page, listName string) error {
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

	for i, existing := range state.Bindings {
		if existing.Page == page && existing.ListName == listName {
			state.Bindings = append(state.Bindings[:i], state.Bindings[i+1:]...)
			encodeState(fm, state)
			return s.writeFrontMatter(profileID, fm)
		}
	}
	return ErrBindingNotFound
}

// FindBinding returns the calling user's binding for (page, listName), if
// any. The boolean second return is "found".
func (s *BindingStore) FindBinding(profileID wikipage.PageIdentifier, page, listName string) (Binding, bool, error) {
	state, err := s.LoadState(profileID)
	if err != nil {
		return Binding{}, false, err
	}
	for _, b := range state.Bindings {
		if b.Page == page && b.ListName == listName {
			return b, true, nil
		}
	}
	return Binding{}, false, nil
}

// readFrontMatter reads the page's frontmatter, returning os.ErrNotExist
// for missing pages so callers can branch.
func (s *BindingStore) readFrontMatter(profileID wikipage.PageIdentifier) (wikipage.FrontMatter, error) {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	return fm, nil
}

func (s *BindingStore) writeFrontMatter(profileID wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("keep bridge: write frontmatter: %w", err)
	}
	return nil
}

// lockProfile acquires the per-profile mutex.
func (s *BindingStore) lockProfile(profileID wikipage.PageIdentifier) func() {
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
		Bindings:            bindings,
	}, nil
}

func decodeBindings(raw any) ([]Binding, error) {
	if raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("wiki.connectors.google_keep.bindings is %T, expected list", raw)
	}
	out := make([]Binding, 0, len(arr))
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
		out = append(out, Binding{
			Page:          getString(m, bindingPageField),
			ListName:      getString(m, bindingListNameField),
			KeepNoteID:    getString(m, bindingKeepNoteIDField),
			KeepNoteTitle: getString(m, bindingKeepNoteTitleField),
			BoundAt:       boundAt,
			ItemIDMap:     idMap,
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
	if len(state.Bindings) > 0 {
		connector[bindingsField] = encodeBindings(state.Bindings)
	} else {
		delete(connector, bindingsField)
	}
}

func encodeBindings(bindings []Binding) []any {
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
		if !b.BoundAt.IsZero() {
			entry[bindingBoundAtField] = b.BoundAt.UTC().Format(time.RFC3339)
		}
		if len(b.ItemIDMap) > 0 {
			m := make(map[string]any, len(b.ItemIDMap))
			for uid, serverID := range b.ItemIDMap {
				m[uid] = serverID
			}
			entry[bindingItemIDMapField] = m
		}
		out[i] = entry
	}
	return out
}

// decodeItemIDMap reads the per-binding wiki-uid → keep-serverID map.
// Frontmatter loaders typically hand back map[string]any with string
// values; this checks each value is a string and rejects any other
// shape rather than silently coercing.
func decodeItemIDMap(raw any) (map[string]string, error) {
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
