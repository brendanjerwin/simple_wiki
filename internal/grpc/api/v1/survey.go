package v1

import (
	"context"
	"errors"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server/surveymutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var errSurveyMutatorNotConfigured = status.Error(codes.FailedPrecondition, "survey mutator not configured on server")

// GetSurvey implements the GetSurvey RPC.
func (s *Server) GetSurvey(ctx context.Context, req *apiv1.GetSurveyRequest) (*apiv1.GetSurveyResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); authErr != nil {
		return nil, authErr
	}
	survey, err := s.surveyMutator.GetSurvey(ctx, req.GetPage(), req.GetName())
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.GetSurveyResponse{Survey: survey}, nil
}

// UpsertSurvey implements the UpsertSurvey RPC.
func (s *Server) UpsertSurvey(ctx context.Context, req *apiv1.UpsertSurveyRequest) (*apiv1.UpsertSurveyResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetName()); err != nil {
		return nil, err
	}
	expected := timestampPtr(req.ExpectedUpdatedAt)
	survey, err := s.surveyMutator.UpsertSurvey(ctx, req.GetPage(), req.GetName(), req.GetQuestion(), req.Closed, expected)
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.UpsertSurveyResponse{Survey: survey}, nil
}

// AddField implements the AddField RPC.
func (s *Server) AddField(ctx context.Context, req *apiv1.AddSurveyFieldRequest) (*apiv1.AddSurveyFieldResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	survey, err := s.surveyMutator.AddField(ctx, req.GetPage(), req.GetSurveyName(), req.GetField(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.AddSurveyFieldResponse{Survey: survey}, nil
}

// UpdateField implements the UpdateField RPC.
func (s *Server) UpdateField(ctx context.Context, req *apiv1.UpdateSurveyFieldRequest) (*apiv1.UpdateSurveyFieldResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	if req.GetFieldName() == "" {
		return nil, status.Error(codes.InvalidArgument, "field_name is required")
	}
	survey, err := s.surveyMutator.UpdateField(ctx, req.GetPage(), req.GetSurveyName(), req.GetFieldName(), req.GetField(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.UpdateSurveyFieldResponse{Survey: survey}, nil
}

// RemoveField implements the RemoveField RPC.
func (s *Server) RemoveField(ctx context.Context, req *apiv1.RemoveSurveyFieldRequest) (*apiv1.RemoveSurveyFieldResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	if req.GetFieldName() == "" {
		return nil, status.Error(codes.InvalidArgument, "field_name is required")
	}
	survey, err := s.surveyMutator.RemoveField(ctx, req.GetPage(), req.GetSurveyName(), req.GetFieldName(), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.RemoveSurveyFieldResponse{Survey: survey}, nil
}

// ReorderField implements the ReorderField RPC.
func (s *Server) ReorderField(ctx context.Context, req *apiv1.ReorderSurveyFieldRequest) (*apiv1.ReorderSurveyFieldResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	if req.GetFieldName() == "" {
		return nil, status.Error(codes.InvalidArgument, "field_name is required")
	}
	survey, err := s.surveyMutator.ReorderField(ctx, req.GetPage(), req.GetSurveyName(), req.GetFieldName(), int(req.GetNewIndex()), timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.ReorderSurveyFieldResponse{Survey: survey}, nil
}

// SubmitResponse implements the SubmitResponse RPC.
func (s *Server) SubmitResponse(ctx context.Context, req *apiv1.SubmitSurveyResponseRequest) (*apiv1.SubmitSurveyResponseResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	identity := tailscale.IdentityFromContext(ctx)
	survey, err := s.surveyMutator.SubmitResponse(ctx, req.GetPage(), req.GetSurveyName(), req.GetValues(), req.GetAnonymous(), identity)
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.SubmitSurveyResponseResponse{Survey: survey}, nil
}

// ListResponses implements the ListResponses RPC.
func (s *Server) ListResponses(ctx context.Context, req *apiv1.ListSurveyResponsesRequest) (*apiv1.ListSurveyResponsesResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetSurveyName() == "" {
		return nil, status.Error(codes.InvalidArgument, "survey_name is required")
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); authErr != nil {
		return nil, authErr
	}
	responses, err := s.surveyMutator.ListResponses(ctx, req.GetPage(), req.GetSurveyName())
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.ListSurveyResponsesResponse{Responses: responses}, nil
}

// DeleteResponse implements the DeleteResponse RPC.
func (s *Server) DeleteResponse(ctx context.Context, req *apiv1.DeleteSurveyResponseRequest) (*apiv1.DeleteSurveyResponseResponse, error) {
	if s.surveyMutator == nil {
		return nil, errSurveyMutatorNotConfigured
	}
	if err := s.requireSurveyMutation(ctx, req.GetPage(), req.GetSurveyName()); err != nil {
		return nil, err
	}
	var submittedAt *time.Time
	if req.GetSubmittedAt() != nil {
		t := req.GetSubmittedAt().AsTime()
		submittedAt = &t
	}
	survey, err := s.surveyMutator.DeleteResponse(ctx, req.GetPage(), req.GetSurveyName(), req.GetUser(), submittedAt, timestampPtr(req.ExpectedUpdatedAt))
	if err != nil {
		return nil, mapSurveyMutatorErr(err)
	}
	return &apiv1.DeleteSurveyResponseResponse{Survey: survey}, nil
}

func (s *Server) requireSurveyMutation(ctx context.Context, page, surveyName string) error {
	if page == "" {
		return status.Error(codes.InvalidArgument, errPageRequired)
	}
	if surveyName == "" {
		return status.Error(codes.InvalidArgument, "survey_name is required")
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(page)); guardErr != nil {
		return guardErr
	}
	return requireAuthorized(ctx, s.pageReaderMutator, wikipage.PageIdentifier(page))
}

func mapSurveyMutatorErr(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}
	switch {
	case errors.Is(err, surveymutator.ErrSurveyNotFound):
		return status.Error(codes.NotFound, "survey not found")
	case errors.Is(err, surveymutator.ErrFieldNotFound):
		return status.Error(codes.NotFound, "survey field not found")
	case errors.Is(err, surveymutator.ErrResponseNotFound):
		return status.Error(codes.NotFound, "survey response not found")
	case errors.Is(err, surveymutator.ErrFieldExists):
		return status.Error(codes.AlreadyExists, "survey field already exists")
	case errors.Is(err, surveymutator.ErrPageNotFound), errors.Is(err, os.ErrNotExist):
		return status.Error(codes.NotFound, "page not found")
	default:
		return status.Errorf(codes.Internal, "survey mutation failed: %v", err)
	}
}
