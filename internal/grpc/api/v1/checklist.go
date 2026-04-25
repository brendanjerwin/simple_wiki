package v1

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errChecklistMutatorNotConfigured is returned when a ChecklistService RPC
// is called on a Server that wasn't constructed with a checklistmutator.
// Indicates a wiring bug in production.
var errChecklistMutatorNotConfigured = status.Error(codes.FailedPrecondition, "checklist mutator not configured on server")

// AddItem implements the AddItem RPC.
func (s *Server) AddItem(ctx context.Context, req *apiv1.AddItemRequest) (*apiv1.AddItemResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}

	args := checklistmutator.AddItemArgs{
		Text:         req.GetText(),
		Tags:         req.GetTags(),
		Description:  req.Description,
		AlarmPayload: nil,
	}
	if req.SortOrder != nil {
		so := req.GetSortOrder()
		args.SortOrder = &so
	}
	if req.Due != nil {
		due := req.Due.AsTime()
		args.Due = &due
	}

	identity := tailscale.IdentityFromContext(ctx)
	item, list, err := s.checklistMutator.AddItem(ctx, req.GetPage(), req.GetListName(), args, identity)
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.AddItemResponse{Item: item, Checklist: list}, nil
}

// UpdateItem implements the UpdateItem RPC.
func (s *Server) UpdateItem(ctx context.Context, req *apiv1.UpdateItemRequest) (*apiv1.UpdateItemResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}

	args := checklistmutator.UpdateItemArgs{
		Text:        req.Text,
		Tags:        req.GetTags(),
		TagsSet:     req.Tags != nil,
		Description: req.Description,
		DescriptionSet: req.Description != nil,
		AlarmPayload: req.AlarmPayload,
		AlarmPayloadSet: req.AlarmPayload != nil,
		DueSet:      req.Due != nil,
	}
	if req.Due != nil {
		due := req.Due.AsTime()
		args.Due = &due
	}

	identity := tailscale.IdentityFromContext(ctx)
	expected := timestampPtr(req.ExpectedUpdatedAt)
	item, list, err := s.checklistMutator.UpdateItem(ctx, req.GetPage(), req.GetListName(), req.GetUid(), args, expected, identity)
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.UpdateItemResponse{Item: item, Checklist: list}, nil
}

// ToggleItem implements the ToggleItem RPC.
func (s *Server) ToggleItem(ctx context.Context, req *apiv1.ToggleItemRequest) (*apiv1.ToggleItemResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}

	identity := tailscale.IdentityFromContext(ctx)
	expected := timestampPtr(req.ExpectedUpdatedAt)
	item, list, err := s.checklistMutator.ToggleItem(ctx, req.GetPage(), req.GetListName(), req.GetUid(), expected, identity)
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.ToggleItemResponse{Item: item, Checklist: list}, nil
}

// DeleteItem implements the DeleteItem RPC.
func (s *Server) DeleteItem(ctx context.Context, req *apiv1.DeleteItemRequest) (*apiv1.DeleteItemResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}

	identity := tailscale.IdentityFromContext(ctx)
	expected := timestampPtr(req.ExpectedUpdatedAt)
	list, err := s.checklistMutator.DeleteItem(ctx, req.GetPage(), req.GetListName(), req.GetUid(), expected, identity)
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.DeleteItemResponse{Checklist: list}, nil
}

// ReorderItem implements the ReorderItem RPC.
func (s *Server) ReorderItem(ctx context.Context, req *apiv1.ReorderItemRequest) (*apiv1.ReorderItemResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}

	identity := tailscale.IdentityFromContext(ctx)
	expected := timestampPtr(req.ExpectedUpdatedAt)
	list, err := s.checklistMutator.ReorderItem(ctx, req.GetPage(), req.GetListName(), req.GetUid(), req.GetNewSortOrder(), expected, identity)
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.ReorderItemResponse{Checklist: list}, nil
}

// ListItems implements the ListItems RPC.
func (s *Server) ListItems(ctx context.Context, req *apiv1.ListItemsRequest) (*apiv1.ListItemsResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}

	list, err := s.checklistMutator.ListItems(ctx, req.GetPage(), req.GetListName())
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.ListItemsResponse{Checklist: list}, nil
}

// GetChecklists implements the GetChecklists RPC.
func (s *Server) GetChecklists(ctx context.Context, req *apiv1.GetChecklistsRequest) (*apiv1.GetChecklistsResponse, error) {
	if s.checklistMutator == nil {
		return nil, errChecklistMutatorNotConfigured
	}
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}

	lists, err := s.checklistMutator.GetChecklists(ctx, req.GetPage())
	if err != nil {
		return nil, mapChecklistMutatorErr(err)
	}
	return &apiv1.GetChecklistsResponse{Checklists: lists}, nil
}

// timestampPtr converts an optional timestamppb to a *time.Time.
func timestampPtr(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

// mapChecklistMutatorErr converts mutator-package errors to gRPC status
// errors. Mutator-internal errors that are already status.Error pass
// through unchanged.
func mapChecklistMutatorErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, checklistmutator.ErrItemNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, checklistmutator.ErrListNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, checklistmutator.ErrPageNotFound):
		return status.Errorf(codes.NotFound, "page not found")
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Errorf(codes.Internal, "checklist mutation: %v", err)
}
