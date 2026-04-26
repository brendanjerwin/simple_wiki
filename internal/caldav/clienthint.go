package caldav

import (
	"context"
	"strings"
)

// ClientKind identifies which CalDAV client family a request came from.
// Used to flip on per-client outbound iCalendar variations — most
// clients can share the protocol-correct rendering, but Apple
// Reminders has documented quirks around how it surfaces tags and
// ordering, and we've found at least one case (`#tag` round-trip) where
// emitting an Apple-specific shape produces a meaningfully better UX
// without breaking RFC compliance for other clients.
type ClientKind int

const (
	// ClientUnknown is the catch-all for User-Agents we don't recognize
	// or that arrive without a header. Falls back to standards-compliant
	// rendering — no Apple-specific extensions.
	ClientUnknown ClientKind = iota

	// ClientIOS marks any of the iOS / macOS CalDAV clients that share
	// the system EventKit account framework: Apple Reminders, Apple
	// Calendar (iCal), Fantastical, Things 3, etc. They all funnel
	// through `dataaccessd` (the iOS background CalDAV daemon) or send
	// `iOS/<version>` / `macOS/<version>` strings in the User-Agent on
	// behalf of the requesting app.
	ClientIOS
)

// detectClientFromUserAgent maps a raw User-Agent header to a
// ClientKind. Lowercase substring match — User-Agents are not
// machine-readable contracts and clients change their strings between
// OS versions, so we match on stable substrings rather than parsing
// product/version pairs.
//
// Conservative on purpose: the only client family worth distinguishing
// today is iOS-family. Adding a new branch should require evidence
// from a real packet capture, not just speculation.
func detectClientFromUserAgent(ua string) ClientKind {
	if ua == "" {
		return ClientUnknown
	}
	lc := strings.ToLower(ua)
	for _, hint := range iOSUserAgentHints {
		if strings.Contains(lc, hint) {
			return ClientIOS
		}
	}
	return ClientUnknown
}

// iOSUserAgentHints is the lowercase-substring set we treat as
// evidence the request came from an iOS-family client. Captured from
// real wiki traffic against household iPhones / iPads:
//
//   - "ios/" — the OS version product token; e.g. `iOS/17.4`
//   - "macos/" — same family on macOS desktops
//   - "dataaccessd" — the iOS background CalDAV daemon that brokers
//     EventKit syncs
//   - "remindd" — older iOS Reminders sync daemon
//   - "calaccessd" — calendar access daemon
//   - "fantastical" — Flexibits' Fantastical sets a distinctive UA
//   - "iCal" / "icalendar" — older Apple-family clients
var iOSUserAgentHints = []string{
	"ios/",
	"macos/",
	"dataaccessd",
	"remindd",
	"calaccessd",
	"fantastical",
	"icalendar",
	"ical/",
}

// clientKindCtxKeyType is the unexported context-key type — using a
// dedicated type prevents collisions with any other package that
// stuffs values into the same context.
type clientKindCtxKeyType struct{}

// clientKindCtxKey is the singleton key under which we store the
// detected ClientKind on every request's context.
var clientKindCtxKey = clientKindCtxKeyType{}

// withClientKind returns a copy of ctx tagged with kind. Called once
// per request from ServeHTTP so every downstream layer (backend
// renderers, audit logs) can branch on the client family without
// re-parsing the User-Agent header.
func withClientKind(ctx context.Context, kind ClientKind) context.Context {
	return context.WithValue(ctx, clientKindCtxKey, kind)
}

// ClientKindFromContext returns the ClientKind detected for the
// current request. Returns ClientUnknown when ctx wasn't tagged (e.g.
// a unit test that called the backend directly without going through
// ServeHTTP). Exported because the backend lives in the caldav
// package but the renderer that branches on it lives in icalcodec —
// the backend reads the value here and passes it across the package
// boundary as a plain bool / enum.
func ClientKindFromContext(ctx context.Context) ClientKind {
	if ctx == nil {
		return ClientUnknown
	}
	v, ok := ctx.Value(clientKindCtxKey).(ClientKind)
	if !ok {
		return ClientUnknown
	}
	return v
}
