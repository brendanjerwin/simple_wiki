package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultGoogleTokenURL is the production OAuth 2.0 token endpoint
// (RFC 6749 §3.2). Tests inject a stand-in URL via NewRefreshClient.
const DefaultGoogleTokenURL = "https://oauth2.googleapis.com/token"

// DefaultGoogleAuthURL is the production OAuth 2.0 authorization
// endpoint. Phase 6's `/oauth/google/callback` handler uses
// `BuildAuthURL` with this URL to start the auth-code grant; the
// gateway exports it so callers don't have to hard-code Google's
// addresses in two places.
const DefaultGoogleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"

// GoogleIssuer is the value RFC 9207 expects in the callback `iss`
// parameter when accounts.google.com is the authorization server.
// Pinned here so PKCE+iss validation is one config change away from
// supporting iCloud/Microsoft (per plan OAuth security profile).
const GoogleIssuer = "https://accounts.google.com"

// TasksScope is the Tasks-API OAuth scope. tasks.readonly does NOT
// satisfy tasks.insert/patch/delete (per plan §"Scope"). This is the
// scope the post-exchange echo check requires (RFC 6749 §3.3).
const TasksScope = "https://www.googleapis.com/auth/tasks"

// RequestedScopes is the full scope string sent on the auth-URL. We
// add OIDC's openid+email so the token-endpoint response includes a
// signed id_token whose claims carry the verified account email. The
// connector's downstream identity attribution (state.Email) depends
// on it — without these scopes Google issues an id_token with no
// email claim, the user's profile shows "Connected as ." with empty
// value, and Tasks-side system writes fall through to the generic
// "system:connector-sync" attribution. Order isn't significant; the
// space-separated form matches Google's documented examples.
const RequestedScopes = "openid email " + TasksScope

// accessTokenLeewaySeconds is how long before a cached access token's
// nominal expiry we consider it "stale" and refresh proactively. Google
// access tokens are issued with one-hour expiry; refreshing 60s early
// keeps a long-running Sync from straddling the expiry boundary.
const accessTokenLeewaySeconds = 60

// RefreshTokenStore is the persistence contract the RefreshClient uses
// to honor RFC 6749 §10.4 atomic rotation. Implementations write the
// refresh token to durable storage (the wiki's user-profile frontmatter)
// before the new access token is consumed by the caller — write-ahead
// persist prevents loss-on-crash.
type RefreshTokenStore interface {
	// LoadRefreshToken returns the currently-stored refresh token. The
	// RefreshClient calls this on first use and after rotation.
	LoadRefreshToken(ctx context.Context) (string, error)

	// SaveRefreshToken atomically replaces the stored refresh token.
	// Must complete (or fail loudly) before the RefreshClient hands
	// the new access token back to the caller.
	SaveRefreshToken(ctx context.Context, token string) error
}

// StaticRefreshTokenStore is an in-memory RefreshTokenStore for tests
// and the tasks-debug CLI. Production callers wire a frontmatter-backed
// store from the connector package.
type StaticRefreshTokenStore struct {
	mu    sync.Mutex
	token string
}

// NewStaticRefreshTokenStore seeds an in-memory store with the supplied
// refresh token.
func NewStaticRefreshTokenStore(initial string) *StaticRefreshTokenStore {
	return &StaticRefreshTokenStore{token: initial}
}

// LoadRefreshToken returns the currently-held token.
func (s *StaticRefreshTokenStore) LoadRefreshToken(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == "" {
		return "", errors.New("tasks: refresh token store is empty")
	}
	return s.token, nil
}

// SaveRefreshToken replaces the in-memory token. Returns nil even if
// the new token equals the old — Google's rotation contract is "treat
// the response field as authoritative when present."
func (s *StaticRefreshTokenStore) SaveRefreshToken(_ context.Context, token string) error {
	if token == "" {
		return errors.New("tasks: refresh token must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	return nil
}

// Snapshot returns the token currently held — test helper.
func (s *StaticRefreshTokenStore) Snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// RefreshClient exchanges a refresh token for a short-lived bearer
// access token, caching it until expiry. Implements:
//
//   - refresh_token grant against the token endpoint (RFC 6749 §6).
//   - Atomic refresh-token rotation (RFC 6749 §10.4): if the token
//     endpoint returns a new refresh_token, the store SaveRefreshToken
//     call completes BEFORE the new access token is returned.
//   - invalid_grant retry-once (RFC 9700 §4.14.2): a single retry
//     handles the rotation race where the persisted token is one
//     generation behind the in-flight one.
//   - Scope echo check (RFC 6749 §3.3): if the response scope is not
//     empty AND does not include the wiki's required scope, return
//     ErrScopeDowngraded — never silently use a downgraded grant.
//
// Concurrent calls are serialized via mu; the first caller does the
// network round-trip, subsequent callers within the leeway window get
// the cached token.
type RefreshClient struct {
	httpClient   *http.Client
	tokenURL     string
	clientID     string
	clientSecret string
	store        RefreshTokenStore
	now          func() time.Time

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// NewRefreshClient wires a RefreshClient. tokenURL is normally
// DefaultGoogleTokenURL; tests inject httptest.Server.URL. clientID
// and clientSecret are operator-configured (env vars per plan).
// store is the persistence layer for atomic rotation.
func NewRefreshClient(httpClient *http.Client, tokenURL, clientID, clientSecret string, store RefreshTokenStore) (*RefreshClient, error) {
	if httpClient == nil {
		return nil, errors.New("tasks: httpClient must not be nil")
	}
	if tokenURL == "" {
		return nil, errors.New("tasks: tokenURL must not be empty")
	}
	if clientID == "" {
		return nil, errors.New("tasks: clientID must not be empty")
	}
	if clientSecret == "" {
		return nil, errors.New("tasks: clientSecret must not be empty")
	}
	if store == nil {
		return nil, errors.New("tasks: refresh token store must not be nil")
	}
	return &RefreshClient{
		httpClient:   httpClient,
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		store:        store,
		now:          time.Now,
	}, nil
}

// AccessToken returns a fresh bearer access token, refreshing if the
// cached one is missing or within accessTokenLeewaySeconds of expiry.
// Concurrent callers within the leeway window get the same cached
// token; only one network round-trip happens at a time.
func (c *RefreshClient) AccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && c.now().Add(time.Duration(accessTokenLeewaySeconds)*time.Second).Before(c.expiresAt) {
		return c.accessToken, nil
	}

	resp, err := c.refreshLocked(ctx)
	if err != nil {
		return "", err
	}
	c.accessToken = resp.AccessToken
	c.expiresAt = c.now().Add(resp.ExpiresIn)
	return c.accessToken, nil
}

// Invalidate clears the cached access token. Callers use this on a 401
// from a downstream Tasks REST call to force a refresh on the next
// AccessToken request.
func (c *RefreshClient) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.expiresAt = time.Time{}
}

// ExpiresAt reports the cached access token's expiry. Used by the
// tasks-debug CLI's verbose output. Zero when no token is cached.
func (c *RefreshClient) ExpiresAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.expiresAt
}

// refreshLocked is invoked with c.mu held. Performs the refresh-grant
// network call, handles rotation + invalid_grant retry-once, and
// returns the parsed response on success.
func (c *RefreshClient) refreshLocked(ctx context.Context) (TokenResponse, error) {
	refreshToken, err := c.store.LoadRefreshToken(ctx)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("load refresh token: %w", err)
	}

	resp, err := c.exchangeRefreshGrant(ctx, refreshToken)
	if errors.Is(err, ErrInvalidGrant) {
		// RFC 9700 §4.14.2: a single retry handles the rotation race
		// where the persisted token is one generation behind the
		// in-flight one. Re-load the store — it may have been updated
		// by a concurrent process — and try once more before giving
		// up.
		retryToken, loadErr := c.store.LoadRefreshToken(ctx)
		if loadErr != nil {
			return TokenResponse{}, fmt.Errorf("retry load refresh token: %w", loadErr)
		}
		if retryToken == refreshToken {
			// Same token, same outcome — stop here.
			return TokenResponse{}, err
		}
		resp, err = c.exchangeRefreshGrant(ctx, retryToken)
	}
	if err != nil {
		return TokenResponse{}, err
	}

	// RFC 6749 §10.4 atomic rotation: persist BEFORE consuming.
	if resp.RefreshToken != "" && resp.RefreshToken != refreshToken {
		if saveErr := c.store.SaveRefreshToken(ctx, resp.RefreshToken); saveErr != nil {
			return TokenResponse{}, fmt.Errorf("persist rotated refresh token: %w", saveErr)
		}
	}
	return resp, nil
}

// exchangeRefreshGrant POSTs the refresh_token grant to the token
// endpoint and decodes the response. Handles invalid_grant, scope
// echo, and protocol drift. Surfaces typed sentinels so callers can
// branch via errors.Is.
func (c *RefreshClient) exchangeRefreshGrant(ctx context.Context, refreshToken string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("%w: read token response: %w", ErrProtocolDrift, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return TokenResponse{}, classifyTokenError(httpResp.StatusCode, body)
	}

	var wire wireTokenResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return TokenResponse{}, fmt.Errorf("%w: decode token response: %w", ErrProtocolDrift, err)
	}
	if wire.AccessToken == "" {
		return TokenResponse{}, fmt.Errorf("%w: token response missing access_token", ErrProtocolDrift)
	}

	if wire.Scope != "" && !scopeIncludes(wire.Scope, TasksScope) {
		return TokenResponse{}, fmt.Errorf("%w: granted scope %q does not include %q", ErrScopeDowngraded, wire.Scope, TasksScope)
	}

	return TokenResponse{
		AccessToken:  wire.AccessToken,
		TokenType:    wire.TokenType,
		ExpiresIn:    time.Duration(wire.ExpiresIn) * time.Second,
		RefreshToken: wire.RefreshToken,
		Scope:        wire.Scope,
	}, nil
}

// wireTokenResponse mirrors RFC 6749 §5.1 (success) and Google's
// extension fields. Decode error or missing access_token is structural
// drift.
type wireTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// wireTokenError mirrors RFC 6749 §5.2.
type wireTokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// classifyTokenError maps an OAuth token-endpoint error response to
// a typed sentinel. Per CLAUDE.md "Never Branch Logic on Error
// Messages" — but RFC 6749 §5.2 makes the `error` *field* (not
// description) the structured branch point, so this is fine.
func classifyTokenError(status int, body []byte) error {
	var wire wireTokenError
	_ = json.Unmarshal(body, &wire)
	switch wire.Error {
	case "invalid_grant":
		return fmt.Errorf("%w: %s", ErrInvalidGrant, wire.ErrorDescription)
	case "invalid_scope":
		return fmt.Errorf("%w: %s", ErrScopeDowngraded, wire.ErrorDescription)
	default:
		// fall through to status-based classification below
	}
	switch status {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: token endpoint", ErrRateLimited)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: token endpoint rejected client credentials", ErrAuthRevoked)
	default:
		return fmt.Errorf("token endpoint HTTP %d: %s", status, truncateBody(body))
	}
}

// scopeIncludes returns true if the space-separated scope string
// `granted` includes `want` as a whole token. Per RFC 6749 §3.3 the
// scope value is space-delimited.
func scopeIncludes(granted, want string) bool {
	for _, s := range strings.Fields(granted) {
		if s == want {
			return true
		}
	}
	return false
}

// --- PKCE helpers (used by Phase 6 callback handler) ----------------

// pkceVerifierBytes is the entropy length for a PKCE code_verifier.
// RFC 7636 §4.1 mandates 43-128 chars after base64url; 64 bytes maps
// to a 86-char unpadded base64url string, well inside the spec.
const pkceVerifierBytes = 64

// stateTokenBytes is the entropy length for an OAuth state token.
// Plan calls for "≥256 random bits"; 32 bytes = 256 bits.
const stateTokenBytes = 32

// GeneratePKCEVerifier returns a cryptographically random PKCE
// code_verifier per RFC 7636 §4.1 (unpadded base64url, 43-128 chars).
// crypto/rand failure is exceptional but real (entropy starvation on
// container startup) — surface as error rather than silently produce
// a deterministic verifier.
func GeneratePKCEVerifier() (string, error) {
	var entropy [pkceVerifierBytes]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(entropy[:]), nil
}

// PKCEChallengeS256 returns the base64url-encoded SHA-256 of a PKCE
// verifier — the value sent as `code_challenge` with
// `code_challenge_method=S256` (RFC 7636 §4.2). The wiki only supports
// S256 (the `plain` method is mandatory-to-reject per RFC 9700 §2.1.1).
func PKCEChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateStateToken returns a cryptographically random OAuth state
// token (≥256 bits, base64url-encoded). The Phase 6 handler keys its
// single-use state-token store by this value.
func GenerateStateToken() (string, error) {
	var entropy [stateTokenBytes]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(entropy[:]), nil
}

// BuildAuthURL constructs the authorization-endpoint redirect URL with
// PKCE S256 baked in. Phase 6 uses this when the user clicks
// "Connect Google Tasks." authURL is normally DefaultGoogleAuthURL;
// tests can inject a stand-in.
//
// The five inputs all come from per-flow state (state, codeChallenge)
// or operator config (clientID, redirectURI). scope is exported as a
// parameter rather than baked-in so the same helper can be reused for
// other Google scopes when adding the Calendar/Drive bridges (deeply
// hypothetical, but cheap to leave open).
func BuildAuthURL(authURL, clientID, redirectURI, scope, state, codeChallenge string) (string, error) {
	if authURL == "" {
		return "", errors.New("tasks: authURL must not be empty")
	}
	if clientID == "" {
		return "", errors.New("tasks: clientID must not be empty")
	}
	if redirectURI == "" {
		return "", errors.New("tasks: redirectURI must not be empty")
	}
	if scope == "" {
		return "", errors.New("tasks: scope must not be empty")
	}
	if state == "" {
		return "", errors.New("tasks: state must not be empty")
	}
	if codeChallenge == "" {
		return "", errors.New("tasks: codeChallenge must not be empty")
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		return "", fmt.Errorf("parse authURL: %w", err)
	}
	q := parsed.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scope)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("access_type", "offline") // request a refresh token
	q.Set("prompt", "consent")      // force the consent screen so refresh_token is reissued
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// ValidateIssuer enforces the RFC 9207 `iss` callback parameter against
// the configured authorization-server issuer. Pinned at the gateway
// layer so adding iCloud/Microsoft is a config change rather than a
// security retrofit (per plan OAuth security profile).
func ValidateIssuer(got, want string) error {
	if got == "" {
		return fmt.Errorf("%w: callback missing iss parameter", ErrIssuerMismatch)
	}
	if got != want {
		return fmt.Errorf("%w: got %q want %q", ErrIssuerMismatch, got, want)
	}
	return nil
}

// truncateBody bounds an HTTP body to 4 KB so a chatty payload doesn't
// blow out journalctl lines. Mirrors the Keep gateway's helper but
// inlined here to avoid an import cycle / cross-package dependency.
func truncateBody(b []byte) string {
	const maxLen = 4096
	if len(b) > maxLen {
		b = b[:maxLen]
	}
	return strings.Map(func(r rune) rune {
		if r >= 0x20 && r < 0x7f {
			return r
		}
		if r == '\n' || r == '\t' {
			return ' '
		}
		return '?'
	}, string(b))
}
