package server

import (
	"fmt"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// IllegalScheduleTransitionError is returned by ValidateScheduleTransition when
// the requested status transition is not in the legal-transition table.
type IllegalScheduleTransitionError struct {
	From apiv1.ScheduleStatus
	To   apiv1.ScheduleStatus
}

// Error implements the error interface.
func (e *IllegalScheduleTransitionError) Error() string {
	return fmt.Sprintf("illegal schedule transition: %s -> %s", e.From, e.To)
}

// legalScheduleTransitions enumerates every transition the state machine
// considers valid. Anything outside this set is rejected with an
// IllegalScheduleTransitionError. UNSPECIFIED is initial-only and is never a
// valid destination; terminal-to-terminal transitions are rejected to catch
// bookkeeping bugs.
var legalScheduleTransitions = map[[2]apiv1.ScheduleStatus]bool{
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_UNSPECIFIED, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING}: true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, apiv1.ScheduleStatus_SCHEDULE_STATUS_OK}:          true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR}:       true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT}:     true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING}:          true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING}:       true,
	{apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING}:     true,
}

// ValidateScheduleTransition returns nil if the transition from -> to is legal
// and an *IllegalScheduleTransitionError otherwise.
func ValidateScheduleTransition(from, to apiv1.ScheduleStatus) error {
	if legalScheduleTransitions[[2]apiv1.ScheduleStatus{from, to}] {
		return nil
	}
	return &IllegalScheduleTransitionError{From: from, To: to}
}
