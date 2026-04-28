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

// CreateList creates a brand-new LIST node in the user's Keep account
// with the given title. Returns the server-assigned ID so callers can
// store it as the binding's keep_note_id.
//
// Used by the bind flow when the user picks "Create new Keep note named
// '<list_name>'" from the picker.
func (c *KeepClient) CreateList(ctx context.Context, title string) (string, error) {
	now := time.Now().UTC()
	clientID := generateKeepID(now)
	sessionID := generateSessionID(now)

	listNode := Node{
		Kind: "notes#node",
		ID:   clientID,
		Type: NodeTypeList,
		Text: title,
		Timestamps: Timestamps{
			Created: now,
			Updated: now,
		},
	}

	resp, err := c.Changes(ctx, ChangesRequest{
		Nodes:           []Node{listNode},
		SessionID:       sessionID,
		ClientTimestamp: clientTimestamp(now),
	})
	if err != nil {
		return "", err
	}

	for _, n := range resp.Nodes {
		if n.ID == clientID && n.Type == NodeTypeList {
			return n.ServerID, nil
		}
	}
	return "", fmt.Errorf("%w: server did not echo the created list", ErrProtocolDrift)
}

// generateKeepID returns a Keep-style identifier of the form
// "<ms-hex>.<16-hex-char random>". Matches the gkeepapi reference
// implementation's _generateId.
func generateKeepID(now time.Time) string {
	var entropy [randomBytes]byte
	_, _ = io.ReadFull(rand.Reader, entropy[:])
	return fmt.Sprintf("%x.%016x", now.UnixMilli(), binary.BigEndian.Uint64(entropy[:]))
}

// generateSessionID returns a Keep-style session id ("s--<ms>--<10 digits>").
func generateSessionID(now time.Time) string {
	var entropy [randomBytes]byte
	_, _ = io.ReadFull(rand.Reader, entropy[:])
	n := binary.BigEndian.Uint64(entropy[:]) % sessionIDRange
	return fmt.Sprintf("s--%d--%010d", now.UnixMilli(), n+sessionIDOffset)
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
		errBody, _ := io.ReadAll(resp.Body)
		return ChangesResponse{}, fmt.Errorf("%w: stage3 HTTP %d: %s", classified, resp.StatusCode, truncateBody(errBody))
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
	const maxLen = 4096
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
	Nodes           []wireNode    `json:"nodes"`
	ClientTimestamp string        `json:"clientTimestamp"`
	RequestHeader   wireReqHeader `json:"requestHeader"`
	TargetVersion   string        `json:"targetVersion,omitempty"`
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
	Kind            string     `json:"kind"`
	ToVersion       *string    `json:"toVersion"`
	Nodes           []wireNode `json:"nodes"`
	ForceFullResync bool       `json:"forceFullResync"`
	Truncated       bool       `json:"truncated"`
}

type wireNode struct {
	Kind        string         `json:"kind,omitempty"`
	ID          string         `json:"id"`
	ServerID    string         `json:"serverId,omitempty"`
	ParentID    string         `json:"parentId,omitempty"`
	Type        string         `json:"type"`
	Title       string         `json:"title,omitempty"`
	Text        string         `json:"text,omitempty"`
	Checked     bool           `json:"checked,omitempty"`
	SortValue   string         `json:"sortValue,omitempty"`
	BaseVersion string         `json:"baseVersion,omitempty"`
	Timestamps  wireTimestamps `json:"timestamps"`
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
	return wireChangesRequest{
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
}

func encodeNode(n Node) wireNode {
	out := wireNode{
		Kind:        firstNonEmpty(n.Kind, "notes#node"),
		ID:          n.ID,
		ServerID:    n.ServerID,
		ParentID:    n.ParentID,
		Type:        string(n.Type),
		Title:       n.Title,
		Text:        n.Text,
		Checked:     n.Checked,
		SortValue:   n.SortValue,
		BaseVersion: n.BaseVersion,
		Timestamps:  encodeTimestamps(n.Timestamps),
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
		out.Nodes = append(out.Nodes, decodeNode(wn))
	}
	return out, nil
}

func decodeNode(w wireNode) Node {
	return Node{
		Kind:        w.Kind,
		ID:          w.ID,
		ServerID:    w.ServerID,
		ParentID:    w.ParentID,
		Type:        NodeType(w.Type),
		Title:       w.Title,
		Text:        w.Text,
		Checked:     w.Checked,
		SortValue:   w.SortValue,
		BaseVersion: w.BaseVersion,
		Timestamps:  decodeTimestamps(w.Timestamps),
	}
}

func decodeTimestamps(w wireTimestamps) Timestamps {
	return Timestamps{
		Created:    parseRFC3339Micros(w.Created),
		Updated:    parseRFC3339Micros(w.Updated),
		UserEdited: parseRFC3339Micros(w.UserEdited),
		Trashed:    parseRFC3339Micros(w.Trashed),
		Deleted:    parseRFC3339Micros(w.Deleted),
	}
}

func parseRFC3339Micros(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(rfc3339Micros, s)
	if err != nil {
		// Try the more lenient RFC3339Nano in case Google trims trailing
		// zeros. If still invalid, return zero — caller treats as "absent".
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return time.Time{}
		}
	}
	return t.UTC()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
