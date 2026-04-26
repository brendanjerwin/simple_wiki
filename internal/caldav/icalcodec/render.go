package icalcodec

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// RenderItem encodes a single ChecklistItem as a VCALENDAR with one
// VTODO child. The returned bytes are an iCalendar document suitable
// for serving from CalDAV GET on a `<uid>.ics` resource and for
// embedding in PROPFIND / REPORT calendar-data property responses.
//
// page and listName name the wiki page and the checklist within it;
// they're used to build the VTODO's URL property as a back-link to the
// wiki UI. baseURL is the wiki's externally-visible base URL (e.g.,
// "https://wiki.example.com") and is the prefix on the URL property.
// Trailing slashes on baseURL are tolerated.
//
// nowFn returns the wall-clock time used for the DTSTAMP property,
// which RFC 5545 mandates on every VTODO. Callers in production should
// pass time.Now; tests pass a deterministic clock.
//
// Defaults to standards-only rendering — for client-specific shapes
// (e.g. embedding tags into SUMMARY for Apple Reminders) call
// RenderItemWithOptions.
func RenderItem(item *apiv1.ChecklistItem, page string, listName string, baseURL string, nowFn func() time.Time) []byte {
	return RenderItemWithOptions(item, page, listName, baseURL, nowFn, RenderOptions{})
}

// RenderOptions controls per-call shape variations on the rendered
// VTODO. Each field is opt-in — the zero value matches the
// standards-only behavior of RenderItem.
type RenderOptions struct {
	// EmbedTagsInSummary appends `#tag` markers for every entry in
	// item.Tags to the SUMMARY text (skipping any that are already
	// present, case-insensitively, so an inbound `Buy milk #urgent`
	// doesn't round-trip as `Buy milk #urgent #urgent`).
	//
	// Apple Reminders on a non-iCloud CalDAV account does not surface
	// CATEGORIES as tag chips — its native tag chip system is iCloud-
	// only. So a user who created a tag on the wiki would see
	// nothing tag-shaped in iOS Reminders if we relied on CATEGORIES
	// alone. Embedding `#tag` in the title gives them visible text
	// they can recognize and (in iOS 17+) tap as a hashtag chip.
	//
	// We don't do this for non-Apple clients because DAVx5 / tasks.org
	// honor CATEGORIES and a redundant `#tag` in the title clutters
	// the rendered text without adding signal.
	EmbedTagsInSummary bool
}

// RenderItemWithOptions is the explicit-options variant of RenderItem.
// Used by the backend when the request context indicates a client
// family (e.g. Apple Reminders) that benefits from non-default
// rendering.
func RenderItemWithOptions(item *apiv1.ChecklistItem, page string, listName string, baseURL string, nowFn func() time.Time, opts RenderOptions) []byte {
	cal := renderToCalendar(item, page, listName, baseURL, nowFn, opts)
	if cal == nil {
		return nil
	}
	encoded, err := encodeCalendar(cal)
	if err != nil {
		return nil
	}
	return encoded
}

// productID is emitted as VCALENDAR/PRODID. RFC 5545 requires it; the
// value is a stable opaque string identifying the producing application.
const productID = "-//simple_wiki//CalDAV//EN"

// embedTagsInText appends `#tag` for every entry in tags that's not
// already present in text (case-insensitive). Used for Apple-family
// rendering — see RenderOptions.EmbedTagsInSummary.
//
// Idempotent on round-trip: if iOS sent `Buy milk #urgent` and we
// later re-emit with tags=["urgent"], the existing `#urgent` is
// detected and skipped, so the SUMMARY stays `Buy milk #urgent`.
// Without the dedup the text would grow `Buy milk #urgent #urgent` on
// every read.
func embedTagsInText(text string, tags []string) string {
	if len(tags) == 0 {
		return text
	}
	lower := strings.ToLower(text)
	out := text
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		marker := "#" + strings.ToLower(tag)
		if strings.Contains(lower, marker) {
			continue
		}
		if out != "" {
			out += " "
		}
		out += "#" + tag
		lower = strings.ToLower(out)
	}
	return out
}

// priorityUndefined is the RFC 5545 PRIORITY value for "no
// user-set priority." iOS Reminders renders any non-zero PRIORITY
// as a "!" badge in the task list, so deriving a per-item priority
// from sort_order turns into visual noise. We emit 0 so iOS shows
// no badges and let X-APPLE-SORT-ORDER carry the actual ordering.
//
// PRIORITY is semantic importance, not display order, per RFC 5545
// §3.8.1.9 and per the user's intent — keeping these orthogonal
// also means a future "set priority" UI can write an honest
// PRIORITY without colliding with the order field.
const priorityUndefined = 0

// sortOrderStep matches checklistmutator.SortOrderStep — the
// conventional spacing between adjacent items' sort_order values.
// Still referenced by the inbound parser's PRIORITY-to-sort_order
// scaling; kept here to avoid an icalcodec → checklistmutator
// import cycle.
const sortOrderStep = 1000

// xAppleSortOrder is the Apple-namespaced extension property Apple
// Reminders uses to order tasks within a list.
const xAppleSortOrder = "X-APPLE-SORT-ORDER"

// sortOrderBase10 is the numeric base for X-APPLE-SORT-ORDER's value
// (a decimal integer per Apple's convention).
const sortOrderBase10 = 10

// statusValueCompleted / statusValueNeedsAction are the two STATUS
// values we emit on VTODO.
const (
	statusValueCompleted   = "COMPLETED"
	statusValueNeedsAction = "NEEDS-ACTION"
)

// percentCompleteDone / percentCompletePending are the values we emit
// on PERCENT-COMPLETE. PERCENT-COMPLETE is informational for older
// clients that don't read STATUS COMPLETED.
const (
	percentCompleteDone    = "100"
	percentCompletePending = "0"
)

// renderToCalendar builds the VCALENDAR/VTODO tree but does not encode
// it. Split out of RenderItem so future callers (e.g., the multiget
// REPORT handler that wants a *ical.Calendar to merge) can reuse the
// mapping logic without going through bytes.
func renderToCalendar(item *apiv1.ChecklistItem, page string, listName string, baseURL string, nowFn func() time.Time, opts RenderOptions) *ical.Calendar {
	if item == nil {
		return nil
	}
	now := nowFn()

	todo := ical.NewComponent(ical.CompToDo)
	todo.Props.SetText(ical.PropUID, item.Uid)
	summary := item.Text
	if opts.EmbedTagsInSummary {
		summary = embedTagsInText(summary, item.Tags)
	}
	todo.Props.SetText(ical.PropSummary, summary)

	if item.Checked {
		todo.Props.SetText(ical.PropStatus, statusValueCompleted)
		setRawValue(todo, ical.PropPercentComplete, percentCompleteDone)
		if item.CompletedAt != nil {
			todo.Props.SetDateTime(ical.PropCompleted, item.CompletedAt.AsTime())
		}
	} else {
		todo.Props.SetText(ical.PropStatus, statusValueNeedsAction)
		setRawValue(todo, ical.PropPercentComplete, percentCompletePending)
	}

	if len(item.Tags) > 0 {
		categories := ical.NewProp(ical.PropCategories)
		categories.SetTextList(item.Tags)
		todo.Props.Set(categories)
	}

	setRawValue(todo, xAppleSortOrder, strconv.FormatInt(item.SortOrder, sortOrderBase10))
	setRawValue(todo, ical.PropPriority, strconv.Itoa(priorityUndefined))

	if u, err := url.Parse(buildBacklink(baseURL, page, listName)); err == nil {
		todo.Props.SetURI(ical.PropURL, u)
	}

	if item.Description != nil && *item.Description != "" {
		todo.Props.SetText(ical.PropDescription, *item.Description)
	}

	if item.Due != nil {
		todo.Props.SetDateTime(ical.PropDue, item.Due.AsTime())
	}

	if item.CreatedAt != nil {
		todo.Props.SetDateTime(ical.PropCreated, item.CreatedAt.AsTime())
	}
	if item.UpdatedAt != nil {
		todo.Props.SetDateTime(ical.PropLastModified, item.UpdatedAt.AsTime())
	}
	todo.Props.SetDateTime(ical.PropDateTimeStamp, now)

	if item.AlarmPayload != nil && *item.AlarmPayload != "" {
		alarm, err := RenderAlarm(*item.AlarmPayload, item.Text)
		if err == nil && alarm != nil {
			todo.Children = append(todo.Children, alarm)
		}
	}

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, productID)
	cal.Children = append(cal.Children, todo)
	return cal
}

// buildBacklink returns the URL property pointing at the wiki page
// view, tolerating a trailing slash on baseURL.
//
// listName is currently elided from the back-link — phones/tablets jump
// to the page, not to a specific checklist within it. We keep the
// parameter on the public Render signature in case future surfaces
// (e.g., one URL per item) want it.
func buildBacklink(baseURL, page, _ string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	return trimmed + "/" + page + "/view"
}

// setRawValue writes a property whose iCalendar value type is non-TEXT
// (PRIORITY/PERCENT-COMPLETE = INTEGER, X-APPLE-SORT-ORDER = vendor
// extension). Going through Props.SetText would mark the value as
// VALUE=TEXT in the wire form, which CalDAV clients may parse as a
// string instead of the expected type. Constructing a Prop directly
// with no Params keeps the wire form clean: `NAME:VALUE`.
func setRawValue(comp *ical.Component, name, value string) {
	prop := ical.NewProp(name)
	prop.Value = value
	comp.Props.Set(prop)
}

// encodeCalendar serializes an *ical.Calendar to bytes using the
// library's encoder. Centralized so RenderItem and any future caller
// share the same line-folding / escape behavior.
func encodeCalendar(cal *ical.Calendar) ([]byte, error) {
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
