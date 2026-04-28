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
type Binding struct {
	Page          string
	ListName      string
	KeepNoteID    string
	KeepNoteTitle string
	BoundAt       time.Time
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
	return decodeState(fm), nil
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
	state := decodeState(fm)
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
	state := decodeState(fm)

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
	mu, ok := v.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
	}
	mu.Lock()
	return mu.Unlock
}

// --- codec ----------------------------------------------------------------

func decodeState(fm wikipage.FrontMatter) ConnectorState {
	connector := connectorMap(fm)
	if connector == nil {
		return ConnectorState{}
	}
	return ConnectorState{
		Email:               getString(connector, emailField),
		MasterToken:         getString(connector, masterTokenField),
		ConnectedAt:         parseTime(getString(connector, connectedAtField)),
		LastVerifiedAt:      parseTime(getString(connector, lastVerifiedAtField)),
		PollIntervalSeconds: getInt64(connector, pollIntervalSecondsField),
		Bindings:            decodeBindings(connector[bindingsField]),
	}
}

func decodeBindings(raw any) []Binding {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Binding, 0, len(arr))
	for _, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, Binding{
			Page:          getString(m, bindingPageField),
			ListName:      getString(m, bindingListNameField),
			KeepNoteID:    getString(m, bindingKeepNoteIDField),
			KeepNoteTitle: getString(m, bindingKeepNoteTitleField),
			BoundAt:       parseTime(getString(m, bindingBoundAtField)),
		})
	}
	return out
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
		out[i] = entry
	}
	return out
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

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
