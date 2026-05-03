package icalcodec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Sentinel errors so callers can distinguish "no alarm" from
// "malformed alarm" without parsing error messages.
var (
	errEmptyAlarmPayload      = errors.New("icalcodec: empty alarm payload")
	errAlarmMissingTrigger    = errors.New("icalcodec: alarm payload missing trigger")
	errVALARMMissingTrigger   = errors.New("icalcodec: VALARM missing TRIGGER")
)

// AlarmPayload is the JSON shape stored in
// wiki.checklists.<list>.items[].alarm_payload. The wiki itself doesn't
// fire alarms — clients (Apple Reminders, tasks.org) do — so the
// payload is opaque-but-structured and round-trips through CalDAV's
// VALARM component.
//
// Trigger is either an RFC 5545 duration (e.g. "-PT15M") or an
// RFC 3339 timestamp. Absolute=true marks the latter case so the
// renderer can emit `TRIGGER;VALUE=DATE-TIME:...` instead of the
// duration default. Description is optional; when absent the renderer
// falls back to the item's SUMMARY when invoked from RenderItem.
type AlarmPayload struct {
	Trigger     string `json:"trigger"`
	Absolute    bool   `json:"absolute,omitempty"`
	Description string `json:"description,omitempty"`
}

// RenderAlarm decodes payload (JSON) and returns a VALARM component
// suitable for appending to a VTODO. fallbackDescription is used when
// the payload's Description is empty — typically the parent item's
// text. Returns nil and an error when the payload is malformed; the
// caller should treat that as "drop the alarm" rather than failing
// the whole item render.
func RenderAlarm(payload string, fallbackDescription string) (*ical.Component, error) {
	if strings.TrimSpace(payload) == "" {
		return nil, errEmptyAlarmPayload
	}
	var p AlarmPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return nil, fmt.Errorf("icalcodec: decode alarm payload: %w", err)
	}
	if p.Trigger == "" {
		return nil, errAlarmMissingTrigger
	}

	alarm := ical.NewComponent(ical.CompAlarm)
	alarm.Props.SetText(ical.PropAction, "DISPLAY")

	desc := p.Description
	if desc == "" {
		desc = fallbackDescription
	}
	if desc != "" {
		alarm.Props.SetText(ical.PropDescription, desc)
	}

	if p.Absolute {
		t, err := time.Parse(time.RFC3339, p.Trigger)
		if err != nil {
			return nil, fmt.Errorf("icalcodec: alarm absolute trigger not RFC3339: %w", err)
		}
		trigger := ical.NewProp(ical.PropTrigger)
		trigger.Params.Set(ical.ParamValue, string(ical.ValueDateTime))
		trigger.Value = t.UTC().Format("20060102T150405Z")
		alarm.Props.Set(trigger)
	} else {
		// Relative duration trigger: emit raw to skip the library's
		// default-type machinery, which would otherwise normalize the
		// representation.
		setRawValue(alarm, ical.PropTrigger, p.Trigger)
	}
	return alarm, nil
}

// ParseAlarm extracts an AlarmPayload from a VALARM component. Returns
// the JSON-encoded payload string (the wire form for storage) and an
// error if the alarm is malformed. The caller decides whether a
// malformed VALARM dropped from a CalDAV PUT is worth surfacing.
func ParseAlarm(alarm *ical.Component) (string, error) {
	if alarm == nil || alarm.Name != ical.CompAlarm {
		return "", fmt.Errorf("icalcodec: expected VALARM, got %v", alarmName(alarm))
	}
	triggerProp := alarm.Props.Get(ical.PropTrigger)
	if triggerProp == nil {
		return "", errVALARMMissingTrigger
	}

	payload := AlarmPayload{}
	if descProp := alarm.Props.Get(ical.PropDescription); descProp != nil {
		payload.Description = descProp.Value
	}

	valueType := triggerProp.ValueType()
	if valueType == ical.ValueDateTime {
		t, err := time.Parse("20060102T150405Z", triggerProp.Value)
		if err != nil {
			return "", fmt.Errorf("icalcodec: VALARM trigger not iCal datetime: %w", err)
		}
		payload.Absolute = true
		payload.Trigger = t.UTC().Format(time.RFC3339)
	} else {
		// Default value type for TRIGGER is DURATION per RFC 5545.
		payload.Trigger = triggerProp.Value
	}

	out, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("icalcodec: encode alarm payload: %w", err)
	}
	return string(out), nil
}

// alarmName is a small safety helper for the error message in
// ParseAlarm — gives "<nil>" for nil components and the component
// name otherwise.
func alarmName(c *ical.Component) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name
}
