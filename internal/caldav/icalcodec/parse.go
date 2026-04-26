package icalcodec

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
)

// DescriptionMaxBytes is the per-item DESCRIPTION cap from the plan
// (#983 §"iCalendar mapping summary"). Inbound VTODOs whose DESCRIPTION
// exceeds this size are rejected with ErrDescriptionTooLarge so the
// HTTP layer can surface a 413 Payload Too Large.
const DescriptionMaxBytes = 64 * 1024

// Sentinel errors so the CalDAV PUT handler can distinguish parse
// failures and map each to the appropriate HTTP status without
// pattern-matching on error strings.
var (
	// ErrNoVTODO is returned when the VCALENDAR body decoded cleanly
	// but contained no VTODO child component.
	ErrNoVTODO = errors.New("icalcodec: VCALENDAR body has no VTODO")

	// ErrMultipleVTODOs is returned when the VCALENDAR body contained
	// more than one VTODO. The CalDAV "<uid>.ics" resource model is
	// strictly one VTODO per resource.
	ErrMultipleVTODOs = errors.New("icalcodec: VCALENDAR body has more than one VTODO")

	// ErrMissingUID is returned when the VTODO has no UID property.
	// UID is mandatory per RFC 5545 and the wiki uses it as the
	// checklist item key.
	ErrMissingUID = errors.New("icalcodec: VTODO missing UID")

	// ErrDescriptionTooLarge is returned when the VTODO's DESCRIPTION
	// exceeds DescriptionMaxBytes. The HTTP layer maps this to 413.
	ErrDescriptionTooLarge = errors.New("icalcodec: DESCRIPTION exceeds 64 KB cap")
)

// ParsedVTODO is the wiki-shaped projection of an inbound VTODO. The
// CalDAV PUT handler converts this into UpsertFromCalDAVArgs before
// calling the mutator.
type ParsedVTODO struct {
	// UID is the VTODO/UID property (required).
	UID string

	// Text is the VTODO/SUMMARY property. May be empty if the client
	// sent SUMMARY but it was blank.
	Text string

	// Checked is true when STATUS == "COMPLETED".
	Checked bool

	// CompletedAt is the VTODO/COMPLETED timestamp, when present.
	CompletedAt *time.Time

	// Tags is the union of CATEGORIES and inline `#tag` references in
	// DESCRIPTION, normalized via hashtags.Normalize and deduplicated
	// in first-seen order.
	Tags []string

	// SortOrder is X-APPLE-SORT-ORDER when present (preferred), else
	// PRIORITY (1..9) when present, else nil. nil means "leave the
	// existing value alone" — the mutator decides the default.
	SortOrder *int64

	// Description is the VTODO/DESCRIPTION property (may be nil when
	// absent). Capped at DescriptionMaxBytes; longer values cause
	// ParseVTODO to return ErrDescriptionTooLarge.
	Description *string

	// Due is the VTODO/DUE timestamp, when present.
	Due *time.Time

	// Created is the VTODO/CREATED timestamp, when present. Used by
	// the mutator only on the create path.
	Created *time.Time

	// AlarmPayload is the JSON-encoded alarm payload from the first
	// VALARM child, when present. Additional VALARM children are
	// ignored — the wiki's data model carries one alarm per item.
	// nil when no VALARM is present.
	AlarmPayload *string
}

// statusCompleted is the case-insensitive string we recognize as the
// "checked" state on STATUS. It mirrors statusValueCompleted in
// render.go but is kept distinct here to make Parse's matching
// behavior (case-insensitive) self-evident at the call site.
const statusCompleted = "COMPLETED"

// sortOrderBitSize is the strconv.ParseInt bit-size for numeric
// sort-order values. The on-the-wire form is a decimal integer
// (sortOrderBase10 from render.go) bounded by int64.
const sortOrderBitSize = 64

// ParseVTODO decodes a VCALENDAR body containing a single VTODO and
// returns a ParsedVTODO ready for the mutator. Out-of-scope properties
// (RRULE, RECURRENCE-ID, RELATED-TO, GEO, LOCATION, ORGANIZER,
// ATTENDEE, CLASS, RESOURCES, X-* other than X-APPLE-SORT-ORDER) are
// stripped silently per the v1 scope guardrails. DESCRIPTION longer
// than DescriptionMaxBytes returns ErrDescriptionTooLarge so the HTTP
// layer can map it to 413 Payload Too Large.
func ParseVTODO(body []byte) (ParsedVTODO, error) {
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return ParsedVTODO{}, err
	}

	todo, err := singleVTODO(cal)
	if err != nil {
		return ParsedVTODO{}, err
	}

	return parseVTODOComponent(todo)
}

// singleVTODO returns the one and only VTODO child from a VCALENDAR,
// or a sentinel error for the zero-or-many cases. Non-VTODO children
// (e.g. VTIMEZONE) are ignored — they're commonly present alongside a
// task and not interesting to the parser.
func singleVTODO(cal *ical.Calendar) (*ical.Component, error) {
	var found *ical.Component
	for _, child := range cal.Children {
		if child.Name != ical.CompToDo {
			continue
		}
		if found != nil {
			return nil, ErrMultipleVTODOs
		}
		found = child
	}
	if found == nil {
		return nil, ErrNoVTODO
	}
	return found, nil
}

// parseVTODOComponent maps a single VTODO into the wiki shape. Split
// out so future callers (e.g., a calendar-data property in PROPFIND)
// can reuse the mapping without going through bytes.
func parseVTODOComponent(todo *ical.Component) (ParsedVTODO, error) {
	uid, err := todo.Props.Text(ical.PropUID)
	if err != nil {
		return ParsedVTODO{}, err
	}
	if uid == "" {
		return ParsedVTODO{}, ErrMissingUID
	}

	parsed := ParsedVTODO{UID: uid}

	if parsed.Text, err = todo.Props.Text(ical.PropSummary); err != nil {
		return ParsedVTODO{}, err
	}

	parsed.Checked = parseChecked(todo)

	if parsed.CompletedAt, err = parseOptionalDateTime(todo, ical.PropCompleted); err != nil {
		return ParsedVTODO{}, err
	}
	if parsed.Due, err = parseOptionalDateTime(todo, ical.PropDue); err != nil {
		return ParsedVTODO{}, err
	}
	if parsed.Created, err = parseOptionalDateTime(todo, ical.PropCreated); err != nil {
		return ParsedVTODO{}, err
	}

	description, err := parseDescription(todo)
	if err != nil {
		return ParsedVTODO{}, err
	}
	parsed.Description = description

	parsed.Tags = collectTags(todo, description)
	parsed.SortOrder = parseSortOrder(todo)
	parsed.AlarmPayload = parseFirstAlarm(todo)

	return parsed, nil
}

// parseChecked reads STATUS and reports whether it equals "COMPLETED"
// (case-insensitively). Anything else — NEEDS-ACTION, IN-PROCESS,
// CANCELLED, or absent — counts as unchecked.
func parseChecked(todo *ical.Component) bool {
	status, _ := todo.Props.Text(ical.PropStatus)
	return strings.EqualFold(strings.TrimSpace(status), statusCompleted)
}

// parseOptionalDateTime returns a pointer to the parsed datetime when
// the named property is present, or nil when it's absent. Decoding
// errors propagate.
func parseOptionalDateTime(todo *ical.Component, name string) (*time.Time, error) {
	prop := todo.Props.Get(name)
	if prop == nil {
		return nil, nil
	}
	t, err := prop.DateTime(time.UTC)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}

// parseDescription returns a pointer to the DESCRIPTION text when
// present, or nil when absent. Returns ErrDescriptionTooLarge when the
// unescaped text exceeds DescriptionMaxBytes — measuring the decoded
// length (not the raw wire value) means a client typing N characters
// of plain text gets a consistent limit regardless of how many of
// those characters are RFC 5545 escapes (`\n`, `\,`, `\;`, `\\`).
func parseDescription(todo *ical.Component) (*string, error) {
	prop := todo.Props.Get(ical.PropDescription)
	if prop == nil {
		return nil, nil
	}
	text, err := prop.Text()
	if err != nil {
		return nil, err
	}
	if len(text) > DescriptionMaxBytes {
		return nil, ErrDescriptionTooLarge
	}
	return &text, nil
}

// collectTags unions CATEGORIES tags with #tag references from
// DESCRIPTION, normalizing each through hashtags.Normalize and
// preserving first-seen order for determinism. CATEGORIES is processed
// first because clients (e.g., DAVx5 with tasks.org) treat it as the
// authoritative tag list.
func collectTags(todo *ical.Component, description *string) []string {
	seen := make(map[string]struct{})
	var tags []string

	addTag := func(raw string) {
		normalized := hashtags.Normalize(raw)
		if normalized == "" {
			return
		}
		if _, dup := seen[normalized]; dup {
			return
		}
		seen[normalized] = struct{}{}
		tags = append(tags, normalized)
	}

	if catProp := todo.Props.Get(ical.PropCategories); catProp != nil {
		// TextList parses RFC 5545 escaping and splits on unescaped
		// commas — the correct shape for CATEGORIES. If it errors,
		// fall back to a simple comma split rather than dropping
		// tags entirely.
		list, err := catProp.TextList()
		if err != nil {
			list = strings.Split(catProp.Value, ",")
		}
		for _, raw := range list {
			addTag(strings.TrimSpace(raw))
		}
	}

	if description != nil {
		for _, raw := range hashtags.Extract(*description) {
			addTag(raw)
		}
	}

	return tags
}

// parseSortOrder prefers X-APPLE-SORT-ORDER and falls back to PRIORITY.
// Returns nil when neither is parseable as int64 — the mutator treats
// that as "leave the stored sort order alone".
func parseSortOrder(todo *ical.Component) *int64 {
	if prop := todo.Props.Get(xAppleSortOrder); prop != nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(prop.Value), sortOrderBase10, sortOrderBitSize); err == nil {
			return &v
		}
	}
	if prop := todo.Props.Get(ical.PropPriority); prop != nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(prop.Value), sortOrderBase10, sortOrderBitSize); err == nil {
			return &v
		}
	}
	return nil
}

// parseFirstAlarm returns the JSON-encoded alarm payload for the first
// VALARM child of todo, or nil when no VALARM is present or the first
// one fails to parse. Additional VALARM children are ignored: the
// wiki's data model carries one alarm per item, and silently dropping
// extras is preferable to rejecting an otherwise-valid PUT.
func parseFirstAlarm(todo *ical.Component) *string {
	for _, child := range todo.Children {
		if child.Name != ical.CompAlarm {
			continue
		}
		payload, err := ParseAlarm(child)
		if err != nil {
			return nil
		}
		return &payload
	}
	return nil
}
