package v1

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// SubscribeScheduledTurns implements the ScheduledTurnService streaming RPC.
// Forwards every dispatched ScheduledTurnRequest from the in-memory dispatcher
// to the connected pool subscriber.
func (s *Server) SubscribeScheduledTurns(_ *apiv1.SubscribeScheduledTurnsRequest, stream apiv1.ScheduledTurnService_SubscribeScheduledTurnsServer) error {
	if s.scheduledTurnDispatcher == nil {
		return status.Error(codes.FailedPrecondition, "scheduled-turn dispatcher not configured on server")
	}

	requests, unsubscribe := s.scheduledTurnDispatcher.Subscribe()
	defer unsubscribe()

	for {
		select {
		case req, ok := <-requests:
			if !ok {
				return nil
			}
			if err := stream.Send(req); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// CompleteScheduledTurn implements the ScheduledTurnService unary RPC. Called
// by the pool when a headless scheduled-agent turn ends (success, error, or
// timeout).
func (s *Server) CompleteScheduledTurn(_ context.Context, req *apiv1.CompleteScheduledTurnRequest) (*apiv1.CompleteScheduledTurnResponse, error) {
	if s.scheduledTurnDispatcher == nil {
		return nil, status.Error(codes.FailedPrecondition, "scheduled-turn dispatcher not configured on server")
	}
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}

	if err := s.scheduledTurnDispatcher.Complete(req); err != nil {
		// Orphan or duplicate completions are normal failure modes (e.g. the
		// pool restarted mid-turn). Surface as InvalidArgument so the pool can
		// see the error and stop retrying.
		return nil, status.Errorf(codes.InvalidArgument, "complete scheduled turn: %v", err)
	}
	return &apiv1.CompleteScheduledTurnResponse{}, nil
}
