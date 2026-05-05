package engine

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// FrontmatterReadWriter is the wiki-side seam the FrontmatterBindingStore
// uses to read and write frontmatter on profile pages. The production
// wiring satisfies this with the wiki's PageReaderMutator (which does
// the same thing); declaring a narrow interface here keeps the engine
// package free of larger Page* concerns it doesn't need (markdown,
// page open, page delete) and makes the binding store unit-testable
// against an in-memory fake.
type FrontmatterReadWriter interface {
	// ReadFrontMatter reads the frontmatter on the given page. Missing
	// pages return os.ErrNotExist; the binding store treats that case
	// as "no bindings" rather than an error.
	ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)

	// WriteFrontMatter overwrites the frontmatter on the given page.
	// The store always reads-modify-writes the entire frontmatter map
	// to preserve fields the engine doesn't own (other connectors,
	// non-connector wiki.* state, user fields).
	WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error
}

// ProfileLister enumerates every profile page that has at least one
// binding for the given connector kind. The production wiring
// satisfies this with the wiki's frontmatter index (queries the
// wiki.connectors.<kind>.bindings key existence). Tests inject a
// programmable fake.
//
// The lister is its own seam (rather than a method on the
// FrontmatterReadWriter) because the production read+write port and
// the production index are different objects in the wiki.
type ProfileLister interface {
	// ListProfilesWithKey returns every page whose frontmatter has a
	// value at the given dotted key path. Used by the binding store's
	// ListAllProfilesWithBindings to find profiles with new-shape
	// bindings[] AND legacy subscriptions[] (until the Phase 7
	// migration sweeps the latter).
	ListProfilesWithKey(dottedKeyPath wikipage.DottedKeyPath) []wikipage.PageIdentifier
}

// FrontmatterBindingStore is the production engine.BindingStore. It
// persists bindings on each profile page under the
// wiki.connectors.<kind>.bindings[] key, with each adapter's per-binding
// state in a nested adapter_state subtree (per ADR-0016).
//
// Defensive read path: LoadBindings / FindBinding read both the new
// shape and the legacy wiki.connectors.<kind>.subscriptions[] shape;
// the new shape wins when both are present. The Phase 7 eager
// migration rewrites legacy data on first read; until then dual-read
// is the load-bearing guard against breaking existing bindings.
//
// Concurrency: a per-profile sync.Mutex serializes critical sections
// (Save, Delete, WithProfileLock). The Bind / Unbind ceremonies in
// engine/bind.go and engine/unbind.go rely on WithProfileLock to
// enforce the mutex+fan-out-re-read invariant per ADR-0011.
type FrontmatterBindingStore struct {
	pages    FrontmatterReadWriter
	profiles ProfileLister
	logger   Logger

	// profileLocks holds one *sync.Mutex per profile. Lazily created
	// in lockProfile. Keys are wikipage.PageIdentifier; values are
	// *sync.Mutex. Mirrors the legacy SubscriptionStore pattern.
	profileLocks sync.Map
}

// NewFrontmatterBindingStore wires the production binding store. Every
// dependency is required; nil collaborators are wiring bugs and return
// an error rather than crashing later.
func NewFrontmatterBindingStore(pages FrontmatterReadWriter, profiles ProfileLister, logger Logger) (*FrontmatterBindingStore, error) {
	if pages == nil {
		return nil, errors.New("connectors/engine: pages must not be nil")
	}
	if profiles == nil {
		return nil, errors.New("connectors/engine: profiles must not be nil")
	}
	if logger == nil {
		return nil, errors.New("connectors/engine: logger must not be nil")
	}
	return &FrontmatterBindingStore{
		pages:    pages,
		profiles: profiles,
		logger:   logger,
	}, nil
}

// Frontmatter path constants. The connector state lives at
//
//	wiki.connectors.<kind>.*
//
// on the user's profile page. wiki.* is a reserved namespace
// (wikipage/reserved_namespaces.go) so generic frontmatter writes
// can't reach in here — the typed binding store is the sole funnel.
const (
	fmKeyWiki         = "wiki"
	fmKeyConnectors   = "connectors"
	fmKeyBindings     = "bindings"      // new shape
	fmKeySubscriptions = "subscriptions" // legacy shape (dual-read only)
	fmKeyAdapterState = "adapter_state"

	// Engine-owned per-binding fields (new shape). Top-level on the
	// binding entry; round-tripped verbatim by Save and Load.
	fmKeyPage                 = "page"
	fmKeyListName             = "list_name"
	fmKeyRemoteHandle         = "remote_handle"
	fmKeyRemoteListTitle      = "remote_list_title"
	fmKeyState                = "state"
	fmKeyPausedReason         = "paused_reason"
	fmKeyPausedAt             = "paused_at"
	fmKeyBoundAt              = "bound_at"
	fmKeyLastSyncedSeq        = "last_synced_seq"
	fmKeyLastSuccessfulSyncAt = "last_successful_sync_at"

	// Legacy-shape engine field aliases (read-only; LoadBindings maps
	// these to the new-shape fields when dual-reading).
	fmLegacyKeyRemoteListID = "remote_list_id"
	fmLegacyKeySubscribedAt = "subscribed_at"
)

// engineOwnedLegacyKeys is the set of keys on a legacy subscription
// entry that LoadBindings interprets as engine-owned (i.e. they map to
// fields on the Binding struct rather than landing in AdapterState).
// Anything NOT in this set on a legacy entry is preserved in
// AdapterState verbatim. This is the load-bearing rule that lets us
// safely round-trip Tasks's adapter-specific bookkeeping
// (item_id_map, item_etags, last_updated_min, synced_items) and Keep's
// equivalents (item_mapping, label_ids, keep_cursor, ...) through the
// dual-read without per-adapter conditionals here.
var engineOwnedLegacyKeys = map[string]struct{}{
	fmKeyPage:                 {},
	fmKeyListName:             {},
	fmKeyRemoteHandle:         {},
	fmLegacyKeyRemoteListID:   {},
	fmKeyRemoteListTitle:      {},
	fmKeyState:                {},
	fmKeyPausedReason:         {},
	fmKeyPausedAt:             {},
	fmKeyBoundAt:              {},
	fmLegacyKeySubscribedAt:   {},
	fmKeyLastSyncedSeq:        {},
	fmKeyLastSuccessfulSyncAt: {},
}

// LoadBindings implements BindingStore.LoadBindings.
//
// Reads the new wiki.connectors.<kind>.bindings[] shape first; if
// absent or empty, falls back to the legacy
// wiki.connectors.<kind>.subscriptions[] shape and translates each
// entry. The new shape wins when both keys are present (transitional
// state during Phase 7 migration).
func (s *FrontmatterBindingStore) LoadBindings(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind) ([]connectors.Binding, error) {
	connector, err := s.readConnectorMap(profileID, kind)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, nil
	}
	return decodeBindingsList(profileID, connector)
}

// FindBinding implements BindingStore.FindBinding.
func (s *FrontmatterBindingStore) FindBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) (connectors.Binding, bool, error) {
	bindings, err := s.LoadBindings(profileID, kind)
	if err != nil {
		return connectors.Binding{}, false, err
	}
	for _, b := range bindings {
		if b.Page == page && b.ListName == listName {
			return b, true, nil
		}
	}
	return connectors.Binding{}, false, nil
}

// SaveBinding implements BindingStore.SaveBinding. Always writes the
// new shape (bindings[] with adapter_state subtree). Does NOT touch
// the legacy subscriptions[] key — the Phase 7 eager migration sweeps
// that on first read.
//
// A missing profile page is treated as a fresh write: the store
// constructs an empty frontmatter map and proceeds to write the new
// binding. Whether the page-store creates the page on demand or
// rejects the write is the page store's contract; this method does
// not pre-validate page existence.
func (s *FrontmatterBindingStore) SaveBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, binding connectors.Binding) error {
	fm, err := s.readFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("connectors/engine: read frontmatter for %s: %w", profileID, err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	connector := ensureConnectorMap(fm, kind)
	rawList := getAnySlice(connector, fmKeyBindings)
	updated := upsertBinding(rawList, binding)
	connector[fmKeyBindings] = updated
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("connectors/engine: write frontmatter for %s: %w", profileID, err)
	}
	return nil
}

// DeleteBinding implements BindingStore.DeleteBinding. No-op when the
// binding does not exist (per the BindingStore contract).
func (s *FrontmatterBindingStore) DeleteBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) error {
	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if fm == nil {
		return nil
	}
	connector := connectorMap(fm, kind)
	if connector == nil {
		return nil
	}
	rawList := getAnySlice(connector, fmKeyBindings)
	if len(rawList) == 0 {
		return nil
	}
	filtered := make([]any, 0, len(rawList))
	removed := false
	for _, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if getString(m, fmKeyPage) == page && getString(m, fmKeyListName) == listName {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return nil
	}
	if len(filtered) == 0 {
		delete(connector, fmKeyBindings)
	} else {
		connector[fmKeyBindings] = filtered
	}
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("connectors/engine: write frontmatter for %s: %w", profileID, err)
	}
	return nil
}

// WithProfileLock implements BindingStore.WithProfileLock. Acquires
// the per-profile mutex, runs fn, and releases. Concurrent callers on
// the same profile serialize; concurrent callers on different
// profiles run in parallel.
func (s *FrontmatterBindingStore) WithProfileLock(profileID wikipage.PageIdentifier, fn func() error) error {
	if fn == nil {
		return errors.New("connectors/engine: WithProfileLock fn must not be nil")
	}
	unlock := s.lockProfile(profileID)
	defer unlock()
	return fn()
}

// ListAllProfilesWithBindings implements
// BindingStore.ListAllProfilesWithBindings. Queries the frontmatter
// index for both the new bindings[] key and the legacy subscriptions[]
// key (until Phase 7's migration sweeps the latter), then deduplicates.
func (s *FrontmatterBindingStore) ListAllProfilesWithBindings(kind connectors.ConnectorKind) ([]wikipage.PageIdentifier, error) {
	newKey := connectorKeyPath(kind, fmKeyBindings)
	legacyKey := connectorKeyPath(kind, fmKeySubscriptions)
	seen := map[wikipage.PageIdentifier]struct{}{}
	var out []wikipage.PageIdentifier
	for _, p := range s.profiles.ListProfilesWithKey(newKey) {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range s.profiles.ListProfilesWithKey(legacyKey) {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

// --- internal helpers -----------------------------------------------------

// readFrontMatter reads the page's frontmatter, returning os.ErrNotExist
// for missing pages so callers can branch.
func (s *FrontmatterBindingStore) readFrontMatter(profileID wikipage.PageIdentifier) (wikipage.FrontMatter, error) {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return make(wikipage.FrontMatter), nil
	}
	return fm, nil
}

// readConnectorMap returns the wiki.connectors.<kind> subtree for the
// profile, or nil (no error) when the page is missing or has no
// frontmatter at that path.
func (s *FrontmatterBindingStore) readConnectorMap(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind) (map[string]any, error) {
	fm, err := s.readFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("connectors/engine: read frontmatter for %s: %w", profileID, err)
	}
	if fm == nil {
		return nil, nil
	}
	return connectorMap(fm, kind), nil
}

// decodeBindingsList walks the connector's bindings[] (new shape) or
// subscriptions[] (legacy) and produces engine Bindings. New shape
// wins when both are present.
func decodeBindingsList(profileID wikipage.PageIdentifier, connector map[string]any) ([]connectors.Binding, error) {
	if rawList, ok := connector[fmKeyBindings].([]any); ok && len(rawList) > 0 {
		return decodeNewShapeBindings(profileID, rawList)
	}
	if rawList, ok := connector[fmKeySubscriptions].([]any); ok && len(rawList) > 0 {
		return decodeLegacyBindings(profileID, rawList)
	}
	return nil, nil
}

func decodeNewShapeBindings(profileID wikipage.PageIdentifier, rawList []any) ([]connectors.Binding, error) {
	out := make([]connectors.Binding, 0, len(rawList))
	for i, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("connectors/engine: bindings[%d] is %T, expected map", i, entry)
		}
		b, err := decodeNewShapeBinding(profileID, m)
		if err != nil {
			return nil, fmt.Errorf("connectors/engine: bindings[%d]: %w", i, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func decodeNewShapeBinding(profileID wikipage.PageIdentifier, m map[string]any) (connectors.Binding, error) {
	pausedAt, err := parseRFC3339(getString(m, fmKeyPausedAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("paused_at: %w", err)
	}
	boundAt, err := parseRFC3339(getString(m, fmKeyBoundAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("bound_at: %w", err)
	}
	lastSync, err := parseRFC3339(getString(m, fmKeyLastSuccessfulSyncAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("last_successful_sync_at: %w", err)
	}
	state := connectors.BindingState(getString(m, fmKeyState))
	if state == "" {
		state = connectors.BindingStateActive
	}
	rawAdapterState := getStringMap(m, fmKeyAdapterState)
	return connectors.Binding{
		ProfileID:            profileID,
		Page:                 getString(m, fmKeyPage),
		ListName:             getString(m, fmKeyListName),
		RemoteHandle:         getString(m, fmKeyRemoteHandle),
		RemoteListTitle:      getString(m, fmKeyRemoteListTitle),
		LastSyncedSeq:        getInt64(m, fmKeyLastSyncedSeq),
		State:                state,
		PausedReason:         getString(m, fmKeyPausedReason),
		PausedAt:             pausedAt,
		BoundAt:              boundAt,
		LastSuccessfulSyncAt: lastSync,
		AdapterState:         connectors.AdapterState(deepCopyMap(rawAdapterState)),
	}, nil
}

// decodeLegacyBindings translates legacy subscriptions[] entries into
// engine Bindings. Engine-owned fields (page, list_name,
// remote_list_id, state, paused_*, subscribed_at, last_synced_seq)
// map to the corresponding Binding fields; everything else on the
// entry lands in AdapterState verbatim. This is the load-bearing
// guard that preserves Tasks's item_id_map / item_etags /
// last_updated_min / synced_items and Keep's analogous bookkeeping
// during the dual-read window.
func decodeLegacyBindings(profileID wikipage.PageIdentifier, rawList []any) ([]connectors.Binding, error) {
	out := make([]connectors.Binding, 0, len(rawList))
	for i, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("connectors/engine: subscriptions[%d] is %T, expected map", i, entry)
		}
		b, err := decodeLegacyBinding(profileID, m)
		if err != nil {
			return nil, fmt.Errorf("connectors/engine: subscriptions[%d]: %w", i, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func decodeLegacyBinding(profileID wikipage.PageIdentifier, m map[string]any) (connectors.Binding, error) {
	pausedAt, err := parseRFC3339(getString(m, fmKeyPausedAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("paused_at: %w", err)
	}
	boundAt, err := parseRFC3339(getString(m, fmLegacyKeySubscribedAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("subscribed_at: %w", err)
	}
	lastSync, err := parseRFC3339(getString(m, fmKeyLastSuccessfulSyncAt))
	if err != nil {
		return connectors.Binding{}, fmt.Errorf("last_successful_sync_at: %w", err)
	}
	state := connectors.BindingState(getString(m, fmKeyState))
	if state == "" {
		state = connectors.BindingStateActive
	}
	// Remote handle: prefer the new key if it's somehow present in a
	// legacy entry; otherwise the legacy remote_list_id.
	remoteHandle := getString(m, fmKeyRemoteHandle)
	if remoteHandle == "" {
		remoteHandle = getString(m, fmLegacyKeyRemoteListID)
	}
	// Adapter state: every key on the entry NOT in the engine-owned
	// allowlist rides through verbatim (deep-copied so the loaded
	// Binding is independent of the source frontmatter map).
	adapterState := connectors.AdapterState{}
	for k, v := range m {
		if _, owned := engineOwnedLegacyKeys[k]; owned {
			continue
		}
		adapterState[k] = deepCopyAny(v)
	}
	if len(adapterState) == 0 {
		adapterState = nil
	}
	return connectors.Binding{
		ProfileID:            profileID,
		Page:                 getString(m, fmKeyPage),
		ListName:             getString(m, fmKeyListName),
		RemoteHandle:         remoteHandle,
		RemoteListTitle:      getString(m, fmKeyRemoteListTitle),
		LastSyncedSeq:        getInt64(m, fmKeyLastSyncedSeq),
		State:                state,
		PausedReason:         getString(m, fmKeyPausedReason),
		PausedAt:             pausedAt,
		BoundAt:              boundAt,
		LastSuccessfulSyncAt: lastSync,
		AdapterState:         adapterState,
	}, nil
}

// upsertBinding replaces the entry matching (page, listName) in
// rawList, or appends a new entry if none matches. Returns the
// updated list.
func upsertBinding(rawList []any, b connectors.Binding) []any {
	encoded := encodeBinding(b)
	for i, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if getString(m, fmKeyPage) == b.Page && getString(m, fmKeyListName) == b.ListName {
			rawList[i] = encoded
			return rawList
		}
	}
	return append(rawList, encoded)
}

// encodeBinding produces the new-shape map representation of a
// Binding, suitable for round-tripping through TOML/YAML frontmatter.
// Empty engine fields are omitted to keep the persisted shape compact;
// AdapterState is always rendered as a map (possibly empty) under
// adapter_state so a missing key on read unambiguously means "no
// adapter state yet" rather than "wrong shape."
func encodeBinding(b connectors.Binding) map[string]any {
	entry := map[string]any{
		fmKeyPage:         b.Page,
		fmKeyListName:     b.ListName,
		fmKeyRemoteHandle: b.RemoteHandle,
	}
	if b.RemoteListTitle != "" {
		entry[fmKeyRemoteListTitle] = b.RemoteListTitle
	}
	if b.State != "" {
		entry[fmKeyState] = string(b.State)
	}
	if b.PausedReason != "" {
		entry[fmKeyPausedReason] = b.PausedReason
	}
	if !b.PausedAt.IsZero() {
		entry[fmKeyPausedAt] = b.PausedAt.UTC().Format(time.RFC3339)
	}
	if !b.BoundAt.IsZero() {
		entry[fmKeyBoundAt] = b.BoundAt.UTC().Format(time.RFC3339)
	}
	if !b.LastSuccessfulSyncAt.IsZero() {
		entry[fmKeyLastSuccessfulSyncAt] = b.LastSuccessfulSyncAt.UTC().Format(time.RFC3339)
	}
	if b.LastSyncedSeq > 0 {
		entry[fmKeyLastSyncedSeq] = b.LastSyncedSeq
	}
	if len(b.AdapterState) > 0 {
		entry[fmKeyAdapterState] = deepCopyMap(map[string]any(b.AdapterState))
	}
	return entry
}

// connectorMap returns wiki.connectors.<kind>, or nil if any segment
// is missing or wrong-typed.
func connectorMap(fm wikipage.FrontMatter, kind connectors.ConnectorKind) map[string]any {
	wiki, ok := fm[fmKeyWiki].(map[string]any)
	if !ok {
		return nil
	}
	conns, ok := wiki[fmKeyConnectors].(map[string]any)
	if !ok {
		return nil
	}
	c, ok := conns[string(kind)].(map[string]any)
	if !ok {
		return nil
	}
	return c
}

// ensureConnectorMap returns wiki.connectors.<kind>, creating any
// missing intermediate maps along the way. After this call the chain
// is guaranteed to exist on fm.
func ensureConnectorMap(fm wikipage.FrontMatter, kind connectors.ConnectorKind) map[string]any {
	wiki, ok := fm[fmKeyWiki].(map[string]any)
	if !ok {
		wiki = make(map[string]any)
		fm[fmKeyWiki] = wiki
	}
	conns, ok := wiki[fmKeyConnectors].(map[string]any)
	if !ok {
		conns = make(map[string]any)
		wiki[fmKeyConnectors] = conns
	}
	c, ok := conns[string(kind)].(map[string]any)
	if !ok {
		c = make(map[string]any)
		conns[string(kind)] = c
	}
	return c
}

// connectorKeyPath builds the dotted-key-path string the
// FrontmatterIndexQueryer accepts for ListProfilesWithKey.
func connectorKeyPath(kind connectors.ConnectorKind, leaf string) wikipage.DottedKeyPath {
	return fmt.Sprintf("%s.%s.%s.%s", fmKeyWiki, fmKeyConnectors, string(kind), leaf)
}

// lockProfile acquires the per-profile mutex. Returns the unlock
// closure. The mutex is lazily created on first acquisition and
// retained for the lifetime of the store.
//
// INVARIANT ASSERTION: every value stored in profileLocks is
// *sync.Mutex. Anything else is a programming bug.
func (s *FrontmatterBindingStore) lockProfile(profileID wikipage.PageIdentifier) func() {
	v, _ := s.profileLocks.LoadOrStore(profileID, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		panic(fmt.Sprintf("connectors/engine: profileLocks held a %T, expected *sync.Mutex — programming bug", v))
	}
	mu.Lock()
	return mu.Unlock
}

// --- primitive type helpers ----------------------------------------------

// getString reads a string field from a frontmatter map; non-string
// or missing entries return empty string.
func getString(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// getAnySlice reads a []any field from a frontmatter map; non-slice
// or missing entries return nil.
func getAnySlice(m map[string]any, key string) []any {
	v, ok := m[key].([]any)
	if !ok {
		return nil
	}
	return v
}

// getStringMap reads a map[string]any field from a frontmatter map;
// non-map or missing entries return nil.
func getStringMap(m map[string]any, key string) map[string]any {
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

// getInt64 reads an int64 field. TOML decodes integers as int64
// directly; JSON-via-structpb round-trips them through float64. Treat
// both as authoritative; everything else returns 0.
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

// parseRFC3339 accepts an empty string (returns zero, no error —
// "absent") or an RFC3339 string.
func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a valid RFC3339 timestamp: %w", err)
	}
	return t.UTC(), nil
}

// deepCopyAny recurses into maps and slices so loaded Bindings are
// independent of the source frontmatter map.
func deepCopyAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCopyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyAny(vv)
		}
		return out
	default:
		return v
	}
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyAny(v)
	}
	return out
}

// Compile-time check: *FrontmatterBindingStore satisfies BindingStore.
var _ BindingStore = (*FrontmatterBindingStore)(nil)
