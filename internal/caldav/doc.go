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
//     and DELETE on URLs of the form /<page>/<list>/<uid>.ics.
//
//     Spike outcome (per Phase 1 P1-C0): we initially planned to delegate
//     method dispatch and XML serialization to github.com/emersion/go-webdav,
//     but its caldav.Backend interface is path-based with method
//     parameters that don't map cleanly onto our (page, list, uid)
//     decomposition, and it doesn't expose RFC 6578 sync-collection at
//     all — which we need. We hand-rolled the multistatus / sync-collection
//     XML against encoding/xml directly and use github.com/emersion/go-ical
//     for the iCalendar body codec only. go-webdav is not a dependency.
//
//   - Authentication is Tailscale-only. CalDAV handlers reject
//     IsAnonymous() identities with 403 Forbidden — no Basic-Auth
//     challenge, no realm, no 401. The Authorization header is never
//     read.
package caldav
