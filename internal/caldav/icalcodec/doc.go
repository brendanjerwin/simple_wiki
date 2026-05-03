// Package icalcodec converts wiki checklist items to and from iCalendar
// VTODO components for the CalDAV bridge.
//
// RenderItem produces a single-VTODO VCALENDAR for a ChecklistItem, used
// by the CalDAV server to satisfy GET, PROPFIND calendar-data, and REPORT
// calendar-multiget responses.
//
// ParseVTODO (added in #983 Phase 2) reads inbound VCALENDAR bodies from
// CalDAV PUT requests, normalizes tags via the wiki's hashtag rules, and
// strips out-of-scope properties (RRULE, RELATED-TO, GEO, etc.) silently
// per the v1 scope guardrails in the plan.
package icalcodec
