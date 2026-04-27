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
	Type        NodeType
	Text        string
	SortValue   string
	BaseVersion string
	Checked     bool
	Timestamps  Timestamps
}

// ChangesRequest is the body of POST /notes/v1/changes — both pulls (with
// TargetVersion) and pushes (with Nodes populated). See REFERENCE.md
// "Sync endpoint".
type ChangesRequest struct {
	Nodes           []Node
	TargetVersion   string
	ClientTimestamp string
	SessionID       string
}

// ChangesResponse is the parsed body returned by the Keep server.
type ChangesResponse struct {
	ToVersion       string
	Nodes           []Node
	ForceFullResync bool
	Truncated       bool
}
