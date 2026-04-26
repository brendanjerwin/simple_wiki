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
func RenderItem(item *apiv1.ChecklistItem, page string, listName string, baseURL string, nowFn func() time.Time) []byte {
	cal := renderToCalendar(item, page, listName, baseURL, nowFn)
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

// neutralPriority is the RFC 5545 "no priority" / undefined sentinel.
// We emit it on every VTODO because PRIORITY is required to be in
// 0..9; clients that ignore X-APPLE-SORT-ORDER fall back to PRIORITY,
// and emitting 5 (medium) avoids accidentally communicating "high" or
// "low" to those clients. Real ordering is conveyed by
// X-APPLE-SORT-ORDER.
const neutralPriority = 5

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
func renderToCalendar(item *apiv1.ChecklistItem, page string, listName string, baseURL string, nowFn func() time.Time) *ical.Calendar {
	if item == nil {
		return nil
	}
	now := nowFn()

	todo := ical.NewComponent(ical.CompToDo)
	todo.Props.SetText(ical.PropUID, item.Uid)
	todo.Props.SetText(ical.PropSummary, item.Text)

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
	setRawValue(todo, ical.PropPriority, strconv.Itoa(neutralPriority))

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
