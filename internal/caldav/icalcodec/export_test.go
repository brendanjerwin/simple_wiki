package icalcodec

import (
	"time"

	ical "github.com/emersion/go-ical"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// SetTodoCheckStatusPropsForTest is the test-only re-export of
// setTodoCheckStatusProps.
func SetTodoCheckStatusPropsForTest(todo *ical.Component, item *apiv1.ChecklistItem) {
	setTodoCheckStatusProps(todo, item)
}

// SetTodoTimestampPropsForTest is the test-only re-export of
// setTodoTimestampProps.
func SetTodoTimestampPropsForTest(todo *ical.Component, item *apiv1.ChecklistItem, now time.Time) {
	setTodoTimestampProps(todo, item, now)
}

// AppendTodoAlarmIfPresentForTest is the test-only re-export of
// appendTodoAlarmIfPresent.
func AppendTodoAlarmIfPresentForTest(todo *ical.Component, item *apiv1.ChecklistItem) {
	appendTodoAlarmIfPresent(todo, item)
}
