// Package server — OAuth error/success page rendering.
//
// The OAuth callback handler must NEVER fall back to Go's default
// http.Error (a plain text/plain response) on the user-visible path:
// the user landed here from a Google consent screen and a wall-of-text
// error is jarring and unhelpful. This file renders a small HTML page
// that reuses the wiki's stylesheet and offers two clear actions:
// "Try again" (re-launch the OAuth flow with a fresh state token) and
// "Cancel" (return to the user's profile page).
package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
)

// oauthErrorPageTitle is the title bar text for the error page.
const oauthErrorPageTitle = "Google Tasks connection failed"

// oauthSuccessRedirectPath is where successful callbacks redirect.
const oauthSuccessRedirectPath = "/profile"

// oauthErrorTemplate is the HTML emitted by the error page. Inline
// template (not in static/templates) because it's only used by the
// callback handler and we want it self-contained for security review:
// the page must not load arbitrary JavaScript that could exfiltrate
// the URL fragment containing OAuth artifacts.
//
// The page intentionally does NOT pull in the web-components bundle
// — it's a static error frame, not an interactive wiki page.
const oauthErrorTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{ .Title }}</title>
<link rel="stylesheet" href="/static/vendor/css/base-min.css">
<link rel="stylesheet" href="/static/css/default.css">
<style>
body { max-width: 640px; margin: 4rem auto; padding: 0 1rem; font-family: system-ui, sans-serif; }
.oauth-error { border: 1px solid #d33; border-radius: 6px; padding: 1.5rem; background: #fff5f5; }
.oauth-error h1 { margin-top: 0; color: #b00; }
.oauth-error pre { white-space: pre-wrap; background: #fafafa; padding: 0.75rem; border-radius: 4px; overflow-x: auto; }
.oauth-actions { margin-top: 1.5rem; display: flex; gap: 0.75rem; }
.oauth-actions a { display: inline-block; padding: 0.5rem 1rem; border-radius: 4px; text-decoration: none; }
.oauth-actions a.primary { background: #1565c0; color: #fff; }
.oauth-actions a.secondary { background: #eee; color: #333; }
@media (prefers-color-scheme: dark) {
  body { background: #1e1e1e; color: #ddd; }
  .oauth-error { background: #2a1a1a; border-color: #c33; }
  .oauth-error h1 { color: #ff6b6b; }
  .oauth-error pre { background: #111; color: #ddd; }
  .oauth-actions a.secondary { background: #333; color: #ddd; }
}
</style>
</head>
<body>
<div class="oauth-error">
<h1>{{ .Heading }}</h1>
<p>{{ .Message }}</p>
{{ if .Detail }}<pre>{{ .Detail }}</pre>{{ end }}
<div class="oauth-actions">
{{ if .RetryURL }}<a class="primary" href="{{ .RetryURL }}">Try again</a>{{ end }}
<a class="secondary" href="{{ .CancelURL }}">Cancel</a>
</div>
</div>
</body>
</html>
`

// oauthErrorPage is the data passed to the error template.
type oauthErrorPage struct {
	Title     string
	Heading   string
	Message   string
	Detail    string // optional — printed in a <pre> when present
	RetryURL  string // empty → no Try again button rendered
	CancelURL string
}

// renderOAuthErrorPage writes the wiki-styled error HTML to w with
// the given HTTP status. detail may be empty.
//
// status MUST be a real HTTP error status (4xx or 5xx). Callers MUST
// NOT use this helper for the success path — that lives in
// renderOAuthSuccess (a redirect).
func renderOAuthErrorPage(w http.ResponseWriter, status int, heading, message, detail, retryURL, cancelURL string) {
	if cancelURL == "" {
		cancelURL = oauthSuccessRedirectPath
	}
	page := oauthErrorPage{
		Title:     oauthErrorPageTitle,
		Heading:   heading,
		Message:   message,
		Detail:    detail,
		RetryURL:  retryURL,
		CancelURL: cancelURL,
	}

	tmpl, err := template.New("oauth-error").Parse(oauthErrorTemplate)
	if err != nil {
		// Should never happen — the template is a constant. Fall
		// back to plain text rather than panic so the user sees
		// *something* and the wiki doesn't 500-by-panic.
		http.Error(w, fmt.Sprintf("%s: %s", heading, message), status)
		return
	}

	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, page); execErr != nil {
		http.Error(w, fmt.Sprintf("%s: %s", heading, message), status)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// renderOAuthSuccess redirects to the post-success destination. The
// handler routes callers here once the token exchange + persistence
// have succeeded; the user lands on their profile page where the
// wiki's UI surfaces "✓ Connected to Google Tasks".
func renderOAuthSuccess(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, oauthSuccessRedirectPath, http.StatusFound)
}
