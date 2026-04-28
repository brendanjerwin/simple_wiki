// Package protocol is a Go port of the reverse-engineered Google Keep wire
// protocol. See REFERENCE.md for the pinned upstream sources, the auth flow
// description, and the failure-surface mapping.
package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AuthURL is the production Google Play Services auth endpoint used for both
// Stage 1 (master login) and Stage 2 (per-service OAuth). Tests inject a
// stand-in URL.
const AuthURL = "https://android.clients.google.com/auth"

const (
	// userAgent is the exact UA Google requires; other UAs return 403.
	userAgent = "GoogleAuth/1.4"

	// playServicesClientSig is the SHA-1 of the Google Play Services 7.3.29
	// signing cert. Older sigs have been revoked server-side.
	playServicesClientSig = "38918a453d07199354f8b19af05ec6562ced5788"

	// playServicesVersion is the integer version we claim. Bumping this is
	// the first knob to try when previously-working accounts start failing.
	playServicesVersion = "240913000"

	// httpServerErrorThreshold is the lowest 5xx code; responses at or
	// above this are treated as transport-level failures rather than
	// auth-level failures.
	httpServerErrorThreshold = 500
)

// Authenticator performs Google Play Services-style auth exchanges (Stage 1
// and Stage 2 in REFERENCE.md). All HTTP requests are dispatched through the
// injected http.Client so tests can substitute a fake transport.
type Authenticator struct {
	httpClient *http.Client
	authURL    string
	deviceID   string
}

// NewAuthenticator constructs an Authenticator. authURL is the auth endpoint
// (production callers pass AuthURL); deviceID is the stable 16-hex-char
// Android device id Google sees on the wire — the wiki persists one per
// profile so subsequent calls don't trip "new device" heuristics.
func NewAuthenticator(httpClient *http.Client, authURL, deviceID string) *Authenticator {
	return &Authenticator{httpClient: httpClient, authURL: authURL, deviceID: deviceID}
}

// KeepServiceScope is the OAuth scope string we hand to Stage 2 to receive
// a bearer usable against the Keep API.
const KeepServiceScope = "oauth2:https://www.googleapis.com/auth/memento"

// keepAndroidApp is the Android package name we claim to be — required by
// Stage 2.
const keepAndroidApp = "com.google.android.keep"

// ExchangeMasterTokenForBearer performs Stage 2 of the auth flow: trade a
// long-lived master token for a short-lived bearer scoped to the Keep API.
// Bearer is returned verbatim — caller adds it as
// "Authorization: GoogleLogin auth=<bearer>" on Keep API requests.
//
// Note that, unlike Stage 1, the master token is sent verbatim in the
// EncryptedPasswd slot (Google reuses the field name). It is *not* RSA
// re-encrypted.
func (a *Authenticator) ExchangeMasterTokenForBearer(ctx context.Context, email, masterToken string) (string, error) {
	form := a.commonAuthFields(email)
	form.Set("EncryptedPasswd", masterToken)
	form.Set("service", KeepServiceScope)
	form.Set("app", keepAndroidApp)

	resp, err := a.postAuth(ctx, form)
	if err != nil {
		return "", err
	}

	if bearer := resp["Auth"]; bearer != "" {
		return bearer, nil
	}
	return "", classifyBearerError(resp)
}

// classifyBearerError maps Stage 2 errors to the appropriate sentinels.
// Differs from classifyAuthError in that BadAuthentication here means the
// master token is revoked (not a wrong-ASP situation).
func classifyBearerError(resp map[string]string) error {
	switch resp["Error"] {
	case "":
		return ErrProtocolDrift
	case "BadAuthentication", "NeedsBrowser":
		return ErrAuthRevoked
	default:
		return fmt.Errorf("%w: %s", ErrAuthRevoked, resp["Error"])
	}
}

// ExchangeASPForMasterToken performs Stage 1 of the auth flow: trade an
// App-Specific Password for a long-lived master token.
//
// The ASP is consumed only for the duration of this call. It is never
// returned, never logged, and never persisted by this function.
func (a *Authenticator) ExchangeASPForMasterToken(ctx context.Context, email, asp string) (string, error) {
	encryptedPasswd, err := encryptCredential(androidKey, email, asp)
	if err != nil {
		return "", fmt.Errorf("encrypt credential: %w", err)
	}

	form := a.commonAuthFields(email)
	form.Set("add_account", "1")
	form.Set("EncryptedPasswd", encryptedPasswd)
	form.Set("service", "ac2dm")
	form.Set("callerSig", playServicesClientSig)
	form.Set("droidguard_results", "dummy123")

	resp, err := a.postAuth(ctx, form)
	if err != nil {
		return "", err
	}

	if token := resp["Token"]; token != "" {
		return token, nil
	}
	return "", classifyAuthError(resp)
}

// commonAuthFields populates the request fields shared between Stage 1 and
// Stage 2. Callers add stage-specific fields on top.
func (a *Authenticator) commonAuthFields(email string) url.Values {
	form := url.Values{}
	form.Set("accountType", "HOSTED_OR_GOOGLE")
	form.Set("Email", email)
	form.Set("has_permission", "1")
	form.Set("source", "android")
	form.Set("androidId", a.deviceID)
	form.Set("device_country", "us")
	form.Set("operatorCountry", "us")
	form.Set("lang", "en")
	form.Set("sdk_version", "17")
	form.Set("google_play_services_version", playServicesVersion)
	form.Set("client_sig", playServicesClientSig)
	return form
}

// postAuth sends a form-urlencoded POST to the configured authURL and
// returns the parsed key=value response, or an error if the transport
// failed or the response code was clearly non-auth (e.g., 5xx with no
// auth-error body).
func (a *Authenticator) postAuth(ctx context.Context, form url.Values) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build auth request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read auth response: %w", err)
	}

	parsed := parseAuthResponse(string(body))

	// 4xx with a recognizable auth-error body is fine — the caller will
	// classify it. 5xx with no auth-error body is a transport-level
	// failure.
	if resp.StatusCode >= httpServerErrorThreshold {
		return nil, fmt.Errorf("auth: HTTP %d: %s", resp.StatusCode, parsed["Error"])
	}
	return parsed, nil
}

// parseAuthResponse parses Google's text/plain key=value response.
func parseAuthResponse(body string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		out[key] = val
	}
	return out
}

// classifyAuthError maps Google's "Error=…" responses into our typed
// sentinels. Branches are on field values, never on free-form messages
// (per CLAUDE.md "Never Branch Logic on Error Messages").
func classifyAuthError(resp map[string]string) error {
	switch resp["Error"] {
	case "BadAuthentication":
		return ErrInvalidCredentials
	case "NeedsBrowser":
		return ErrAuthRevoked
	case "":
		// No Token AND no Error — treat as drift.
		return ErrProtocolDrift
	default:
		return fmt.Errorf("%w: %s", ErrAuthRevoked, resp["Error"])
	}
}
