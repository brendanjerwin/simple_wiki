// Package caldav implements the wiki's CalDAV server. It exposes each
// (page, checklist) pair as an iCalendar collection and each checklist
// item as a VTODO resource, letting native task apps (Apple Reminders,
// DAVx5+tasks.org) bidirectionally sync wiki checklists.
//
// Architecture (per the plan in /home/brendanjerwin/.claude/plans/
// plan-and-build-983-vivid-puppy.md):
//
//   - CalendarBackend is the boundary between the CalDAV protocol layer
//     and the wiki's storage. The default implementation delegates
//     reads to wikipage.PageReader and writes to checklistmutator.Mutator
//     so all the funnel-level bookkeeping (sync_token, tombstones,
//     attribution) happens automatically.
//
//   - The HTTP handler dispatches OPTIONS, PROPFIND, REPORT, GET, PUT,
//     and DELETE on URLs of the form /<page>/<list>/<uid>.ics. Method
//     dispatch and most XML/iCal serialization are delegated to
//     github.com/emersion/go-webdav. Sync-collection (RFC 6578) is
//     hand-rolled because go-webdav does not expose it.
//
//   - Authentication is Tailscale-only. CalDAV handlers reject
//     IsAnonymous() identities with 403 Forbidden — no Basic-Auth
//     challenge, no realm, no 401. The Authorization header is never
//     read.
package caldav
