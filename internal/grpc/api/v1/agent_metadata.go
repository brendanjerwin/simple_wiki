package v1

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// errAgentScheduleStoreNotConfigured is returned when an AgentMetadataService
// schedule RPC is called on a Server that wasn't constructed with an
// AgentScheduleStore. Indicates a wiring bug in production.
var errAgentScheduleStoreNotConfigured = status.Error(codes.FailedPrecondition, "agent schedule store not configured on server")

// errAgentChatContextStoreNotConfigured mirrors the above for the chat-context
// store.
var errAgentChatContextStoreNotConfigured = status.Error(codes.FailedPrecondition, "agent chat-context store not configured on server")

// scheduleCronParser parses the 6-field cron format used everywhere else in
// this codebase (sec min hr dom mon dow). The seconds field is optional so a
// 5-field cron is accepted too.
var scheduleCronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ListSchedules implements the ListSchedules RPC.
func (s *Server) ListSchedules(_ context.Context, req *apiv1.ListSchedulesRequest) (*apiv1.ListSchedulesResponse, error) {
	if s.agentScheduleStore == nil {
		return nil, errAgentScheduleStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	schedules, err := s.agentScheduleStore.List(req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list schedules: %v", err)
	}
	return &apiv1.ListSchedulesResponse{Schedules: schedules}, nil
}

// UpsertSchedule implements the UpsertSchedule RPC. Validates the cron
// expression and silently strips the wiki-managed status fields off the input
// so the only legal mutation path for those fields remains the schedule
// state machine.
func (s *Server) UpsertSchedule(_ context.Context, req *apiv1.UpsertScheduleRequest) (*apiv1.UpsertScheduleResponse, error) {
	if s.agentScheduleStore == nil {
		return nil, errAgentScheduleStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	schedule := req.GetSchedule()
	if schedule == nil {
		return nil, status.Error(codes.InvalidArgument, "schedule is required")
	}
	if schedule.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule.id is required")
	}
	if _, err := scheduleCronParser.Parse(schedule.GetCron()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid cron expression %q: %v", schedule.GetCron(), err)
	}
	// Empty timezone is treated as UTC at register time. A non-empty value
	// must be a valid IANA name so we don't surface the failure deep in the
	// scheduler later when the user can't see it.
	if tz := schedule.GetTimezone(); tz != "" {
		if _, tzErr := time.LoadLocation(tz); tzErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid timezone %q: %v", tz, tzErr)
		}
	}

	// Strip wiki-managed fields so callers cannot forge status. The store
	// layer also strips them; we belt-and-suspenders here so the response
	// reflects the truth.
	clean := &apiv1.AgentSchedule{
		Id:       schedule.GetId(),
		Cron:     schedule.GetCron(),
		Prompt:   schedule.GetPrompt(),
		MaxTurns: schedule.GetMaxTurns(),
		Enabled:  schedule.GetEnabled(),
		Timezone: schedule.GetTimezone(),
	}
	if err := s.agentScheduleStore.Upsert(req.GetPage(), clean); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert schedule: %v", err)
	}
	// Re-read so the response carries any preserved wiki-managed fields from
	// the prior record.
	current, err := s.agentScheduleStore.List(req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "re-read schedules: %v", err)
	}
	for _, sc := range current {
		if sc.GetId() == clean.GetId() {
			return &apiv1.UpsertScheduleResponse{Schedule: sc}, nil
		}
	}
	// Defensive fallback; the upsert should always be readable.
	return &apiv1.UpsertScheduleResponse{Schedule: clean}, nil
}

// DeleteSchedule implements the DeleteSchedule RPC. Idempotent.
func (s *Server) DeleteSchedule(_ context.Context, req *apiv1.DeleteScheduleRequest) (*apiv1.DeleteScheduleResponse, error) {
	if s.agentScheduleStore == nil {
		return nil, errAgentScheduleStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetScheduleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}
	if err := s.agentScheduleStore.Delete(req.GetPage(), req.GetScheduleId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete schedule: %v", err)
	}
	return &apiv1.DeleteScheduleResponse{}, nil
}

// GetChatContext implements the GetChatContext RPC.
func (s *Server) GetChatContext(_ context.Context, req *apiv1.GetChatContextRequest) (*apiv1.GetChatContextResponse, error) {
	if s.agentChatContextStore == nil {
		return nil, errAgentChatContextStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	ctx, err := s.agentChatContextStore.Read(req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read chat context: %v", err)
	}
	return &apiv1.GetChatContextResponse{ChatContext: ctx}, nil
}

// UpdateChatContext implements the UpdateChatContext RPC. The store
// server-stamps last_updated; merge semantics are documented on the store.
func (s *Server) UpdateChatContext(_ context.Context, req *apiv1.UpdateChatContextRequest) (*apiv1.UpdateChatContextResponse, error) {
	if s.agentChatContextStore == nil {
		return nil, errAgentChatContextStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetChatContext() == nil {
		return nil, status.Error(codes.InvalidArgument, "chat_context is required")
	}
	merged, err := s.agentChatContextStore.UpdateMerge(req.GetPage(), req.GetChatContext())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update chat context: %v", err)
	}
	return &apiv1.UpdateChatContextResponse{ChatContext: merged}, nil
}

// AppendBackgroundActivitySummary implements the
// AppendBackgroundActivitySummary RPC. Maps SummaryTargetNotFoundError to
// codes.NotFound so callers (typically scheduled agents calling out of order)
// see a clear failure mode.
func (s *Server) AppendBackgroundActivitySummary(_ context.Context, req *apiv1.AppendBackgroundActivitySummaryRequest) (*apiv1.AppendBackgroundActivitySummaryResponse, error) {
	if s.agentChatContextStore == nil {
		return nil, errAgentChatContextStoreNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetScheduleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}
	if req.GetSummary() == "" {
		return nil, status.Error(codes.InvalidArgument, "summary is required")
	}

	err := s.agentChatContextStore.AppendBackgroundActivitySummary(req.GetPage(), req.GetScheduleId(), req.GetSummary())
	if err == nil {
		return &apiv1.AppendBackgroundActivitySummaryResponse{}, nil
	}
	// Map the typed not-found error to the gRPC NotFound code so the agent
	// can distinguish "no recent entry to attach to" from infrastructure
	// failure.
	if isSummaryTargetNotFound(err) {
		return nil, status.Errorf(codes.NotFound, "no recent background-activity entry for schedule %q on page %q", req.GetScheduleId(), req.GetPage())
	}
	return nil, status.Errorf(codes.Internal, "append summary: %v", err)
}

// isSummaryTargetNotFound returns true when err (or any error in its chain)
// originated as a SummaryTargetNotFoundError from the server-package
// chat-context store. The gRPC layer can't errors.As on the typed error
// without growing an awkward import (the store interface lives here, but the
// concrete error type lives there). Substring check is acceptable because the
// error message is stable and the alternative — re-exporting a sentinel
// across the package boundary — adds two more files for one lookup.
func isSummaryTargetNotFound(err error) bool {
	const sentinel = "no recent background-activity entry for schedule"
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		if strings.Contains(cur.Error(), sentinel) {
			return true
		}
	}
	return false
}
