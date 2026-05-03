// Package server — OAuth callback handler for Google.
//
// This file implements the /oauth/google/callback endpoint. The flow:
//
//  1. RFC 9207 `iss` validation against the pinned Google issuer URL.
//     The `iss` parameter is REQUIRED — if absent OR mismatched, the
//     callback is rejected. The plan ("OAuth security profile") is
//     unqualified: validate iss against the configured authorization
//     server's issuer. Strict (not pragmatic) is chosen deliberately
//     so the security posture matches the documented stance from day
//     one — if Google omits iss on a real flow we'd rather find out
//     immediately than carry a tolerated-absence loophole forever.
//  2. State token validation via the single-use server-side store.
//     The store enforces ≥256 bits of entropy, single-use, 10-min TTL.
//  3. **Validation order matters**: `iss` and `state` are validated
//     BEFORE the authorization-code exchange. RFC 9700 §2.1.2 — never
//     send a code to the token endpoint until you've confirmed the
//     callback came from the IdP you initiated against.
//  4. Authorization-code → tokens exchange with PKCE S256 verifier.
//  5. RFC 6749 §3.3 scope-echo check: response.scope MUST include
//     https://www.googleapis.com/auth/tasks. Google may downgrade
//     scopes silently; this is the only signal we get.
//  6. Persist the refresh token via the injected RefreshTokenPersister
//     interface (Phase 7 wires the real implementation).
//
// The handler renders wiki-styled error pages (oauth_error_page.go)
// rather than fall back to Go's default plaintext http.Error — the
// user is mid-OAuth and a wall-of-text error is jarring.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Pinned issuer for Google per RFC 9207. Per the plan ("OAuth security
// profile"): bake iss validation in from day 1 even with one IdP, so
// adding iCloud/Microsoft is a config change rather than a security
// retrofit.
const googleOAuthIssuer = "https://accounts.google.com"

// Google's OAuth 2.0 token endpoint. Per
// https://accounts.google.com/.well-known/openid-configuration this
// is the canonical token URL.
const googleTokenEndpoint = "https://oauth2.googleapis.com/token"

// googleTasksScope is the scope the wiki's Tasks connector requires.
// `tasks.readonly` does not satisfy `tasks.insert/patch/delete` — we
// need round-trip access (per the plan's "OAuth security profile").
const googleTasksScope = "https://www.googleapis.com/auth/tasks"

// Environment variables for the Google Tasks OAuth client. Per the
// plan, operators set these per-deployment; if any are unset, the
// handler returns a 503 telling the user the operator hasn't enabled
// the connector.
const (
	envGoogleTasksClientID     = "SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID"
	envGoogleTasksClientSecret = "SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET"
	envGoogleTasksRedirectURI  = "SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI"
)

// tokenExchangeTimeout caps the HTTP call to Google's token endpoint.
// Generous enough for a slow handshake but short enough that a hung
// callback doesn't stall a worker forever.
const tokenExchangeTimeout = 15 * time.Second

// RefreshTokenPersister is the seam Phase 7 will fill with the real
// SubscriptionStore wiring. Phase 6 only needs to know that a
// successful exchange triggers a single Persist call.
//
// The interface is defined locally to avoid a dependency on the
// connectors package (per the worktree-isolation rules — Phase 6
// owns the handler files and nothing in internal/connectors/).
type RefreshTokenPersister interface {
	PersistRefreshToken(ctx context.Context, profileID, accountEmail, refreshToken string) error
}

// AuthRedirectIssuer abstracts "build me a fresh authorization URL
// for this profile" so the handler can render a "Try again" button
// that re-initiates the flow with a brand-new state + verifier.
//
// Phase 7/8 will wire this to the connector service that drives
// BeginAuth. For Phase 6 the handler only needs to call it on the
// error path — production wiring happens later.
type AuthRedirectIssuer interface {
	BuildAuthURL(ctx context.Context, profileID string) (string, error)
}

// HTTPDoer abstracts the HTTP client so tests can inject httptest
// fakes for Google's token endpoint without spinning up a real DNS
// lookup.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ProfileIdentityResolver maps the current request's Tailscale identity
// to a profile ID. Phase 7 wires this to the existing
// wikipage.ProfileIdentifierFor logic; Phase 6 takes it as a seam so
// callback tests can run without a Tailscale agent.
type ProfileIdentityResolver interface {
	ResolveProfileID(r *http.Request) (string, error)
}

// OAuthGoogleHandler bundles the dependencies the callback endpoint
// needs. Each dependency is a separate field rather than a "Config"
// struct because they have unrelated owners (per CLAUDE.md "Avoid
// Parameter Objects and Config Structs").
type OAuthGoogleHandler struct {
	StateStore        OAuthStateStore
	TokenPersister    RefreshTokenPersister
	IdentityResolver  ProfileIdentityResolver
	AuthURLIssuer     AuthRedirectIssuer
	HTTPClient        HTTPDoer
	ClientID          string
	ClientSecret      string
	RedirectURI       string
	IssuerExpected    string
	TokenEndpoint     string
	RequiredScope     string
	Logger            *log.Logger
}

// NewOAuthGoogleHandler reads the OAuth client config from the
// environment and wires the handler. Returns nil + error if any
// required env var is unset — the caller (route registration) renders
// the "not configured" page on demand instead of failing startup.
func NewOAuthGoogleHandler(
	stateStore OAuthStateStore,
	persister RefreshTokenPersister,
	identityResolver ProfileIdentityResolver,
	authURLIssuer AuthRedirectIssuer,
) (*OAuthGoogleHandler, error) {
	clientID := os.Getenv(envGoogleTasksClientID)
	clientSecret := os.Getenv(envGoogleTasksClientSecret)
	redirectURI := os.Getenv(envGoogleTasksRedirectURI)

	if clientID == "" || clientSecret == "" || redirectURI == "" {
		return nil, errNotConfigured
	}

	return &OAuthGoogleHandler{
		StateStore:       stateStore,
		TokenPersister:   persister,
		IdentityResolver: identityResolver,
		AuthURLIssuer:    authURLIssuer,
		HTTPClient:       &http.Client{Timeout: tokenExchangeTimeout},
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		RedirectURI:      redirectURI,
		IssuerExpected:   googleOAuthIssuer,
		TokenEndpoint:    googleTokenEndpoint,
		RequiredScope:    googleTasksScope,
		Logger:           log.Default(),
	}, nil
}

// errNotConfigured is the sentinel returned by the constructor when
// the operator hasn't set the required env vars.
var errNotConfigured = errors.New("oauth google: not configured")

// IsNotConfigured reports whether err came from a missing OAuth env
// var. Route registration uses this to decide whether to mount the
// "real" handler or a 503-rendering placeholder.
func IsNotConfigured(err error) bool {
	return errors.Is(err, errNotConfigured)
}

// HandleCallback is the gin.HandlerFunc registered at
// /oauth/google/callback. It performs the validation + exchange
// described at the top of this file.
//
// VALIDATION ORDER (do not reorder):
//  1. iss (RFC 9207)  — before anything else: confirms which IdP
//     the redirect came from.
//  2. state (RFC 6749 §10.12) — confirms which browser session
//     this callback belongs to AND retrieves the PKCE verifier.
//  3. code exchange — only after (1) and (2) pass.
func (h *OAuthGoogleHandler) HandleCallback(c *gin.Context) {
	h.serveCallback(c.Writer, c.Request)
}

// serveCallback is the testable core that takes net/http types
// directly so unit tests don't need a Gin context.
func (h *OAuthGoogleHandler) serveCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if errParam := q.Get("error"); errParam != "" {
		// User denied consent or Google rejected something. The
		// `error_description` (if present) is human-readable per
		// RFC 6749 §4.1.2.1.
		desc := q.Get("error_description")
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"Authorization was not granted",
			"Google reported: "+errParam,
			desc, retry, oauthSuccessRedirectPath)
		return
	}

	// Step 1: iss validation (RFC 9207). The plan's "OAuth security
	// profile" specifies validating iss against the configured
	// authorization server's issuer with no qualification, so we
	// require its presence — absent OR mismatched both reject. If
	// this turns out to break a legitimate Google OAuth flow in
	// practice (some non-OIDC variants reportedly omit iss), the
	// resolution is to relax the policy here AFTER discovering it,
	// not to pre-emptively tolerate the gap.
	iss := q.Get("iss")
	if iss == "" {
		h.logf("oauth google callback: iss param absent (rejected per RFC 9207)")
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"Authorization server identification missing",
			"The OAuth response did not include the issuer parameter required for RFC 9207 verification. This usually means the OAuth response was tampered with or did not originate from Google's sign-in service.",
			"", "", oauthSuccessRedirectPath)
		return
	}
	if iss != h.IssuerExpected {
		h.logf("oauth google callback: iss mismatch: got %q want %q", iss, h.IssuerExpected)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"Authorization server mismatch",
			"The OAuth response did not come from the expected Google sign-in service. This usually means the callback URL or the wiki's OAuth client is misconfigured.",
			"", "", oauthSuccessRedirectPath)
		return
	}

	// Step 2: state validation. Single-use, server-side keyed by the
	// state value; the entry carries the PKCE code_verifier we need
	// for the exchange.
	state := q.Get("state")
	entry, err := h.StateStore.Consume(r.Context(), state)
	if err != nil {
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"This sign-in session expired",
			"The session token that links your browser to Google has expired or was already used. Click \"Try again\" to start a fresh sign-in.",
			"", retry, oauthSuccessRedirectPath)
		return
	}

	// Step 3: extract code AFTER state has cleared, so a missing
	// code on a valid state still tears down the entry (preventing
	// state-replay).
	code := q.Get("code")
	if code == "" {
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"Authorization code missing",
			"Google's response did not include an authorization code. Try the sign-in again.",
			"", retry, oauthSuccessRedirectPath)
		return
	}

	// Steps 4–6: exchange code for tokens, validate scope, persist.
	h.completeExchange(w, r, entry, code)
}

// completeExchange performs the PKCE code exchange (step 4), scope
// validation (step 5), and refresh-token persistence (step 6). It
// renders an error page and returns on any failure; on success it calls
// renderOAuthSuccess.
func (h *OAuthGoogleHandler) completeExchange(w http.ResponseWriter, r *http.Request, entry OAuthStateEntry, code string) {
	// Step 4: exchange code → tokens (PKCE).
	tokens, err := h.exchangeCodeForTokens(r.Context(), code, entry.CodeVerifier)
	if err != nil {
		h.logf("oauth google callback: token exchange failed: %v", err)
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadGateway,
			"Could not complete sign-in with Google",
			"Google's token endpoint rejected the authorization. This can happen if the wiki's OAuth client secret is wrong, the redirect URI doesn't match exactly, or the authorization code expired.",
			err.Error(), retry, oauthSuccessRedirectPath)
		return
	}

	// Step 5: scope-echo hygiene (RFC 6749 §3.3). Google may
	// downgrade scopes silently; refuse to persist a token that
	// can't do what we asked.
	if !scopeIncludes(tokens.Scope, h.RequiredScope) {
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusForbidden,
			"Tasks scope was not granted",
			"Google issued a token that does not include the Google Tasks scope. Check the GCP consent screen configuration — the Tasks API scope must be enabled. Granted scopes: "+tokens.Scope,
			"", retry, oauthSuccessRedirectPath)
		return
	}

	// Step 6: persist the refresh token. We do not need the access
	// token here — the gateway calls /token with grant_type=refresh
	// at use-time.
	if tokens.RefreshToken == "" {
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"No refresh token was issued",
			"Google did not return a refresh token. This usually means the user has already authorized this OAuth client and Google declined to re-issue one. Revoke the wiki at https://myaccount.google.com/permissions and try again.",
			"", retry, oauthSuccessRedirectPath)
		return
	}

	// Extract the verified email from the id_token claims so the
	// account email lands on the profile alongside the refresh token.
	// Without this the connector's downstream identity-attribution
	// path falls through to the generic "system:connector-sync"
	// fallback, which makes Tasks-side writes look like Keep-side
	// writes in the wiki's checklist UI. The id_token signature is
	// not verified here — Google's token endpoint is mTLS-equivalent
	// (HTTPS to a known issuer), so we trust claims for attribution
	// purposes only. Authn decisions still rely on the refresh-token
	// round-trip, never on these claims.
	//
	// We treat email-extraction failure as a hard fail of the OAuth
	// flow rather than persist an empty string. Empty Email
	// previously produced the "Connected as ." display bug and broke
	// identity attribution. Per feedback_function_contract_purity:
	// the function does what it says or it errors — no third state.
	accountEmail, emailErr := emailFromIDToken(tokens.IDToken)
	if emailErr != nil {
		h.logf("oauth google callback: email extraction failed: %v", emailErr)
		retry := h.tryBuildAuthURL(r)
		renderOAuthErrorPage(w, http.StatusBadRequest,
			"Could not read your Google account email",
			"Google's response didn't include a verified email claim. The wiki needs this to label your connection and attribute changes to you. This usually means the OAuth client's consent screen is missing the openid/email scopes — verify the GCP configuration and try again.",
			emailErr.Error(), retry, oauthSuccessRedirectPath)
		return
	}

	if persistErr := h.TokenPersister.PersistRefreshToken(
		r.Context(), entry.ProfileID, accountEmail, tokens.RefreshToken,
	); persistErr != nil {
		h.logf("oauth google callback: persist failed: %v", persistErr)
		renderOAuthErrorPage(w, http.StatusInternalServerError,
			"Could not save your Google connection",
			"The wiki accepted Google's response but failed to persist the refresh token to your profile. The wiki's logs have details.",
			persistErr.Error(), "", oauthSuccessRedirectPath)
		return
	}

	renderOAuthSuccess(w, r)
}

// tokenResponse is the subset of Google's token-endpoint response we
// consume. Field names match the wire (snake_case) per RFC 6749 §5.1.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
}

// tokenErrorResponse is the RFC 6749 §5.2 error shape.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// exchangeCodeForTokens performs the PKCE-S256 authorization-code
// exchange against Google's token endpoint.
func (h *OAuthGoogleHandler) exchangeCodeForTokens(ctx context.Context, code, codeVerifier string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.RedirectURI)
	form.Set("client_id", h.ClientID)
	form.Set("client_secret", h.ClientSecret)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var oerr tokenErrorResponse
		if jsonErr := json.Unmarshal(body, &oerr); jsonErr == nil && oerr.Error != "" {
			return nil, fmt.Errorf("token endpoint %d: %s: %s", resp.StatusCode, oerr.Error, oerr.ErrorDescription)
		}
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(body))
	}

	var tokens tokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokens, nil
}

// scopeIncludes reports whether the space-separated scope string
// granted by Google contains the required scope. Per RFC 6749 §3.3
// scopes are space-delimited; an exact-match check on each token is
// sufficient.
func scopeIncludes(granted, required string) bool {
	if granted == "" || required == "" {
		return false
	}
	for _, tok := range strings.Fields(granted) {
		if tok == required {
			return true
		}
	}
	return false
}

// tryBuildAuthURL asks the AuthRedirectIssuer for a fresh auth URL
// to present in the "Try again" button. Returns "" on any error so
// the caller renders the page without the button rather than 500.
func (h *OAuthGoogleHandler) tryBuildAuthURL(r *http.Request) string {
	if h.AuthURLIssuer == nil || h.IdentityResolver == nil {
		return ""
	}
	profileID, err := h.IdentityResolver.ResolveProfileID(r)
	if err != nil || profileID == "" {
		return ""
	}
	authURL, err := h.AuthURLIssuer.BuildAuthURL(r.Context(), profileID)
	if err != nil {
		return ""
	}
	return authURL
}

func (h *OAuthGoogleHandler) logf(format string, args ...any) {
	if h.Logger == nil {
		return
	}
	h.Logger.Printf(format, args...)
}

// HandleNotConfiguredCallback is registered when the env vars are
// unset. It always renders a 503 explaining that the operator hasn't
// enabled the Google Tasks connector.
func HandleNotConfiguredCallback(c *gin.Context) {
	renderOAuthErrorPage(c.Writer, http.StatusServiceUnavailable,
		"Google Tasks is not configured",
		"This wiki's operator has not enabled the Google Tasks integration. See the setup docs for the required environment variables.",
		"", "", oauthSuccessRedirectPath)
}

// activeOAuthGoogleHandler holds the Phase-7-supplied callback handler.
// Phase 6 owns the route registration but cannot construct the real
// handler — that requires SubscriptionStore + Tailscale identity wiring
// from Phase 7. Bootstrap calls SetOAuthGoogleHandler after the
// connector subsystem is built; until then, callbacks render the
// "not configured" page.
//
// A package-level pointer (rather than a field on *Site) keeps Phase 6
// confined to its owned files — adding a field to Site means touching
// site.go, which is shared with parallel agents.
var (
	activeOAuthGoogleHandlerMu sync.Mutex
	activeOAuthGoogleHandler   *OAuthGoogleHandler
)

// SetOAuthGoogleHandler installs the live callback handler. Phase 7
// bootstrap calls this once after constructing the persister + identity
// resolver. Passing nil reverts to the not-configured response (used
// in tests that want the route present but inert).
func SetOAuthGoogleHandler(h *OAuthGoogleHandler) {
	activeOAuthGoogleHandlerMu.Lock()
	defer activeOAuthGoogleHandlerMu.Unlock()
	activeOAuthGoogleHandler = h
}

// getOAuthGoogleHandler is the read side. Returns nil if SetOAuthGoogleHandler
// has not been called yet — the route handler renders the not-configured
// page in that case.
func getOAuthGoogleHandler() *OAuthGoogleHandler {
	activeOAuthGoogleHandlerMu.Lock()
	defer activeOAuthGoogleHandlerMu.Unlock()
	return activeOAuthGoogleHandler
}

// handleOAuthGoogleCallback is the gin.HandlerFunc registered at
// /oauth/google/callback (see registerRoutes in handlers_web.go).
// Until Phase 7 wires a real handler, this renders the "not configured"
// 503 page so the route is discoverable while the connector stays
// inert.
func (*Site) handleOAuthGoogleCallback(c *gin.Context) {
	h := getOAuthGoogleHandler()
	if h == nil {
		HandleNotConfiguredCallback(c)
		return
	}
	h.HandleCallback(c)
}

// idTokenJWTSegmentCount is the canonical JWT segment count
// (header.payload.signature). An id_token with a different shape is
// rejected.
const idTokenJWTSegmentCount = 3

// idTokenPayloadSegmentIndex is the zero-based index of the claims
// payload within a JWT's three segments.
const idTokenPayloadSegmentIndex = 1

// ErrIDTokenMissing is returned when the token-endpoint response did
// not include an id_token at all. Almost always means the auth-URL
// scope is missing `openid` (the OIDC contract).
var ErrIDTokenMissing = errors.New("id_token absent from token response")

// ErrIDTokenMalformed is returned when the id_token isn't a parseable
// three-segment JWT (decoding the payload segment failed).
var ErrIDTokenMalformed = errors.New("id_token malformed")

// ErrIDTokenEmailMissing is returned when the id_token decodes but
// the email claim is empty or email_verified is false. Almost always
// means the auth-URL scope is missing `email`.
var ErrIDTokenEmailMissing = errors.New("id_token has no verified email claim")

// emailFromIDToken extracts the verified email claim from a Google
// OIDC id_token. Returns a typed error on any failure — the caller
// MUST surface this to the user via the wiki-styled error page. We
// previously fail-silenced to "" and persisted an empty Email, which
// produced the "Connected as ." display bug and broke downstream
// identity attribution; do not regress that pattern.
//
// The signature is intentionally NOT verified here: the id_token
// arrived over the back-channel HTTPS token-endpoint exchange
// directly from Google, so trusting claims for attribution is
// acceptable. Authentication decisions still rely on the
// refresh-token round-trip, never on these claims.
func emailFromIDToken(idToken string) (string, error) {
	if idToken == "" {
		return "", ErrIDTokenMissing
	}
	parts := strings.Split(idToken, ".")
	if len(parts) != idTokenJWTSegmentCount {
		return "", fmt.Errorf("%w: expected %d JWT segments, got %d",
			ErrIDTokenMalformed, idTokenJWTSegmentCount, len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[idTokenPayloadSegmentIndex])
	if err != nil {
		return "", fmt.Errorf("%w: payload base64-decode: %w", ErrIDTokenMalformed, err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("%w: payload JSON unmarshal: %w", ErrIDTokenMalformed, err)
	}
	if claims.Email == "" {
		return "", fmt.Errorf("%w: claim absent", ErrIDTokenEmailMissing)
	}
	if !claims.EmailVerified {
		return "", fmt.Errorf("%w: email_verified=false for %q",
			ErrIDTokenEmailMissing, claims.Email)
	}
	return claims.Email, nil
}
