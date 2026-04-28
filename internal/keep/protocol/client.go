package protocol

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultKeepBaseURL is the production Keep API base. Tests inject a
// stand-in URL via NewKeepClient.
const DefaultKeepBaseURL = "https://www.googleapis.com/notes/v1/"

// rfc3339Micros is Keep's wire format for timestamps. It's RFC3339 with
// microsecond precision and the literal Z suffix.
const rfc3339Micros = "2006-01-02T15:04:05.000000Z"

// KeepClient performs Keep API calls authenticated with a short-lived
// bearer obtained from Authenticator.ExchangeMasterTokenForBearer. The
// caller refreshes the bearer on ErrAuthRevoked and retries.
type KeepClient struct {
	httpClient *http.Client
	baseURL    string
	bearer     string
	debug      DebugLogger
}

// NewKeepClient constructs a KeepClient. baseURL is the Keep API base
// (production callers pass DefaultKeepBaseURL). bearer is the value to put
// after "OAuth " in the Authorization header (matches gkeepapi).
func NewKeepClient(httpClient *http.Client, baseURL, bearer string) *KeepClient {
	return &KeepClient{httpClient: httpClient, baseURL: baseURL, bearer: bearer}
}

// ListItemSpec is the input shape for creating a LIST_ITEM node when
// bundling items into the same Changes request that creates the LIST.
// SortValue is a numeric string; lower values sort to the bottom in the
// Keep app, so callers should compute it as (n-i)*1000 to keep the
// natural top-to-bottom order.
type ListItemSpec struct {
	Text      string
	Checked   bool
	SortValue string
}

// CreateListResult is what CreateListWithItems hands back. Item server
// IDs are index-aligned with the input items slice. The mapping is the
// thread an eventual sync engine pulls on to push subsequent edits to
// the right Keep nodes.
type CreateListResult struct {
	ListServerID  string
	ItemServerIDs []string
}

// CreateList creates a brand-new empty LIST node. Thin wrapper around
// CreateListWithItems for callers that don't have items to push yet.
func (c *KeepClient) CreateList(ctx context.Context, title string) (string, error) {
	r, err := c.CreateListWithItems(ctx, title, nil)
	if err != nil {
		return "", err
	}
	return r.ListServerID, nil
}

// CreateListWithItems creates a new LIST node and (optionally) its
// initial children in a SINGLE Changes request. This is the only shape
// Google's Keep backend accepts — splitting the create and the item
// push into two requests returns 500 "Unknown Error" because the second
// request's parent_id (the list's serverID) doesn't yet refer to a
// node the client has acknowledged. Bundled, the items reference the
// list's CLIENT id and the server resolves the linkage server-side.
func (c *KeepClient) CreateListWithItems(ctx context.Context, title string, items []ListItemSpec) (CreateListResult, error) {
	now := time.Now().UTC()
	listClientID, err := generateKeepID(now)
	if err != nil {
		return CreateListResult{}, err
	}
	sessionID, err := generateSessionID(now)
	if err != nil {
		return CreateListResult{}, err
	}

	nodes := make([]Node, 0, 1+len(items))
	nodes = append(nodes, Node{
		Kind:  "notes#node",
		ID:    listClientID,
		Type:  NodeTypeList,
		Title: title,
		Timestamps: Timestamps{
			Created: now,
			Updated: now,
		},
	})

	itemClientIDs := make([]string, len(items))
	for i, it := range items {
		// Items each get their own client id; bumping the ms component
		// by (i+1) preserves the user-facing top-to-bottom order in the
		// Keep app even if the random suffix ordering would otherwise
		// not.
		itemClientID, err := generateKeepID(now.Add(time.Duration(i+1) * time.Millisecond))
		if err != nil {
			return CreateListResult{}, err
		}
		itemClientIDs[i] = itemClientID
		nodes = append(nodes, Node{
			Kind:      "notes#node",
			ID:        itemClientID,
			Type:      NodeTypeListItem,
			ParentID:  listClientID,
			Text:      it.Text,
			Checked:   it.Checked,
			SortValue: it.SortValue,
			Timestamps: Timestamps{
				Created: now,
				Updated: now,
			},
		})
	}

	resp, err := c.Changes(ctx, ChangesRequest{
		Nodes:           nodes,
		SessionID:       sessionID,
		ClientTimestamp: clientTimestamp(now),
	})
	if err != nil {
		return CreateListResult{}, err
	}

	result := CreateListResult{
		ItemServerIDs: make([]string, len(items)),
	}
	for _, n := range resp.Nodes {
		if n.ID == listClientID && n.Type == NodeTypeList {
			result.ListServerID = n.ServerID
			continue
		}
		if n.Type == NodeTypeListItem {
			for i, want := range itemClientIDs {
				if n.ID == want {
					result.ItemServerIDs[i] = n.ServerID
					break
				}
			}
		}
	}
	if result.ListServerID == "" {
		return CreateListResult{}, fmt.Errorf("%w: server did not echo the created list", ErrProtocolDrift)
	}
	return result, nil
}

// generateKeepID returns a Keep-style identifier of the form
// "<ms-hex>.<16-hex-char random>". Matches the gkeepapi reference
// implementation's _generateId. crypto/rand failure is exceptional
// (entropy starvation on container startup) but real — surface as an
// error rather than silently producing all-zero ids that would collide
// across simultaneous CreateList calls.
func generateKeepID(now time.Time) (string, error) {
	var entropy [randomBytes]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	return fmt.Sprintf("%x.%016x", now.UnixMilli(), binary.BigEndian.Uint64(entropy[:])), nil
}

// generateSessionID returns a Keep-style session id ("s--<ms>--<10 digits>").
// See generateKeepID for the entropy-error rationale.
func generateSessionID(now time.Time) (string, error) {
	var entropy [randomBytes]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	n := binary.BigEndian.Uint64(entropy[:]) % sessionIDRange
	return fmt.Sprintf("s--%d--%010d", now.UnixMilli(), n+sessionIDOffset), nil
}

// clientTimestamp returns the wire format Keep expects: RFC3339 with
// microsecond precision and the literal Z suffix. Mirrors gkeepapi's
// NodeTimestamps.int_to_str / dt_to_str output (not a microseconds
// integer — an early implementation guess that returned a 16-digit
// number was rejected by the Keep API as "malformed").
func clientTimestamp(now time.Time) string {
	return now.UTC().Format(rfc3339Micros)
}

const (
	// randomBytes is how many bytes of entropy we read for a single Keep
	// ID or session id (8 bytes → uint64 → 16-hex-char).
	randomBytes = 8

	// sessionIDOffset and sessionIDRange together produce a 10-digit
	// session-id suffix in [1000000000, 9999999999], matching gkeepapi.
	sessionIDOffset = 1000000000
	sessionIDRange  = 9000000000
)

// DebugLogger is the optional sink Changes() writes a one-line summary
// to on every call. Set via SetDebugLogger; default is nil (silent).
// Method shape matches jcelliott/lumber's ConsoleLogger so the wiki's
// existing logger can be passed directly. Used to chase response-shape
// regressions; remove once diagnosed.
type DebugLogger interface {
	Info(format string, args ...any)
}

// SetDebugLogger attaches a debug logger; pass nil to silence.
func (c *KeepClient) SetDebugLogger(l DebugLogger) { c.debug = l }

// Changes calls POST /notes/v1/changes — the unified pull/push endpoint.
// req.TargetVersion is the cursor to pull *from* (empty = full pull);
// req.Nodes are mutations to push. Response carries the new cursor and
// any inbound diff.
func (c *KeepClient) Changes(ctx context.Context, req ChangesRequest) (ChangesResponse, error) {
	wireReq := buildChangesRequest(req)
	body, err := json.Marshal(wireReq)
	if err != nil {
		return ChangesResponse{}, fmt.Errorf("encode changes request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"changes", bytes.NewReader(body))
	if err != nil {
		return ChangesResponse{}, fmt.Errorf("build changes request: %w", err)
	}
	httpReq.Header.Set("Authorization", "OAuth "+c.bearer)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)

	// TEMP: dump push request bodies (anything with nodes>0) so the
	// "stage3 HTTP 500: Unknown Error" we're chasing on cron-tick
	// pushes is diagnosable. The bearer is on the header, not the
	// body, so this won't leak credentials. Strip once the 500 is
	// resolved.
	if c.debug != nil && len(req.Nodes) > 0 {
		c.debug.Info("keep changes REQUEST (nodes=%d targetVersion=%q): %s",
			len(req.Nodes), req.TargetVersion, truncateBody(body))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChangesResponse{}, fmt.Errorf("changes request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if classified := classifyHTTPStatus(resp.StatusCode); classified != nil {
		// Read the body so the error chain can show what Keep actually
		// said. Bodies are typically JSON like {"error":{"code":401,
		// "message":"..."}}; surfacing them is critical for diagnosing
		// "Stage 2 succeeded but the bearer doesn't pass the Keep API
		// auth check" — distinct from any of the auth-stage rejections.
		errBody, readErr := io.ReadAll(resp.Body)
		bodyTxt := truncateBody(errBody)
		if readErr != nil {
			bodyTxt = fmt.Sprintf("(body read failed: %v) %s", readErr, bodyTxt)
		}
		return ChangesResponse{}, fmt.Errorf("%w: stage3 HTTP %d: %s", classified, resp.StatusCode, bodyTxt)
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChangesResponse{}, fmt.Errorf("%w: read response: %w", ErrProtocolDrift, err)
	}
	if c.debug != nil {
		c.debug.Info("keep changes response (req nodes=%d targetVersion=%q): %s",
			len(req.Nodes), req.TargetVersion, truncateBody(rawBody))
	}

	var wireResp wireChangesResponse
	if err := json.Unmarshal(rawBody, &wireResp); err != nil {
		return ChangesResponse{}, fmt.Errorf("%w: decode response: %w", ErrProtocolDrift, err)
	}

	return decodeChangesResponse(wireResp)
}

// truncateBody bounds an HTTP body to 4 KB so a chatty payload doesn't
// blow out journalctl lines, and scrubs anything shaped like a bearer
// token / master token / oauth_token cookie value before the body lands
// in logs. Defensive — Google's documented Keep API errors don't echo
// credentials, but a future drift could. The 4 KB ceiling is enough to
// see the first ~10 nodes of a Changes response for diagnostics; full
// state can run to MBs and we don't need it.
func truncateBody(b []byte) string {
	// TEMP: bumped from 4KB to 32KB to read past the userInfo/labels
	// preamble and see LIST_ITEM bodies in journalctl. Strip back to
	// 4KB once the version-field key is identified.
	const maxLen = 32768
	if len(b) > maxLen {
		b = b[:maxLen]
	}
	cleaned := strings.Map(func(r rune) rune {
		if r >= 0x20 && r < 0x7f {
			return r
		}
		if r == '\n' || r == '\t' {
			return ' '
		}
		return '?'
	}, string(b))
	cleaned = bearerLikeRE.ReplaceAllString(cleaned, "[REDACTED]")
	cleaned = masterTokenLikeRE.ReplaceAllString(cleaned, "[REDACTED]")
	return cleaned
}

// bearerLikeRE matches Google-style OAuth bearer tokens
// (ya29.<long-string>) wherever they appear in a body.
var bearerLikeRE = regexp.MustCompile(`ya29\.[A-Za-z0-9_-]{20,}`)

// masterTokenLikeRE matches gpsoauth-style master/refresh tokens
// (oauth2rt_<digits>/...) and oauth_token cookie values
// (oauth2_<digits>/...).
var masterTokenLikeRE = regexp.MustCompile(`oauth2(?:rt)?_[0-9]+/[A-Za-z0-9_/+=-]{20,}`)

// classifyHTTPStatus maps Keep API status codes to typed errors. nil means
// "proceed with body decode."
func classifyHTTPStatus(code int) error {
	switch code {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrAuthRevoked
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusNotFound:
		return ErrBoundNoteDeleted
	default:
		return fmt.Errorf("keep: unexpected status %d", code)
	}
}

// --- wire types -----------------------------------------------------------
//
// The wire shape doesn't quite match the public Node — JSON has nested
// timestamps and stringly-typed enum fields. Keep these private; convert
// at the boundary.

type wireChangesRequest struct {
	Nodes           []wireNode     `json:"nodes"`
	ClientTimestamp string         `json:"clientTimestamp"`
	RequestHeader   wireReqHeader  `json:"requestHeader"`
	TargetVersion   string         `json:"targetVersion,omitempty"`
	UserInfo        *wireUserInfo  `json:"userInfo,omitempty"`
}

// wireUserInfo carries label CRUD on the request side; on the response
// side it carries the user's full label state. We only populate the
// labels-pushed channel here; settings/coachmarks/etc. round-trip from
// the server but we don't model them.
type wireUserInfo struct {
	Labels []wireLabel `json:"labels,omitempty"`
}

type wireLabel struct {
	MainID     string         `json:"mainId"`
	Name       string         `json:"name"`
	Timestamps wireTimestamps `json:"timestamps,omitempty"`
}

type wireReqHeader struct {
	ClientSessionID string         `json:"clientSessionId"`
	ClientPlatform  string         `json:"clientPlatform"`
	ClientVersion   wireClientVer  `json:"clientVersion"`
	Capabilities    []wireCapEntry `json:"capabilities"`
}

type wireClientVer struct {
	Major    string `json:"major"`
	Minor    string `json:"minor"`
	Build    string `json:"build"`
	Revision string `json:"revision"`
}

type wireCapEntry struct {
	Type string `json:"type"`
}

type wireChangesResponse struct {
	Kind            string        `json:"kind"`
	ToVersion       *string       `json:"toVersion"`
	Nodes           []wireNode    `json:"nodes"`
	UserInfo        *wireUserInfo `json:"userInfo,omitempty"`
	ForceFullResync bool          `json:"forceFullResync"`
	Truncated       bool          `json:"truncated"`
}

type wireNode struct {
	Kind           string         `json:"kind,omitempty"`
	ID             string         `json:"id"`
	ServerID       string         `json:"serverId,omitempty"`
	ParentID       string         `json:"parentId,omitempty"`
	ParentServerID string         `json:"parentServerId,omitempty"`
	Type           string         `json:"type"`
	Title          string         `json:"title,omitempty"`
	Text           string         `json:"text,omitempty"`
	Checked        bool           `json:"checked,omitempty"`
	SortValue      string         `json:"sortValue,omitempty"`
	BaseVersion    string         `json:"baseVersion,omitempty"`
	LabelIDs       []wireLabelID  `json:"labelIds,omitempty"`
	Timestamps     wireTimestamps `json:"timestamps"`
}

// wireLabelID is the per-node label assignment shape: a labelId
// reference plus a deleted-timestamp marker (zero = assigned;
// non-zero RFC3339 = unassigned). gkeepapi node.py:1162-1174.
type wireLabelID struct {
	LabelID string `json:"labelId"`
	Deleted string `json:"deleted,omitempty"`
}

type wireTimestamps struct {
	Kind       string `json:"kind,omitempty"`
	Created    string `json:"created,omitempty"`
	Updated    string `json:"updated,omitempty"`
	UserEdited string `json:"userEdited,omitempty"`
	Trashed    string `json:"trashed,omitempty"`
	Deleted    string `json:"deleted,omitempty"`
}

// --- request build --------------------------------------------------------

// supportedCapabilities mirrors the gkeepapi capability set. See REFERENCE.md
// "Capability flags".
var supportedCapabilities = []wireCapEntry{
	{Type: "NC"}, {Type: "PI"}, {Type: "LB"}, {Type: "AN"}, {Type: "SH"},
	{Type: "DR"}, {Type: "TR"}, {Type: "IN"}, {Type: "SNB"}, {Type: "MI"},
	{Type: "CO"},
}

// claimedClientVersion is the keep-Android client version we masquerade as.
// The exact values don't matter for any field beyond "all four equal" — we
// pin them here so they stay in sync.
const claimedClientVersion = "9"

func buildChangesRequest(req ChangesRequest) wireChangesRequest {
	wireNodes := make([]wireNode, len(req.Nodes))
	for i, n := range req.Nodes {
		wireNodes[i] = encodeNode(n)
	}
	out := wireChangesRequest{
		Nodes:           wireNodes,
		ClientTimestamp: req.ClientTimestamp,
		TargetVersion:   req.TargetVersion,
		RequestHeader: wireReqHeader{
			ClientSessionID: req.SessionID,
			ClientPlatform:  "ANDROID",
			ClientVersion: wireClientVer{
				Major:    claimedClientVersion,
				Minor:    claimedClientVersion,
				Build:    claimedClientVersion,
				Revision: claimedClientVersion,
			},
			Capabilities: supportedCapabilities,
		},
	}
	if len(req.Labels) > 0 {
		labels := make([]wireLabel, len(req.Labels))
		for i, l := range req.Labels {
			labels[i] = wireLabel{
				MainID: l.MainID,
				Name:   l.Name,
				Timestamps: wireTimestamps{
					Kind:    "notes#timestamps",
					Created: rfcOrEmpty(l.Created),
					Updated: rfcOrEmpty(l.Updated),
					Deleted: rfcOrEmpty(l.Deleted),
				},
			}
		}
		out.UserInfo = &wireUserInfo{Labels: labels}
	}
	return out
}

// rfcOrEmpty formats t using rfc3339Micros, returning "" for zero
// times. Centralized so encodeTimestamps and the label encoder use
// the same convention.
func rfcOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(rfc3339Micros)
}

func encodeNode(n Node) wireNode {
	out := wireNode{
		Kind:           firstNonEmpty(n.Kind, "notes#node"),
		ID:             n.ID,
		ServerID:       n.ServerID,
		ParentID:       n.ParentID,
		ParentServerID: n.ParentServerID,
		Type:           string(n.Type),
		Title:          n.Title,
		Text:           n.Text,
		Checked:        n.Checked,
		SortValue:      n.SortValue,
		BaseVersion:    n.BaseVersion,
		Timestamps:     encodeTimestamps(n.Timestamps),
	}
	if len(n.LabelIDs) > 0 {
		out.LabelIDs = make([]wireLabelID, len(n.LabelIDs))
		for i, id := range n.LabelIDs {
			out.LabelIDs[i] = wireLabelID{LabelID: id}
		}
	}
	return out
}

func encodeTimestamps(t Timestamps) wireTimestamps {
	out := wireTimestamps{Kind: "notes#timestamps"}
	if !t.Created.IsZero() {
		out.Created = t.Created.UTC().Format(rfc3339Micros)
	}
	if !t.Updated.IsZero() {
		out.Updated = t.Updated.UTC().Format(rfc3339Micros)
	}
	if !t.UserEdited.IsZero() {
		out.UserEdited = t.UserEdited.UTC().Format(rfc3339Micros)
	}
	if !t.Trashed.IsZero() {
		out.Trashed = t.Trashed.UTC().Format(rfc3339Micros)
	}
	if !t.Deleted.IsZero() {
		out.Deleted = t.Deleted.UTC().Format(rfc3339Micros)
	}
	return out
}

// --- response decode ------------------------------------------------------

// recognizedNodeTypes is the set of NodeTypes the wiki actively models.
// Decoder filters anything else out without erroring (forward-compat with
// future Keep node types).
var recognizedNodeTypes = map[NodeType]struct{}{
	NodeTypeNote:     {},
	NodeTypeList:     {},
	NodeTypeListItem: {},
	NodeTypeBlob:     {},
}

func decodeChangesResponse(w wireChangesResponse) (ChangesResponse, error) {
	// Structural-drift guard: toVersion is required.
	if w.ToVersion == nil {
		return ChangesResponse{}, fmt.Errorf("%w: missing toVersion", ErrProtocolDrift)
	}

	out := ChangesResponse{
		ToVersion:       *w.ToVersion,
		ForceFullResync: w.ForceFullResync,
		Truncated:       w.Truncated,
	}
	for _, wn := range w.Nodes {
		nt := NodeType(wn.Type)
		if _, ok := recognizedNodeTypes[nt]; !ok {
			// Unknown node type: skip silently (forward compatibility).
			continue
		}
		n, err := decodeNode(wn)
		if err != nil {
			return ChangesResponse{}, fmt.Errorf("%w: node %q: %w", ErrProtocolDrift, wn.ID, err)
		}
		out.Nodes = append(out.Nodes, n)
	}
	if w.UserInfo != nil {
		for _, wl := range w.UserInfo.Labels {
			le, err := decodeLabel(wl)
			if err != nil {
				return ChangesResponse{}, fmt.Errorf("%w: label %q: %w", ErrProtocolDrift, wl.MainID, err)
			}
			out.Labels = append(out.Labels, le)
		}
	}
	return out, nil
}

func decodeLabel(w wireLabel) (LabelEntry, error) {
	created, err := parseRFC3339Micros(w.Timestamps.Created)
	if err != nil {
		return LabelEntry{}, fmt.Errorf("created: %w", err)
	}
	updated, err := parseRFC3339Micros(w.Timestamps.Updated)
	if err != nil {
		return LabelEntry{}, fmt.Errorf("updated: %w", err)
	}
	deleted, err := parseRFC3339Micros(w.Timestamps.Deleted)
	if err != nil {
		return LabelEntry{}, fmt.Errorf("deleted: %w", err)
	}
	return LabelEntry{
		MainID:  w.MainID,
		Name:    w.Name,
		Created: created,
		Updated: updated,
		Deleted: deleted,
	}, nil
}

func decodeNode(w wireNode) (Node, error) {
	ts, err := decodeTimestamps(w.Timestamps)
	if err != nil {
		return Node{}, fmt.Errorf("timestamps: %w", err)
	}
	var labelIDs []string
	for _, l := range w.LabelIDs {
		// Skip entries marked deleted (Keep's tombstone for "label
		// removed from this note").
		if l.Deleted != "" && l.Deleted != "1970-01-01T00:00:00.000Z" {
			continue
		}
		if l.LabelID != "" {
			labelIDs = append(labelIDs, l.LabelID)
		}
	}
	return Node{
		Kind:           w.Kind,
		ID:             w.ID,
		ServerID:       w.ServerID,
		ParentID:       w.ParentID,
		ParentServerID: w.ParentServerID,
		Type:           NodeType(w.Type),
		Title:          w.Title,
		Text:           w.Text,
		Checked:        w.Checked,
		SortValue:      w.SortValue,
		BaseVersion:    w.BaseVersion,
		LabelIDs:       labelIDs,
		Timestamps:     ts,
	}, nil
}

func decodeTimestamps(w wireTimestamps) (Timestamps, error) {
	created, err := parseRFC3339Micros(w.Created)
	if err != nil {
		return Timestamps{}, fmt.Errorf("created: %w", err)
	}
	updated, err := parseRFC3339Micros(w.Updated)
	if err != nil {
		return Timestamps{}, fmt.Errorf("updated: %w", err)
	}
	userEdited, err := parseRFC3339Micros(w.UserEdited)
	if err != nil {
		return Timestamps{}, fmt.Errorf("userEdited: %w", err)
	}
	trashed, err := parseRFC3339Micros(w.Trashed)
	if err != nil {
		return Timestamps{}, fmt.Errorf("trashed: %w", err)
	}
	deleted, err := parseRFC3339Micros(w.Deleted)
	if err != nil {
		return Timestamps{}, fmt.Errorf("deleted: %w", err)
	}
	return Timestamps{
		Created:    created,
		Updated:    updated,
		UserEdited: userEdited,
		Trashed:    trashed,
		Deleted:    deleted,
	}, nil
}

// parseRFC3339Micros returns zero/no-error for absent input ("") and
// errors loudly on a non-empty unparseable input. Silently returning
// zero on parse failure would collapse "trashed" / "deleted" timestamps
// into "live" — surfacing tombstones as if they were active notes.
//
// Keep also uses the literal Unix epoch ("1970-01-01T00:00:00.000Z")
// as a sentinel meaning "this timestamp doesn't apply" — observed on
// alive notes' Trashed/Deleted fields. Treat that as zero so the
// IsZero() filters elsewhere correctly classify the note as alive.
// Verified by reading the user's real-account Changes response: the
// kept-but-not-trashed Grocery list returned trashed=epoch and our
// IsZero check was wrongly treating epoch as "trashed in 1970."
func parseRFC3339Micros(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(rfc3339Micros, s)
	if err != nil {
		// Try the more lenient RFC3339Nano in case Google trims trailing
		// zeros. If still invalid, the caller gets an error.
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("not a valid RFC3339 timestamp: %w", err)
		}
	}
	if t.Unix() == 0 {
		return time.Time{}, nil
	}
	return t.UTC(), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
