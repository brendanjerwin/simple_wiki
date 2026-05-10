package translator

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// TaskToChecklistItem converts a Google Tasks Task to a wiki
// ChecklistItem.
//
// Schema mapping (per plan §3, "Decisions locked"):
//
//	title    → text + tags (TitleAndTagsFromText)
//	status   → checked (completed → true; needsAction → false)
//	notes    → description (with trailing wiki:uid marker stripped)
//	due      → due (RFC3339, time-of-day comes from Google as 00:00 UTC)
//	completed → completed_at
//	position → sort_order (PositionToSortOrder; order-preserving)
//
// The wiki uid is *not* stamped here. Callers (sync orchestrator)
// resolve uid via the per-Subscription item_id_map, with marker-loss
// recovery as described in plan §"Outbound idempotence — Marker-loss
// robustness". If the marker is present, StripWikiUIDMarker exposes
// it via a separate accessor — but this function focuses purely on
// schema translation and never sets `uid` on the returned item.
//
// Backward-compat note: the strip-on-read here is preserved on
// purpose. Outbound writes no longer append a wiki:uid marker (we
// don't dump implementation detail into a user-visible field), but
// tasks created by older builds may still carry the marker in their
// notes. Stripping on read keeps those items pairing correctly until
// the next outbound tick rewrites their notes without the marker.
//
// Returns an error rather than coercing malformed input silently —
// future fields (etag, links, etc.) that surface decode-time issues
// should bubble through this signature.
func TaskToChecklistItem(task Task) (*apiv1.ChecklistItem, error) {
	cleanedNotes, _, _ := StripWikiUIDMarker(task.Notes)
	title, tags := TitleAndTagsFromText(task.Title)

	item := &apiv1.ChecklistItem{
		Text:      title,
		Checked:   task.Status == TaskStatusCompleted,
		Tags:      tags,
		SortOrder: PositionToSortOrder(task.Position),
	}
	if cleanedNotes != "" {
		d := cleanedNotes
		item.Description = &d
	}
	if !task.Due.IsZero() {
		item.Due = timestamppb.New(task.Due)
	}
	if !task.Completed.IsZero() {
		item.CompletedAt = timestamppb.New(task.Completed)
	}
	if !task.Updated.IsZero() {
		item.UpdatedAt = timestamppb.New(task.Updated)
	}
	return item, nil
}

// ChecklistItemToTaskFields converts a wiki ChecklistItem to the
// subset of Google Tasks Task fields suitable for tasks.insert or
// tasks.patch.
//
// Schema mapping is the inverse of TaskToChecklistItem with one
// asymmetry called out:
//
//   - Tasks's `due` field accepts an RFC3339 timestamp but the API
//     drops the time-of-day on the server side (it stores date-only
//     and returns midnight UTC on subsequent fetches). The wiki's
//     time-of-day on `due` is therefore lost on push. Documented in
//     plan §3 ("due↔due (date-only on Google's side; wiki time-of-
//     day dropped on push)"). The translator passes the time
//     through unchanged; the gateway layer is responsible for the
//     RFC3339 serialization.
//
//   - The wiki uid is NOT appended to `notes`. Earlier builds stamped
//     a "wiki:uid=…" marker into the notes field as the wiki↔Tasks
//     identity binding, but `notes` is user-visible (it surfaces as
//     "Details" in the Google Tasks UI), and dumping internal
//     identifiers into a user-facing field is the wrong layering. The
//     wiki↔Tasks binding lives on the Subscription's ItemIDMap. Tasks
//     created by older builds may still carry the marker in their
//     notes; TaskToChecklistItem strips it on read for backward
//     compat, and the next outbound tick that touches such an item
//     will rewrite its notes without the marker (one-time migration).
//
//   - When `checked` flips false→true the wiki's stamped completed_at
//     is sent as Tasks's `completed`. When false (or unset), both
//     status=needsAction and a zero `completed` are sent — the
//     gateway is responsible for translating zero-time into the
//     Tasks API's "clear this field" semantics on patch.
func ChecklistItemToTaskFields(item *apiv1.ChecklistItem) TaskFields {
	if item == nil {
		return TaskFields{}
	}
	title := EncodeTitleWithTags(item.GetText(), item.GetTags())
	notes := item.GetDescription()

	fields := TaskFields{
		Title: title,
		Notes: notes,
	}
	if item.GetChecked() {
		fields.Status = TaskStatusCompleted
		if item.GetCompletedAt() != nil {
			fields.Completed = item.GetCompletedAt().AsTime()
		}
	} else {
		fields.Status = TaskStatusNeedsAction
	}
	if item.GetDue() != nil {
		fields.Due = item.GetDue().AsTime()
	}
	return fields
}
