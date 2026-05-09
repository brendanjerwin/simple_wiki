// Package google_keep owns the per-connector floor of the Google Keep
// bridge: the wire-protocol Gateway (sub-package gateway/), the
// Anti-Corruption Layer translator (sub-package translator/), and the
// BackendAdapter implementation in this file.
//
// Per ADR-0012 (audited 2026-05-04 refinement) the engine's lifecycle
// (per-tick reconcile, Bind/Unbind, ForceFullResync, Pause/Resume,
// precondition recovery, dead-letter retry, scheduler tick, debouncer,
// binding store) lives in internal/connectors/engine. The adapter here
// provides only the wire-protocol verbs, translation, capability bits,
// and error classification — exactly the BackendAdapter contract in
// internal/connectors/adapter.go.
//
// Concurrency: the Engine treats each adapter primitive as safe for
// concurrent use (the scheduler dispatches without per-connector
// serialization). The KeepAdapter holds no mutable state beyond the
// injected collaborators; it is safe for concurrent use across
// scheduler ticks, debouncer fires, and gRPC handler calls.
//
//revive:disable:var-naming // package name google_keep mirrors ConnectorKindGoogleKeep
package google_keep

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/translator"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// AdapterStateKeyItemMapping is the AdapterState subtree key under which
// the adapter persists Keep's per-item bookkeeping (wiki-uid →
// {server_id, base_version, client_id}). The engine treats this as
// opaque; the adapter reads/writes its structured shape via
// EncodeAdapterState / DecodeAdapterState.
const AdapterStateKeyItemMapping = "item_mapping"

// AdapterStateKeyItemIDMap is the engine's wiki-uid → server-id map
// (the rebuild-vs-bind contract). Mirrors the engine's
// adapterStateItemIDMapKey but kept locally so the Keep adapter's
// SeedBindingState/RebuildAdapterState can pre-populate the field
// without a circular import.
const AdapterStateKeyItemIDMap = "item_id_map"

// AdapterStateKeyKeepCursor is the AdapterState subtree key under which
// the adapter persists Keep's `to_version` cursor token. Sent as
// `target_version` on the next pull so Keep returns only the delta
// since the last sync. Empty / missing = full pull.
const AdapterStateKeyKeepCursor = "keep_cursor"

// AdapterStateKeyLabelIDs is the AdapterState subtree key under which
// the adapter persists the per-binding label-name → Keep MainID map.
// Used by translator.MergeKeepLabels as the primary lookup so
// incremental pulls (which usually return no labels) don't cause the
// adapter to emit fresh label CRUD entries every tick for labels Keep
// already knows about.
const AdapterStateKeyLabelIDs = "label_ids"

// AdapterStateKeyKeepNoteClientID is the AdapterState subtree key under
// which the adapter persists the LIST node's client-generated `id`
// (distinct from the server-assigned `serverId` aka RemoteHandle).
// Outbound LIST node updates MUST send `id != serverId`; Keep returns
// stage3 HTTP 500 "Unknown Error" on `id == serverId`.
const AdapterStateKeyKeepNoteClientID = "keep_note_client_id"

// ItemMappingFields are the per-item AdapterState bookkeeping fields
// the adapter persists under AdapterStateKeyItemMapping[uid]. Mirrors
// the legacy package's ItemMapping shape (minus the engine-owned
// fingerprint/dead-letter fields, which the engine now owns at the
// Binding level).
const (
	itemMappingFieldServerID    = "server_id"
	itemMappingFieldBaseVersion = "base_version"
	itemMappingFieldClientID    = "client_id"
)

// MasterTokenBundle is the gpsoauth credential trio the KeepAdapter
// hands to the gateway: a long-lived master token, the user's email
// (Stage 2 needs both), and the persisted Android device id (so
// gpsoauth doesn't trip "new device" heuristics on a stable profile).
type MasterTokenBundle struct {
	MasterToken string
	Email       string
	AndroidID   string
}

// CredentialReader is the per-profile auth-state seam the KeepAdapter
// uses to obtain the master_token + email pair for building KeepClient
// instances. The production wiring satisfies this with the
// frontmatter-backed credential store; tests inject a programmable
// fake.
//
// Per ADR-0014, master tokens live plaintext on the operator-trusted
// profile page. The reader returns ErrCredentialMissing when the
// profile has no master_token (typically: never connected, or
// disconnected). The engine maps that to ErrorClassAuthFailed via the
// adapter's ClassifyError, transitioning the binding to paused.
type CredentialReader interface {
	// LoadMasterToken returns the master token + email + android device
	// id persisted on the given profile page as a MasterTokenBundle.
	// Errors are wrapped; callers branch on ErrCredentialMissing for
	// the "not configured" case.
	LoadMasterToken(ctx context.Context, profileID wikipage.PageIdentifier) (MasterTokenBundle, error)
}

// ErrCredentialMissing is returned by CredentialReader.LoadMasterToken
// when the profile has no master token. The adapter's ClassifyError
// maps this to ErrorClassAuthFailed so the engine transitions the
// binding to paused without bubbling a connector-internal sentinel
// through the dispatch shape.
var ErrCredentialMissing = errors.New("google_keep: profile has no master token (Disconnect or never connected)")

// ErrKeepNoteNotAList is returned by ValidateRemoteBinding when the
// chosen Keep note exists but is not a LIST node (free-form note,
// blob, etc.). Mirrors the legacy package's intent; the gRPC boundary
// maps this to FailedPrecondition.
var ErrKeepNoteNotAList = errors.New("google_keep: bound note is not a checklist (LIST node) — pick a Keep checklist note")

// AuthExchanger is the subset of gateway.Authenticator the adapter
// uses. Stated as an interface so adapter_test.go can substitute a
// fake without spinning up a real httptest server.
type AuthExchanger interface {
	ExchangeMasterTokenForBearer(ctx context.Context, email, masterToken string) (string, error)
}

// KeepClientFactory constructs a *gateway.KeepClient bound to the given
// profile's bearer. The bootstrap-supplied factory authenticates Stage
// 2 (master token → bearer) and constructs the typed client; tests
// inject a stub that returns a fake KeepClient.
type KeepClientFactory func(ctx context.Context, profileID wikipage.PageIdentifier, masterToken, email string) (KeepClient, error)

// KeepClient is the subset of *gateway.KeepClient the adapter calls.
// Stated as an interface so adapter_test.go can substitute a fake
// without spinning up a real httptest.Server.
type KeepClient interface {
	Changes(ctx context.Context, req gateway.ChangesRequest) (gateway.ChangesResponse, error)
	CreateList(ctx context.Context, title string) (string, error)
	CreateListWithItems(ctx context.Context, title string, items []gateway.ListItemSpec) (gateway.CreateListResult, error)
}

// Logger is the minimal log surface the adapter needs.
type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// Clock is the testable wall-clock seam.
type Clock interface {
	Now() time.Time
}

// SystemClock returns time.Now(). The production wiring uses this;
// tests inject a deterministic stub.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// KeepAdapter implements connectors.BackendAdapter against the Google
// Keep gateway + translator. Construction wires the per-profile
// credential reader and the client factory; the adapter holds no
// per-binding state (the engine carries everything in the Binding's
// AdapterState subtree).
type KeepAdapter struct {
	credentials   CredentialReader
	clientFactory KeepClientFactory
	clock         Clock
	logger        Logger
}

// NewKeepAdapter wires a KeepAdapter. Every dependency is required.
func NewKeepAdapter(credentials CredentialReader, clientFactory KeepClientFactory, clock Clock, logger Logger) (*KeepAdapter, error) {
	if credentials == nil {
		return nil, errors.New("google_keep: credentials must not be nil")
	}
	if clientFactory == nil {
		return nil, errors.New("google_keep: clientFactory must not be nil")
	}
	if clock == nil {
		return nil, errors.New("google_keep: clock must not be nil")
	}
	if logger == nil {
		return nil, errors.New("google_keep: logger must not be nil")
	}
	return &KeepAdapter{
		credentials:   credentials,
		clientFactory: clientFactory,
		clock:         clock,
		logger:        logger,
	}, nil
}

// Compile-time check: *KeepAdapter satisfies the BackendAdapter
// contract. Adding a method to BackendAdapter is now a compile error
// for this package — the entire point of the abstraction.
var _ connectors.BackendAdapter = (*KeepAdapter)(nil)

// Kind reports the connector kind. Used by structured logs, metrics,
// and op-log self-source markers.
func (*KeepAdapter) Kind() connectors.ConnectorKind {
	return connectors.ConnectorKindGoogleKeep
}

// SupportsSubtasks reports that Keep's LIST node is flat — there is no
// parent-child item hierarchy. Returns false.
func (*KeepAdapter) SupportsSubtasks() bool {
	return false
}

// PullRemote fetches every Keep-side LIST_ITEM under the bound list
// node that changed since the last successful sync. NewCursor is the
// Keep `to_version` token (string) so AdvanceCursor can store it
// verbatim. Truncated propagates from the gateway response.
//
// Function-length suppression: this primitive owns Keep's pull
// contract — the per-node filter (type / parent / typeless-tombstone),
// the keep_note_client_id self-heal, the truncation handling, and the
// RemoteDiverged comparison. Each of those is a small block but the
// whole flow is one operational unit; splitting it fragments the
// contract.
//
//revive:disable-next-line:function-length
func (a *KeepAdapter) PullRemote(ctx context.Context, binding connectors.Binding) (connectors.RemotePullResult, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return connectors.RemotePullResult{}, err
	}
	cursor := readKeepCursor(binding.AdapterState)
	now := a.clock.Now().UTC()
	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		TargetVersion:   cursor,
		SessionID:       fmt.Sprintf("s--%d--pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return connectors.RemotePullResult{}, err
	}

	knownRefs := readItemIDMapServerIDSet(binding.AdapterState)
	items := make([]connectors.RemoteItem, 0)
	for _, n := range resp.Nodes {
		// Production fix 2026-05-08: a tombstone for a known item must
		// be accepted regardless of node Type and parent linkage. Keep
		// strips both fields when a user deletes the item via the app
		// — the only stable identity left is ServerID, and we recognize
		// it as ours via item_id_map. This check has to run BEFORE the
		// type filter; the legacy ordering (type filter first) dropped
		// typeless tombstones silently. (User-reported 2026-05-08:
		// "deletes from keep side didn't sync"; logs showed barebones
		// tombstones with no Type, no parent, just id+serverId+deleted
		// timestamp.)
		isTombstone := !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero()
		_, knownRef := knownRefs[n.ServerID]
		isKnownTombstone := isTombstone && knownRef

		if !isKnownTombstone {
			// Non-tombstone (or tombstone we don't know about): apply
			// the standard structural filters. Items must be of type
			// LIST_ITEM and parented to our LIST node.
			if n.Type != gateway.NodeTypeListItem {
				continue
			}
			inOurList := n.ParentID == binding.RemoteHandle || n.ParentServerID == binding.RemoteHandle
			if !inOurList {
				continue
			}
		}
		item := listItemNodeToRemoteItem(n)
		// ADR-0015 Fix #1: populate RemoteDiverged by comparing the
		// incoming node's BaseVersion against the stored BaseVersion in
		// AdapterState. Keep's cursor is server-issued (not safety-
		// buffered), so items only appear when they genuinely changed —
		// but we still compare to maintain correctness if the cursor
		// rewinds (e.g., after a ForceFullResync).
		// An absent stored BaseVersion means we have no baseline → not
		// diverged (the first inbound apply should proceed normally).
		if stored := readItemMapping(binding.AdapterState, n.ServerID).BaseVersion; stored != "" && stored != n.BaseVersion {
			item.RemoteDiverged = true
		}
		items = append(items, item)
	}

	// keep_note_client_id self-heal (legacy keepsync had this in
	// connector.go's SyncToKeep; lost in the Phase 5-A port). Outbound
	// pushes that include the LIST node (notably label CRUD) require
	// the LIST node to carry id != serverId per Keep's stage3
	// invariant. When that field is empty, the push 500s. Two-stage
	// recovery:
	//
	//   1. If the LIST node is in the current pull, capture its
	//      client id directly. AdapterState is a map, so the mutation
	//      propagates back to the engine's binding via reference.
	//
	//   2. If the LIST node is NOT in this incremental pull (LIST
	//      didn't change since last cursor), signal Truncated=true so
	//      the engine triggers a ForceFullResync — full pulls always
	//      include the LIST node, so the next reconcile after the
	//      resync will land in step 1.
	truncated := resp.Truncated
	// Self-heal only applies when remote_handle is set. Bindings with
	// empty remote_handle are fundamentally broken (legacy migration
	// gap on `keep_note_id` → `remote_handle`) and re-binding is the
	// only recovery — triggering ForceFullResync hammers the API
	// without making progress (perpetual loop observed in production
	// 2026-05-07 on a binding migrated without the alias translation).
	if binding.RemoteHandle != "" && needsClientIDSelfHeal(binding) {
		if captured := translator.FindListClientID(resp.Nodes, binding.RemoteHandle); captured != "" {
			if binding.AdapterState == nil {
				binding.AdapterState = connectors.AdapterState{}
			}
			binding.AdapterState[AdapterStateKeyKeepNoteClientID] = captured
		} else {
			truncated = true
		}
	}

	return connectors.RemotePullResult{
		Items:     items,
		NewCursor: resp.ToVersion,
		Truncated: truncated,
	}, nil
}

// readItemIDMapServerIDSet returns the set of ServerIDs (values) in
// the engine's item_id_map subtree. PullRemote uses this to identify
// known-by-us items for tombstone-with-cleared-parent recovery.
func readItemIDMapServerIDSet(state connectors.AdapterState) map[string]struct{} {
	out := map[string]struct{}{}
	if state == nil {
		return out
	}
	raw, ok := state[AdapterStateKeyItemIDMap]
	if !ok {
		return out
	}
	switch m := raw.(type) {
	case map[string]string:
		for _, v := range m {
			if v != "" {
				out[v] = struct{}{}
			}
		}
	case map[string]any:
		for _, v := range m {
			if s, isStr := v.(string); isStr && s != "" {
				out[s] = struct{}{}
			}
		}
	default:
		// Unknown shape — silently skip. Matches the engine's
		// readItemIDMap fallback semantics.
	}
	return out
}

// needsClientIDSelfHeal reports whether the binding's stored
// keep_note_client_id is missing or empty. See PullRemote's self-heal
// block for the production scenario.
func needsClientIDSelfHeal(binding connectors.Binding) bool {
	if binding.AdapterState == nil {
		return true
	}
	v, ok := binding.AdapterState[AdapterStateKeyKeepNoteClientID].(string)
	return !ok || v == ""
}

// InsertRemote pushes a fresh wiki item to the bound Keep list as a
// new LIST_ITEM child. Keep's wire shape requires per-item client_ids
// and parent linkage; the gateway's Changes endpoint does the actual
// write. Returns the new ServerID Keep echoed back as the RemoteRef.
func (a *KeepAdapter) InsertRemote(ctx context.Context, binding connectors.Binding, item connectors.WikiItem) (connectors.RemoteRef, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return "", err
	}
	now := a.clock.Now().UTC()
	clientItemID, err := buildKeepItemID(now, item.UID)
	if err != nil {
		return "", fmt.Errorf("build client id for %s: %w", item.UID, err)
	}
	cl := wikiItemToProto(item)
	node := translator.WikiToKeep(cl, binding.RemoteHandle, "")
	node.ID = clientItemID
	node.Timestamps.Created = now
	node.Timestamps.Updated = now

	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		Nodes:           []gateway.Node{node},
		TargetVersion:   readKeepCursor(binding.AdapterState),
		SessionID:       fmt.Sprintf("s--%d--insert", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return "", err
	}
	for _, n := range resp.Nodes {
		if n.Type == gateway.NodeTypeListItem && n.ID == clientItemID {
			if n.ServerID == "" {
				return "", fmt.Errorf("%w: insert echoed node without ServerID", gateway.ErrProtocolDrift)
			}
			return connectors.RemoteRef(n.ServerID), nil
		}
	}
	return "", fmt.Errorf("%w: insert response did not echo our client id %q", gateway.ErrProtocolDrift, clientItemID)
}

// PatchRemote pushes an update to an existing Keep LIST_ITEM. Keep's
// optimistic-concurrency model requires `baseVersion` and the
// client-side `id` from the prior pull; the engine routes a
// stage3-500 "Unknown Error" failure (mapped via ClassifyError to
// ErrorClassPreconditionFailed) through the 3-branch precondition
// recovery (MATRIX row 6).
func (a *KeepAdapter) PatchRemote(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef, item connectors.WikiItem) (connectors.RemoteRef, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return "", err
	}
	mapping := readItemMapping(binding.AdapterState, string(ref))
	cl := wikiItemToProto(item)
	node := translator.WikiToKeep(cl, binding.RemoteHandle, mapping.ClientID)
	node.ServerID = string(ref)
	node.BaseVersion = mapping.BaseVersion

	now := a.clock.Now().UTC()
	node.Timestamps.Updated = now

	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		Nodes:           []gateway.Node{node},
		TargetVersion:   readKeepCursor(binding.AdapterState),
		SessionID:       fmt.Sprintf("s--%d--patch", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return "", err
	}
	if writeFailed(resp.WriteResults, node.ID) {
		return "", fmt.Errorf("%w: keep rejected patch for serverID=%s", gateway.ErrProtocolDrift, ref)
	}
	return ref, nil
}

// DeleteRemote removes a Keep LIST_ITEM by setting the soft-delete
// flag (`Deleted` timestamp). Keep's gkeepapi reference exposes both
// trash() and delete() as separate operations, but only `deleted`
// actually propagates through Keep's Changes API on incremental
// updates — `trashed` alone causes Keep to apply other fields (e.g.
// the omitempty-forced `checked: false`) WITHOUT removing the item.
// Production observation 2026-05-07: items the user deleted in the
// wiki appeared as unchecked-but-present in Keep when the adapter
// only set Trashed.
//
// Idempotent: repeated deletes are no-ops.
func (a *KeepAdapter) DeleteRemote(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef) error {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return err
	}
	mapping := readItemMapping(binding.AdapterState, string(ref))
	now := a.clock.Now().UTC()
	node := gateway.Node{
		Kind:           "notes#node",
		ID:             mapping.ClientID,
		ServerID:       string(ref),
		Type:           gateway.NodeTypeListItem,
		ParentID:       binding.RemoteHandle,
		ParentServerID: binding.RemoteHandle,
		BaseVersion:    mapping.BaseVersion,
		Timestamps: gateway.Timestamps{
			Updated: now,
			Deleted: now,
		},
	}
	if node.ID == "" {
		// Brand-new tombstone for a never-pushed-back item: synthesize
		// a client id so the wire shape is well-formed. The node is
		// recognized as a tombstone via the Deleted timestamp set
		// above (NOT Trashed — see the docstring for why; production
		// regression 2026-05-07).
		idb, idErr := buildKeepItemID(now, string(ref))
		if idErr != nil {
			return fmt.Errorf("build client id for delete: %w", idErr)
		}
		node.ID = idb
	}

	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		Nodes:           []gateway.Node{node},
		TargetVersion:   readKeepCursor(binding.AdapterState),
		SessionID:       fmt.Sprintf("s--%d--delete", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return err
	}
	if writeFailed(resp.WriteResults, node.ID) {
		return fmt.Errorf("%w: keep rejected delete for serverID=%s", gateway.ErrProtocolDrift, ref)
	}
	return nil
}

// RemoteToWiki converts a normalized RemoteItem to a WikiItem via the
// translator. The translator strips inline #tags out of Text into
// Tags, and splits the description suffix off the head line.
//
// The returned UID is empty — the engine resolves the uid via
// AdapterState's item_mapping (Keep's ServerIDs are stable, so the
// mapping is established at SeedBindingState / first PullRemote).
func (*KeepAdapter) RemoteToWiki(remote connectors.RemoteItem) (connectors.WikiItem, error) {
	node := remoteItemToListItemNode(remote)
	cl, err := translator.KeepToWiki(node)
	if err != nil {
		return connectors.WikiItem{}, fmt.Errorf("translate keep node %s: %w", remote.Ref, err)
	}
	desc := ""
	if cl.Description != nil {
		desc = *cl.Description
	}
	return connectors.WikiItem{
		Text:        cl.GetText(),
		Checked:     cl.GetChecked(),
		Tags:        cl.GetTags(),
		Description: desc,
		SortOrder:   cl.GetSortOrder(),
	}, nil
}

// WikiToRemote converts a WikiItem into the normalized RemoteItem
// shape. Outbound primitives (InsertRemote/PatchRemote) consume
// translator.WikiToKeep directly via wikiItemToProto, but the
// BackendAdapter contract requires this method.
func (*KeepAdapter) WikiToRemote(wiki connectors.WikiItem) (connectors.RemoteItem, error) {
	cl := wikiItemToProto(wiki)
	headLine := translator.EncodeTextWithTags(cl.GetText(), cl.GetTags())
	text := headLine
	if d := cl.GetDescription(); d != "" {
		text = headLine + translator.DescriptionSeparator + d
	}
	return connectors.RemoteItem{
		Title:    text,
		Notes:    "",
		Status:   "",
		Position: strconv.FormatInt(cl.GetSortOrder(), sortValueBase),
	}, nil
}

// AdvanceCursor stores the new Keep cursor token (`to_version`) into
// the binding's AdapterState. Per MATRIX.md row 16, Keep's cursor is
// server-issued and opaque — no safety buffer is needed (unlike
// Tasks's timestamp-based cursor).
// SyncCollectionState reconciles per-binding label state. Walks every
// wiki item's tags to compute the union of hashtags, mints Keep labels
// for any tag not yet mapped (via translator.MergeKeepLabels), and
// pushes label CRUD entries plus a LIST-node update with the merged
// labelIDs in a single Changes request. Restores the legacy keepsync
// SyncToKeep label sync block (lost in the Phase 5-A port — production
// regression 2026-05-07: hashtag changes in wiki items didn't propagate
// to Keep labels).
//
// Idempotent: when MergeKeepLabels returns no labelPush AND the LIST
// node's existing labelIDs already match the desired set, this is a
// no-op (no Changes request, no AdapterState mutation).
//
// Function-length suppression: the label CRUD sequence (compute
// merge → check no-op → build LIST node update → push Changes →
// merge response labels into AdapterState) is one atomic operational
// unit per the legacy SyncToKeep block.
//
//revive:disable-next-line:function-length
func (a *KeepAdapter) SyncCollectionState(ctx context.Context, binding connectors.Binding, items []connectors.WikiItem) (connectors.Binding, error) {
	tags := uniqueTagsFromItems(items)
	persisted := readLabelIDs(binding.AdapterState)

	now := a.clock.Now().UTC()
	labelPush, listLabelIDs, err := translator.MergeKeepLabels(tags, persisted, nil, now)
	if err != nil {
		return binding, fmt.Errorf("sync collection state: merge keep labels for profile %s: %w",
			string(binding.ProfileID), err)
	}

	// Decide whether the LIST node needs an update. Treat the persisted
	// set of MainIDs as the desired post-tick state on Keep; if every
	// listLabelID is already in the persisted map AND there are no new
	// labels to mint, skip the request.
	persistedSet := make(map[string]struct{}, len(persisted))
	for _, id := range persisted {
		persistedSet[id] = struct{}{}
	}
	allListIDsPersisted := true
	for _, id := range listLabelIDs {
		if _, ok := persistedSet[id]; !ok {
			allListIDsPersisted = false
			break
		}
	}
	if len(labelPush) == 0 && allListIDsPersisted && len(listLabelIDs) == labelCountInPersisted(persistedSet, listLabelIDs) {
		return binding, nil
	}

	client, clientErr := a.buildClientForProfile(ctx, binding.ProfileID)
	if clientErr != nil {
		return binding, clientErr
	}

	listClientID, _ := binding.AdapterState[AdapterStateKeyKeepNoteClientID].(string)
	if listClientID == "" {
		// Without the LIST node's client id we can't emit a valid
		// labelIDs assignment — Keep's stage3 invariant requires
		// id != serverId on outbound LIST nodes. Self-heal via
		// PullRemote handles this on the next tick; defer the label
		// push until then.
		return binding, nil
	}

	listNode := gateway.Node{
		Kind:           "notes#node",
		ID:             listClientID,
		ServerID:       binding.RemoteHandle,
		Type:           gateway.NodeTypeList,
		ParentID:       "",
		ParentServerID: "",
		Timestamps:     gateway.Timestamps{Updated: now},
		LabelIDs:       listLabelIDs,
	}

	resp, changesErr := client.Changes(ctx, gateway.ChangesRequest{
		Nodes:           []gateway.Node{listNode},
		Labels:          labelPush,
		TargetVersion:   readKeepCursor(binding.AdapterState),
		SessionID:       fmt.Sprintf("s--%d--labels", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if changesErr != nil {
		return binding, changesErr
	}

	updated := persisted
	if updated == nil {
		updated = map[string]string{}
	}
	// MergeKeepLabels emits in the same order as the input tags for any
	// not previously matched, so we persist using the canonical Name
	// directly.
	for _, l := range labelPush {
		updated[l.Name] = l.MainID
	}
	for _, l := range resp.Labels {
		if l.MainID != "" && l.Name != "" {
			updated[l.Name] = l.MainID
		}
	}

	out := persistLabelIDs(binding, updated)
	return out, nil
}

// uniqueTagsFromItems returns the deduplicated, order-preserving set
// of tags across all wiki items.
func uniqueTagsFromItems(items []connectors.WikiItem) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, it := range items {
		for _, tag := range it.Tags {
			if tag == "" {
				continue
			}
			lc := strings.ToLower(tag)
			if _, ok := seen[lc]; ok {
				continue
			}
			seen[lc] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}

// readLabelIDs pulls the persisted name → MainID map out of the
// binding's AdapterState[label_ids] subtree. Returns a non-nil map.
func readLabelIDs(state connectors.AdapterState) map[string]string {
	out := map[string]string{}
	if state == nil {
		return out
	}
	raw, ok := state[AdapterStateKeyLabelIDs]
	if !ok {
		return out
	}
	switch m := raw.(type) {
	case map[string]string:
		for k, v := range m {
			out[k] = v
		}
	case map[string]any:
		for k, v := range m {
			if s, isStr := v.(string); isStr {
				out[k] = s
			}
		}
	default:
		// Unknown shape — silently skip; matches readItemIDMap semantics.
	}
	return out
}

// persistLabelIDs writes the supplied name → MainID map back into the
// binding's AdapterState[label_ids] subtree.
func persistLabelIDs(binding connectors.Binding, labels map[string]string) connectors.Binding {
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	out := make(map[string]any, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	binding.AdapterState[AdapterStateKeyLabelIDs] = out
	return binding
}

// labelCountInPersisted reports how many of listLabelIDs are present
// in persistedSet. Used to detect that the LIST node's desired
// labelIDs assignment is already a no-op against persisted state.
func labelCountInPersisted(persistedSet map[string]struct{}, listLabelIDs []string) int {
	count := 0
	for _, id := range listLabelIDs {
		if _, ok := persistedSet[id]; ok {
			count++
		}
	}
	return count
}

// RefreshItemBaseline updates the stored item_mapping entry for ref's
// BaseVersion (and ClientID) from the freshly-read remote item's
// Vendor map. Used by the engine's precondition_recovery path: after
// ReadRemoteByRef returns, the stored BaseVersion is stale (that's
// why the patch hit stage3-500), and the recovery's re-PATCH would
// loop forever without this refresh. Production fix 2026-05-06.
func (*KeepAdapter) RefreshItemBaseline(binding connectors.Binding, remote connectors.RemoteItem) connectors.Binding {
	serverID := string(remote.Ref)
	if serverID == "" {
		return binding
	}
	baseVersion, _ := remote.Vendor["base_version"].(string)
	clientID, _ := remote.Vendor["client_id"].(string)
	if baseVersion == "" && clientID == "" {
		return binding
	}
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	mappingRaw, ok := binding.AdapterState[AdapterStateKeyItemMapping].(map[string]any)
	if !ok {
		mappingRaw = map[string]any{}
	}
	existing, _ := mappingRaw[serverID].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	existing[itemMappingFieldServerID] = serverID
	if baseVersion != "" {
		existing[itemMappingFieldBaseVersion] = baseVersion
	}
	if clientID != "" {
		existing[itemMappingFieldClientID] = clientID
	}
	mappingRaw[serverID] = existing
	binding.AdapterState[AdapterStateKeyItemMapping] = mappingRaw
	return binding
}

func (*KeepAdapter) AdvanceCursor(binding connectors.Binding, result connectors.RemotePullResult) connectors.Binding {
	cursor, ok := result.NewCursor.(string)
	if !ok || cursor == "" {
		return binding
	}
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	binding.AdapterState[AdapterStateKeyKeepCursor] = cursor
	return binding
}

// SeedBindingState clones the wiki list onto the existing Keep note
// (matched by remote handle) and records per-item ServerIDs. Mirrors
// the legacy connector's seedIDMapFromExistingList: pull the bound
// list's existing LIST_ITEMs, match wiki items by base text, record
// (ServerID, BaseVersion, ClientID) per-uid; capture the LIST node's
// ClientID; capture per-name → MainID map for labels.
//
// The engine calls this inside the bind mutex AFTER
// ValidateRemoteBinding passes. Errors at this layer abort the bind.
//
// On a fresh (empty) Keep note the returned AdapterState carries
// empty maps so EncodeAdapterState / DecodeAdapterState can round-trip
// uniformly.
//
// Function-length suppression: the seed pass owns the bind-time
// alignment contract (pull → build itemMapping + textIndex → text-
// match wikiItems into item_id_map → index labels → capture LIST
// client_id). The sequence is one atomic operational unit and
// matches MATRIX row 2.
//
//revive:disable-next-line:function-length
func (a *KeepAdapter) SeedBindingState(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string, wikiItems []connectors.WikiItem) (connectors.AdapterState, error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	now := a.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--bindseed-%s", now.UnixMilli(), remoteHandle),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return nil, fmt.Errorf("seed bind: pull keep state for %s: %w", remoteHandle, err)
	}
	itemMapping := map[string]any{}
	// Build a per-server-id mapping from the live LIST_ITEMs under
	// our list AND a text → server-id index for the bind-time
	// alignment pass below.
	textToServerID := map[string]string{}
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != remoteHandle && n.ParentServerID != remoteHandle {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		if n.ServerID == "" {
			continue
		}
		itemMapping[n.ServerID] = encodeItemMappingFields(n.ServerID, n.BaseVersion, n.ID)
		// First-write-wins on duplicate text in the pulled set
		// (rare; the operator may already have duplicates from
		// pre-fix sync bugs). Subsequent same-text Keep items remain
		// unmapped here; the engine's applyInbound dedup-by-text is
		// the safety net for any unmapped items the next reconcile
		// processes.
		if n.Text != "" {
			if _, taken := textToServerID[n.Text]; !taken {
				textToServerID[n.Text] = n.ServerID
			}
		}
	}

	// Bind-time alignment (architectural fix 2026-05-08): Keep has
	// no native wiki-uid marker (unlike Tasks's `wiki:<uid>` Notes),
	// so item_id_map cannot be derived from the pull alone. Match
	// wiki items against the pulled set by text (the same canonical
	// form WikiToRemote produces), pre-populating item_id_map at
	// the seed step so the first reconcile after Bind doesn't
	// duplicate. Without this, the first applyInbound saw every
	// Keep item as "unknown" (RemoteToWiki returns wikiItem.UID="")
	// and either took AddItemForSync (creating wiki duplicates) or
	// — with the dedup pass also added today — at least did the
	// matching there. Doing it at seed is earlier and authoritative.
	itemIDMap := map[string]any{}
	for _, w := range wikiItems {
		if w.UID == "" {
			continue
		}
		// Reuse the adapter's WikiToRemote canonicalization so the
		// comparison key is exactly what the pulled Keep item's
		// Text contains.
		remoteForm, txErr := a.WikiToRemote(w)
		if txErr != nil {
			continue
		}
		if ref, ok := textToServerID[remoteForm.Title]; ok {
			itemIDMap[w.UID] = ref
			delete(textToServerID, remoteForm.Title)
		}
	}

	labelIDs := map[string]any{}
	for k, v := range translator.IndexLabelsByName(pull.Labels) {
		labelIDs[k] = v
	}
	keepNoteClientID := translator.FindListClientID(pull.Nodes, remoteHandle)
	return connectors.AdapterState{
		AdapterStateKeyItemMapping:      itemMapping,
		AdapterStateKeyKeepCursor:       pull.ToVersion,
		AdapterStateKeyLabelIDs:         labelIDs,
		AdapterStateKeyKeepNoteClientID: keepNoteClientID,
		AdapterStateKeyItemIDMap:        itemIDMap,
	}, nil
}

// ValidateRemoteBinding checks per-adapter pre-conditions before the
// engine writes a new binding. For Keep, validates that the chosen
// note exists and is a LIST node (free-form notes / blobs are
// rejected via ErrKeepNoteNotAList).
//
// Empty remoteHandle signals "create a new Keep note" — the bind
// flow's adapter-side helper (CreateRemoteCollection) creates the
// note BEFORE engine.Bind, so the engine boundary always sees a
// non-empty handle here. Mirror Tasks's empty-handle rejection so
// the contract is uniform.
func (a *KeepAdapter) ValidateRemoteBinding(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) error {
	if remoteHandle == "" {
		return errors.New("google_keep: remote_handle must not be empty (create-new-list path runs before engine.Bind)")
	}
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return err
	}
	now := a.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--validate-%s", now.UnixMilli(), remoteHandle),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return fmt.Errorf("validate: pull keep state for %s: %w", remoteHandle, err)
	}
	for _, n := range pull.Nodes {
		if n.ServerID != remoteHandle {
			continue
		}
		if n.Type == gateway.NodeTypeList {
			if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
				return fmt.Errorf("%w: bound note %s is trashed or deleted", gateway.ErrBoundNoteDeleted, remoteHandle)
			}
			return nil
		}
		return ErrKeepNoteNotAList
	}
	return fmt.Errorf("%w: note %s not found in user's account", gateway.ErrBoundNoteDeleted, remoteHandle)
}

// RebuildAdapterState pulls the full remote state and rebuilds
// AdapterState from scratch. Resets the Keep cursor so the next
// reconcile re-processes the world.
//
// Preserves the existing item_id_map (wiki-uid → server-id mapping)
// for entries whose refs still appear in the rebuilt item_mapping.
// Without this, every rebuild wiped uid → ref mappings and the next
// reconcile re-Inserted every wiki item — creating duplicates at
// Keep (production regression 2026-05-08, "shopping list is
// doubled"). Entries pointing to refs that no longer exist remote-
// side are dropped (the item was deleted/trashed).
func (a *KeepAdapter) RebuildAdapterState(ctx context.Context, binding connectors.Binding) (connectors.AdapterState, error) {
	// Rebuild does not have ambient access to the wiki checklist's
	// items; pass nil. The engine's applyInbound dedup-by-text and
	// the preserveItemIDMap pass below handle the alignment role
	// that wikiItems would otherwise play at bind time.
	state, err := a.SeedBindingState(ctx, binding.ProfileID, binding.RemoteHandle, nil)
	if err != nil {
		return nil, fmt.Errorf("rebuild: %w", err)
	}
	// Reset the cursor so the next reconcile re-processes the world.
	// SeedBindingState returns the pull's to_version, which is fine
	// for a fresh bind but wrong for a force-resync — the engine
	// expects that rebuild restarts the cursor.
	state[AdapterStateKeyKeepCursor] = ""

	// Preserve item_id_map entries whose refs are still present in
	// the rebuilt item_mapping. Drop entries pointing to refs that
	// no longer exist on the remote.
	state[AdapterStateKeyItemIDMap] = preserveItemIDMap(binding.AdapterState, state)

	return state, nil
}

// preserveItemIDMap walks the binding's existing item_id_map and
// returns a new map containing only the entries whose ref is present
// in the rebuilt state's item_mapping (i.e., the remote item still
// exists). Used by RebuildAdapterState to avoid the legacy data-loss
// behavior that produced duplicate Inserts after rebuild.
func preserveItemIDMap(prevState, rebuiltState connectors.AdapterState) map[string]any {
	out := map[string]any{}
	if prevState == nil || rebuiltState == nil {
		return out
	}
	mapping, ok := rebuiltState[AdapterStateKeyItemMapping].(map[string]any)
	if !ok {
		return out
	}
	rawIDMap, ok := prevState[AdapterStateKeyItemIDMap]
	if !ok {
		return out
	}
	switch m := rawIDMap.(type) {
	case map[string]string:
		for uid, ref := range m {
			if _, exists := mapping[ref]; exists {
				out[uid] = ref
			}
		}
	case map[string]any:
		for uid, refRaw := range m {
			ref, isStr := refRaw.(string)
			if !isStr {
				continue
			}
			if _, exists := mapping[ref]; exists {
				out[uid] = ref
			}
		}
	default:
		// Unknown shape — silently skip; matches readItemIDMap semantics.
	}
	return out
}

// FetchRemoteListTitle returns the friendly title of the bound LIST
// node. Returns ("", false, nil) on transient failure or list-not-
// found; the engine preserves the prior title in that case.
func (a *KeepAdapter) FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error) {
	if remoteHandle == "" {
		return "", false, nil
	}
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		// Auth issues here are best-effort title sync; the engine
		// continues without updating the title.
		return "", false, nil
	}
	now := a.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--title-%s", now.UnixMilli(), remoteHandle),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return "", false, nil
	}
	for _, n := range pull.Nodes {
		if n.ServerID == remoteHandle && n.Type == gateway.NodeTypeList {
			title := n.Title
			if title == "" {
				title = n.Text
			}
			return title, true, nil
		}
	}
	return "", false, nil
}

// ListRemoteCollections enumerates the calling user's LIST-typed Keep
// notes for the bind UI picker. Capabilities.HasSubtasks is always
// false (Keep has no parent-child item hierarchy).
func (a *KeepAdapter) ListRemoteCollections(ctx context.Context, profileID wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	now := a.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--listnotes", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return nil, err
	}
	out := make([]connectors.RemoteCollection, 0)
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeList {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		title := n.Title
		if title == "" {
			title = n.Text
		}
		out = append(out, connectors.RemoteCollection{
			Handle:       n.ServerID,
			Title:        title,
			Capabilities: connectors.CollectionCapabilities{HasSubtasks: false},
		})
	}
	return out, nil
}

// EncodeAdapterState validates the AdapterState shape and returns the
// underlying map for persistence. Empty/missing keys are normalized to
// zero values so the persisted shape is uniform across bindings.
func (*KeepAdapter) EncodeAdapterState(state connectors.AdapterState) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range state {
		out[k] = v
	}
	if _, ok := out[AdapterStateKeyItemMapping]; !ok {
		out[AdapterStateKeyItemMapping] = map[string]any{}
	}
	if _, ok := out[AdapterStateKeyKeepCursor]; !ok {
		out[AdapterStateKeyKeepCursor] = ""
	}
	if _, ok := out[AdapterStateKeyLabelIDs]; !ok {
		out[AdapterStateKeyLabelIDs] = map[string]any{}
	}
	if _, ok := out[AdapterStateKeyKeepNoteClientID]; !ok {
		out[AdapterStateKeyKeepNoteClientID] = ""
	}
	return out, nil
}

// DecodeAdapterState parses a persisted AdapterState (post-TOML round-
// trip) into the engine's opaque envelope. Type coercion handles
// TOML's reduced type vocabulary.
func (*KeepAdapter) DecodeAdapterState(raw map[string]any) (connectors.AdapterState, error) {
	if raw == nil {
		return connectors.AdapterState{}, nil
	}
	out := connectors.AdapterState{}
	for k, v := range raw {
		out[k] = v
	}
	return out, nil
}

// ReadRemoteByRef pulls a single LIST_ITEM by its ServerID. Used by
// the engine's precondition_recovery path. Keep's REST surface
// doesn't expose a per-node GET; the gateway's Changes endpoint is
// the only read primitive, so this method walks the full pull
// response and filters by serverId.
//
// On a 404 (the note containing the item is gone), returns
// (RemoteItem{Deleted: true}, nil) per the BackendAdapter contract.
// On a missing-but-no-error response, also returns Deleted=true.
func (a *KeepAdapter) ReadRemoteByRef(ctx context.Context, binding connectors.Binding, ref connectors.RemoteRef) (connectors.RemoteItem, error) {
	client, err := a.buildClientForProfile(ctx, binding.ProfileID)
	if err != nil {
		return connectors.RemoteItem{}, err
	}
	now := a.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--readref", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		if errors.Is(err, gateway.ErrBoundNoteDeleted) {
			return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
		}
		return connectors.RemoteItem{}, err
	}
	for _, n := range pull.Nodes {
		if n.ServerID != string(ref) {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
		}
		if n.Type != gateway.NodeTypeListItem {
			return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
		}
		return listItemNodeToRemoteItem(n), nil
	}
	return connectors.RemoteItem{Deleted: true, Ref: ref}, nil
}

// ClassifyError maps Keep gateway errors to the engine's ErrorClass
// vocabulary. Branches on errors.Is sentinels — never on string
// contents.
//
// MATRIX rows 5/6/7: AuthFailed pauses the binding; PreconditionFailed
// runs the 3-branch recovery (Keep returns stage3 HTTP 500
// "Unknown Error" on a stale baseVersion — surfaced via
// ErrProtocolDrift wrap, mapped here to PreconditionFailed for the
// recovery path); RateLimited backs off the tick; NotFound mirrors
// the deletion to the wiki; Retryable bumps PushFailureCount; the
// default is ErrorClassRetryable so transient or unclassified errors
// trigger the engine's exponential-backoff dead-letter path (per the
// strictest-behavior-wins audit).
func (*KeepAdapter) ClassifyError(err error) connectors.ErrorClass {
	if err == nil {
		return connectors.ErrorClassNone
	}
	switch {
	case errors.Is(err, ErrCredentialMissing),
		errors.Is(err, gateway.ErrInvalidCredentials),
		errors.Is(err, gateway.ErrAuthRevoked):
		return connectors.ErrorClassAuthFailed
	case errors.Is(err, gateway.ErrRateLimited):
		return connectors.ErrorClassRateLimited
	case errors.Is(err, gateway.ErrBoundNoteDeleted):
		return connectors.ErrorClassNotFound
	case errors.Is(err, gateway.ErrServiceDisabled),
		errors.Is(err, gateway.ErrPermissionDenied):
		return connectors.ErrorClassFatal
	case errors.Is(err, gateway.ErrProtocolDrift):
		// Stage3 HTTP 500 "Unknown Error" on bad baseVersion surfaces
		// here — the gateway wraps unmapped 5xxes through
		// ErrProtocolDrift. The engine's precondition_recovery path
		// is the right home for "stale concurrency token" recovery
		// (MATRIX row 6); routing protocol drift through there is
		// the strictest-behavior-wins choice.
		return connectors.ErrorClassPreconditionFailed
	default:
		return connectors.ErrorClassRetryable
	}
}

// CreateRemoteCollection creates a fresh Keep LIST node titled after
// listName (and optionally seeded with initialItems). Returns the
// LIST node's ServerID (= the binding's RemoteHandle) and its title.
//
// The "Bind to a new Keep note" gRPC path calls this BEFORE
// engine.Bind: the bind ceremony itself takes a non-empty
// remote_handle (the engine doesn't manage remote-list creation).
// Mirrors the legacy Connector.Bind empty-keepNoteID branch's
// CreateListWithItems flow.
func (a *KeepAdapter) CreateRemoteCollection(ctx context.Context, profileID wikipage.PageIdentifier, listName string, initialItems []connectors.WikiItem) (handle, title string, err error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return "", "", fmt.Errorf("build client for profile %s: %w", profileID, err)
	}
	specs := make([]gateway.ListItemSpec, 0, len(initialItems))
	for i, it := range initialItems {
		// Encode tags inline + description suffix so the seeded
		// items round-trip through pull byte-identically.
		cl := wikiItemToProto(it)
		head := translator.EncodeTextWithTags(cl.GetText(), cl.GetTags())
		text := head
		if d := cl.GetDescription(); d != "" {
			text = head + translator.DescriptionSeparator + d
		}
		// (n - i) * 1000 keeps natural top-to-bottom order in the
		// Keep app (lower SortValue sorts to the bottom).
		sortValue := strconv.Itoa((len(initialItems) - i) * sortValueGap)
		specs = append(specs, gateway.ListItemSpec{
			Text:      text,
			Checked:   it.Checked,
			SortValue: sortValue,
		})
	}
	created, err := client.CreateListWithItems(ctx, listName, specs)
	if err != nil {
		return "", "", fmt.Errorf("create remote keep note %q for profile %s: %w", listName, profileID, err)
	}
	return created.ListServerID, listName, nil
}

// sortValueGap is the spacing we leave between adjacent initial
// items' Keep sort values so future inserts can land between them
// without a global re-numbering. 1000 is gkeepapi's gap of choice.
const sortValueGap = 1000

// sortValueBase is the base used when formatting / parsing sort
// values on the Keep wire. Mirrors translator.parseSortValue's
// expectations.
const sortValueBase = 10

// --- internal helpers -----------------------------------------------------

// buildClientForProfile loads the profile's master token + email and
// constructs a KeepClient via the injected factory.
func (a *KeepAdapter) buildClientForProfile(ctx context.Context, profileID wikipage.PageIdentifier) (KeepClient, error) {
	bundle, err := a.credentials.LoadMasterToken(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if bundle.MasterToken == "" {
		return nil, ErrCredentialMissing
	}
	return a.clientFactory(ctx, profileID, bundle.MasterToken, bundle.Email)
}

// readKeepCursor pulls AdapterState[keep_cursor]. Empty / missing
// returns "" (full pull on next call).
func readKeepCursor(state connectors.AdapterState) string {
	if state == nil {
		return ""
	}
	v, ok := state[AdapterStateKeyKeepCursor].(string)
	if !ok {
		return ""
	}
	return v
}

// readItemMappingEntry returns the per-item mapping fields for the
// given server id. Returns a zero-valued struct when missing.
type itemMapping struct {
	ServerID    string
	BaseVersion string
	ClientID    string
}

// readItemMapping pulls AdapterState[item_mapping][serverID] (the
// per-item record). Returns the zero struct if any segment is
// missing or wrong-typed.
func readItemMapping(state connectors.AdapterState, serverID string) itemMapping {
	if state == nil {
		return itemMapping{}
	}
	raw, ok := state[AdapterStateKeyItemMapping].(map[string]any)
	if !ok {
		return itemMapping{}
	}
	entry, ok := raw[serverID].(map[string]any)
	if !ok {
		return itemMapping{}
	}
	out := itemMapping{}
	if v, ok := entry[itemMappingFieldServerID].(string); ok {
		out.ServerID = v
	}
	if v, ok := entry[itemMappingFieldBaseVersion].(string); ok {
		out.BaseVersion = v
	}
	if v, ok := entry[itemMappingFieldClientID].(string); ok {
		out.ClientID = v
	}
	return out
}

// encodeItemMappingFields builds the persisted shape of one item-
// mapping entry. Returns a map[string]any so the parent map can carry
// it through TOML round-trip.
func encodeItemMappingFields(serverID, baseVersion, clientID string) map[string]any {
	return map[string]any{
		itemMappingFieldServerID:    serverID,
		itemMappingFieldBaseVersion: baseVersion,
		itemMappingFieldClientID:    clientID,
	}
}

// listItemNodeToRemoteItem normalizes a gateway.Node (LIST_ITEM) into
// the engine's RemoteItem shape. Title carries the full Text (head +
// inline tags + description suffix); the translator splits it on the
// way to WikiItem.
func listItemNodeToRemoteItem(n gateway.Node) connectors.RemoteItem {
	deleted := !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero()
	status := ""
	if n.Checked {
		status = "completed"
	}
	out := connectors.RemoteItem{
		Ref:      connectors.RemoteRef(n.ServerID),
		Title:    n.Text,
		Status:   status,
		Deleted:  deleted,
		Updated:  n.Timestamps.Updated,
		Position: n.SortValue,
		Vendor: map[string]any{
			"base_version": n.BaseVersion,
			"client_id":    n.ID,
			"checked":      n.Checked,
			"node_text":    n.Text,
			"sort_value":   n.SortValue,
		},
	}
	return out
}

// remoteItemToListItemNode is the inverse of listItemNodeToRemoteItem
// for the inbound translator (RemoteToWiki) which feeds the
// translator's KeepToWiki primitive.
func remoteItemToListItemNode(remote connectors.RemoteItem) gateway.Node {
	checked := remote.Status == "completed"
	if v, ok := remote.Vendor["checked"].(bool); ok {
		checked = v
	}
	var baseVersion string
	if v, ok := remote.Vendor["base_version"].(string); ok {
		baseVersion = v
	}
	var clientID string
	if v, ok := remote.Vendor["client_id"].(string); ok {
		clientID = v
	}
	sortValue := remote.Position
	if sortValue == "" {
		if v, ok := remote.Vendor["sort_value"].(string); ok {
			sortValue = v
		}
	}
	text := remote.Title
	if v, ok := remote.Vendor["node_text"].(string); ok && v != "" {
		text = v
	}
	return gateway.Node{
		ID:          clientID,
		ServerID:    string(remote.Ref),
		Type:        gateway.NodeTypeListItem,
		Text:        text,
		Checked:     checked,
		SortValue:   sortValue,
		BaseVersion: baseVersion,
		Timestamps: gateway.Timestamps{
			Updated: remote.Updated,
		},
	}
}

// wikiItemToProto adapts the engine's flat WikiItem to the proto type
// the translator consumes. Keep's translator emits proto's
// ChecklistItem; we round-trip via this helper.
func wikiItemToProto(item connectors.WikiItem) *apiv1.ChecklistItem {
	cl := &apiv1.ChecklistItem{
		Uid:       item.UID,
		Text:      item.Text,
		Checked:   item.Checked,
		Tags:      item.Tags,
		SortOrder: item.SortOrder,
	}
	if item.Description != "" {
		d := item.Description
		cl.Description = &d
	}
	return cl
}

// writeFailed reports whether the WriteResults entry for the given
// node id reports anything other than SUCCESS. A missing entry is
// treated as success (Keep doesn't always echo write results in
// pure-pull responses; only push responses populate the slice).
func writeFailed(results []gateway.NodeWriteResult, nodeID string) bool {
	if len(results) == 0 {
		return false
	}
	for _, r := range results {
		if r.ID == nodeID {
			return r.Status != "" && r.Status != "SUCCESS"
		}
	}
	return false
}

// buildKeepItemID returns a Keep-style id ("<ms-hex>.<16-hex-char>")
// derived deterministically from (now, salt). Matches the gkeepapi
// reference implementation's _generateId shape; salt is hashed to
// produce a stable suffix (so retries for the same wiki uid don't
// generate divergent client ids that would race in Keep's id space).
func buildKeepItemID(now time.Time, salt string) (string, error) {
	if salt == "" {
		return "", errors.New("google_keep: salt must not be empty")
	}
	sum := sha256.Sum256([]byte(salt))
	suffix := hex.EncodeToString(sum[:8])
	return fmt.Sprintf("%x.%s", now.UnixMilli(), suffix), nil
}
