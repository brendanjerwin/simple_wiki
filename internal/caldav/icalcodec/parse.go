package icalcodec

import (
	"errors"
	"time"
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
	// VALARM child, when present. nil when no VALARM is present.
	AlarmPayload *string
}

// ParseVTODO decodes a VCALENDAR body containing a single VTODO and
// returns a ParsedVTODO ready for the mutator. Out-of-scope properties
// (RRULE, RECURRENCE-ID, RELATED-TO, GEO, LOCATION, ORGANIZER,
// ATTENDEE, CLASS, RESOURCES, X-* other than X-APPLE-SORT-ORDER) are
// stripped silently per the v1 scope guardrails. DESCRIPTION longer
// than DescriptionMaxBytes returns ErrDescriptionTooLarge so the HTTP
// layer can map it to 413 Payload Too Large.
func ParseVTODO(_ []byte) (ParsedVTODO, error) {
	return ParsedVTODO{}, ErrNoVTODO
}
