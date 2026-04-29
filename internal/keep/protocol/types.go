package protocol

import "time"

// NodeType identifies a Keep node's role in the hierarchy. Only the values
// the wiki actively models are listed; future types from Google land in
// UnknownNodeType after decode and are skipped by sync (they don't surface
// as ErrProtocolDrift — that's reserved for *structural* drift like a
// missing required field).
type NodeType string

const (
	NodeTypeNote        NodeType = "NOTE"
	NodeTypeList        NodeType = "LIST"
	NodeTypeListItem    NodeType = "LIST_ITEM"
	NodeTypeBlob        NodeType = "BLOB"
	UnknownNodeType     NodeType = ""
)

// Timestamps mirrors Keep's timestamps subobject. RFC3339-ish strings on
// the wire ("2026-04-25T17:14:00.000000Z"); we parse to time.Time. Optional
// fields are zero-Time when absent.
type Timestamps struct {
	Created    time.Time
	Updated    time.Time
	UserEdited time.Time
	Trashed    time.Time
	Deleted    time.Time
}

// Node is the union of fields used by the node types the wiki cares about.
// We model it as a single struct rather than an interface tree because the
// gRPC bridge consumes a flat shape and the type axis is just a NodeType
// field on the wire. Future Keep node types decode into this same struct
// with NodeType == UnknownNodeType and the extra fields absent.
type Node struct {
	Kind        string
	ID          string
	ServerID    string
	ParentID    string
	// ParentServerID is required on a LIST_ITEM push when the parent
	// LIST already exists server-side. Without it Keep returns a 500
	// "Unknown Error" — the server uses parent_server_id to confirm
	// the client is referring to the same parent it has on file.
	// gkeepapi node.py line 1585; set by ListItem(parent_server_id=...)
	// at every incremental edit. Empty when the list is brand-new in
	// the same request (Keep resolves via parent_id alone in that case).
	ParentServerID string
	Type           NodeType
	Title          string // top-level Note/List title (empty on ListItem)
	Text           string
	SortValue      string
	BaseVersion    string
	Checked        bool
	// LabelIDs is the set of Keep Label MainIDs assigned to this top-
	// level Note/List. Empty on LIST_ITEM. The wiki-side sync engine
	// reads page hashtags and pushes them through here.
	LabelIDs   []string
	Timestamps Timestamps
}

// ChangesRequest is the body of POST /notes/v1/changes — both pulls (with
// TargetVersion) and pushes (with Nodes populated). See REFERENCE.md
// "Sync endpoint".
//
// Labels are sent under userInfo.labels (separate from Nodes); see
// gkeepapi __init__.py:365. Each label CRUD entry has its own mainId
// + name + timestamps; deletion is signaled by setting timestamps.deleted
// (gkeepapi node.py:1162-1174).
type ChangesRequest struct {
	Nodes           []Node
	Labels          []LabelEntry // CRUD batch — added/renamed/deleted labels
	TargetVersion   string
	ClientTimestamp string
	SessionID       string
}

// ChangesResponse is the parsed body returned by the Keep server.
//
// Labels echoes the user's full label set after the sync. Used by the
// outbound-sync engine to find or create the right Keep Label for each
// wiki page hashtag without pushing duplicates.
//
// WriteResults is the per-pushed-node status echoed by Keep on a push.
// Empty on pure pulls. Populated when the request carried Nodes; one
// entry per pushed node, keyed by client-side ID. Used by the bridge
// to decide whether to advance synced_fp for a soft-delete: a SUCCESS
// status means Keep accepted the mutation; anything else means the
// push silently failed and the bridge must NOT mark the binding synced.
// Wire shape was unverified empirically when this field was added —
// the cmd/keep-debug `dump-write-results` subcommand exists to
// inspect the raw response for confirmation. See task #74 in the
// Keep bridge plan.
type ChangesResponse struct {
	ToVersion       string
	Nodes           []Node
	Labels          []LabelEntry
	WriteResults    []NodeWriteResult
	ForceFullResync bool
	Truncated       bool
}

// NodeWriteResult is the per-pushed-node status entry from Keep's
// Changes response. Best-guess shape based on prior keep-debug logs:
// id (client-side node ID echoed back) and status (e.g. "SUCCESS").
// If the wire shape turns out richer, additional fields land here as
// they're confirmed.
type NodeWriteResult struct {
	ID     string
	Status string
}

// LabelEntry is one Keep Label as it appears in userInfo.labels (both
// directions). Identity is MainID — a Keep-internal id with the same
// "ms-hex.16-hex" shape as Node IDs. Name is the human label text;
// adding the same name twice is idempotent if the same MainID is reused.
//
// Deleted is set on outbound to signal "delete this label" (Keep's
// soft-delete on labels). On inbound, non-zero Deleted means the label
// is tombstoned.
type LabelEntry struct {
	MainID  string
	Name    string
	Created time.Time
	Updated time.Time
	Deleted time.Time
}
