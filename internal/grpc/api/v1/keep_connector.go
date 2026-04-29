package v1

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const errKeepConnectorNotConfiguredMsg = "keep_connector orchestrator not configured on this server"

var errKeepConnectorNotConfigured = status.Error(codes.Internal, errKeepConnectorNotConfiguredMsg)

// requireRealUser rejects anonymous and agent identities. Mirrors the
// /profile route's posture: connector flows are for human users only.
func requireRealUser(ctx context.Context) (tailscale.IdentityValue, wikipage.PageIdentifier, error) {
	identity := tailscale.IdentityFromContext(ctx)
	if identity == nil || identity.IsAnonymous() || identity.IsAgent() {
		return nil, "", status.Error(codes.PermissionDenied, "keep connector requires a real user identity")
	}
	id, err := wikipage.ProfileIdentifierFor(identity.LoginName())
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "derive profile identifier: %v", err)
	}
	return identity, id, nil
}

// ExchangeAndStore implements the ExchangeAndStore RPC.
func (s *Server) ExchangeAndStore(ctx context.Context, req *apiv1.ExchangeAndStoreRequest) (*apiv1.ExchangeAndStoreResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.keepConnector.Connect(ctx, profileID, req.GetEmail(), req.GetOauthToken())
	if err != nil {
		// Log the full error chain server-side so journalctl shows
		// Google's exact rejection (memento/reminders scope, ErrorDetail,
		// etc.) without exposing the captured oauth_token. The mapped
		// gRPC error sent back to the client still redacts.
		if s.logger != nil {
			s.logger.Error("KeepConnectorService.ExchangeAndStore failed for profile=%s: %v", string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.ExchangeAndStoreResponse{State: connectorStateToProto(state)}, nil
}

// Disconnect implements the Disconnect RPC.
func (s *Server) Disconnect(ctx context.Context, _ *apiv1.DisconnectRequest) (*apiv1.DisconnectResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.keepConnector.Disconnect(ctx, profileID)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.DisconnectResponse{State: connectorStateToProto(state)}, nil
}

// GetState implements the GetState RPC.
func (s *Server) GetState(ctx context.Context, _ *apiv1.GetStateRequest) (*apiv1.GetStateResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.keepConnector.GetState(ctx, profileID)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.GetStateResponse{State: connectorStateToProto(state)}, nil
}

// ListMyBindings implements the ListMyBindings RPC.
func (s *Server) ListMyBindings(ctx context.Context, _ *apiv1.ListMyBindingsRequest) (*apiv1.ListMyBindingsResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.keepConnector.GetState(ctx, profileID)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	out := make([]*apiv1.BindingState, 0, len(state.Bindings))
	for _, b := range state.Bindings {
		out = append(out, bindingToProto(b, !state.IsConfigured()))
	}
	return &apiv1.ListMyBindingsResponse{Bindings: out}, nil
}

// ListNotes implements the ListNotes RPC.
func (s *Server) ListNotes(ctx context.Context, _ *apiv1.ListNotesRequest) (*apiv1.ListNotesResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	notes, err := s.keepConnector.ListNotes(ctx, profileID)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	out := make([]*apiv1.KeepNoteSummary, 0, len(notes))
	for _, n := range notes {
		out = append(out, &apiv1.KeepNoteSummary{
			KeepNoteId: n.KeepNoteID,
			Title:      n.Title,
			ItemCount:  int32(n.ItemCount),
		})
	}
	return &apiv1.ListNotesResponse{Notes: out}, nil
}

// BindChecklist implements the BindChecklist RPC.
func (s *Server) BindChecklist(ctx context.Context, req *apiv1.BindChecklistRequest) (*apiv1.BindChecklistResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page and list_name are required")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	// Read existing wiki checklist items so they can ride along with
	// the Keep list creation in the same Changes request. We ignore
	// errors here: a missing-checklist or empty-checklist case should
	// still allow Bind to create an empty Keep note, then the user
	// can add items on either side and the sync engine reconciles.
	var initialItems []bridge.InitialItem
	if req.GetKeepNoteId() == "" && s.checklistMutator != nil {
		if checklist, listErr := s.checklistMutator.ListItems(ctx, req.GetPage(), req.GetListName()); listErr == nil {
			for _, it := range checklist.GetItems() {
				initialItems = append(initialItems, bridge.InitialItem{
					UID:         it.GetUid(),
					Text:        it.GetText(),
					Tags:        it.GetTags(),
					Description: it.GetDescription(),
					Checked:     it.GetChecked(),
				})
			}
		}
	}
	binding, err := s.keepConnector.Bind(ctx, profileID, req.GetPage(), req.GetListName(), req.GetKeepNoteId(), initialItems)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.BindChecklistResponse{Binding: bindingToProto(binding, false)}, nil
}

// UnbindChecklist implements the UnbindChecklist RPC.
func (s *Server) UnbindChecklist(ctx context.Context, req *apiv1.UnbindChecklistRequest) (*apiv1.UnbindChecklistResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.keepConnector.Unbind(ctx, profileID, req.GetPage(), req.GetListName()); err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.UnbindChecklistResponse{}, nil
}

// GetChecklistBindingState implements the GetChecklistBindingState RPC.
func (s *Server) GetChecklistBindingState(ctx context.Context, req *apiv1.GetChecklistBindingStateRequest) (*apiv1.GetChecklistBindingStateResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.keepConnector.GetState(ctx, profileID)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	out := &apiv1.ChecklistBindingState{
		ConnectorConfigured: state.IsConfigured(),
	}
	for _, b := range state.Bindings {
		if b.Page == req.GetPage() && b.ListName == req.GetListName() {
			out.CurrentBinding = bindingToProto(b, false)
			break
		}
	}
	return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
}

// ListDeadLetters implements the ListDeadLetters RPC.
func (s *Server) ListDeadLetters(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page and list_name are required")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.keepConnector.ListDeadLetters(ctx, profileID, req.GetPage(), req.GetListName())
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	out := make([]*apiv1.DeadLetterItem, 0, len(entries))
	for _, e := range entries {
		out = append(out, &apiv1.DeadLetterItem{
			ItemUid:          e.ItemUID,
			Text:             e.Text,
			PushFailureCount: int32(e.PushFailureCount),
			LastFailureCode:  e.LastFailureCode,
		})
	}
	return &apiv1.ListDeadLettersResponse{Items: out}, nil
}

// ClearDeadLetter implements the ClearDeadLetter RPC.
func (s *Server) ClearDeadLetter(ctx context.Context, req *apiv1.ClearDeadLetterRequest) (*emptypb.Empty, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" || req.GetItemUid() == "" {
		return nil, status.Error(codes.InvalidArgument, "page, list_name, and item_uid are required")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.keepConnector.ClearDeadLetter(ctx, profileID, req.GetPage(), req.GetListName(), req.GetItemUid()); err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &emptypb.Empty{}, nil
}

// --- helpers --------------------------------------------------------------

func connectorStateToProto(state bridge.ConnectorState) *apiv1.ConnectorState {
	out := &apiv1.ConnectorState{
		Configured:          state.IsConfigured(),
		Email:               state.Email,
		PollIntervalSeconds: state.PollIntervalSeconds,
	}
	if !state.ConnectedAt.IsZero() {
		out.ConnectedAt = timestamppb.New(state.ConnectedAt)
	}
	if !state.LastVerifiedAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(state.LastVerifiedAt)
	}
	for _, b := range state.Bindings {
		out.Bindings = append(out.Bindings, bindingToProto(b, !state.IsConfigured()))
	}
	return out
}

func bindingToProto(b bridge.Binding, paused bool) *apiv1.BindingState {
	out := &apiv1.BindingState{
		Page:          b.Page,
		ListName:      b.ListName,
		KeepNoteId:    b.KeepNoteID,
		KeepNoteTitle: b.KeepNoteTitle,
		Paused:        paused,
	}
	if !b.BoundAt.IsZero() {
		out.BoundAt = timestamppb.New(b.BoundAt)
	}
	return out
}

// mapKeepConnectorErr maps bridge/protocol errors to typed gRPC codes.
// Branches only on errors.Is — never on string contents.
func mapKeepConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bridge.ErrConnectorNotConfigured):
		return status.Error(codes.FailedPrecondition, "keep_connector_not_configured: connect Google Keep on your profile first")
	case errors.Is(err, bridge.ErrAlreadyBoundForChecklist):
		return status.Error(codes.FailedPrecondition, "already_bound_for_checklist: this checklist is already bound by you")
	case errors.Is(err, bridge.ErrAlreadyBoundToKeepNote):
		return status.Error(codes.FailedPrecondition, "keep_note_already_bound_by_you: pick a different note or unbind first")
	case errors.Is(err, bridge.ErrBindingNotFound):
		return status.Error(codes.NotFound, "binding_not_found")
	case errors.Is(err, bridge.ErrDeadLetterItemNotFound):
		return status.Error(codes.NotFound, "dead_letter_item_not_found")
	case errors.Is(err, protocol.ErrInvalidCredentials):
		return status.Errorf(codes.Unauthenticated, "invalid_credentials: Google rejected the oauth_token (it may have expired — capture a fresh one): %v", err)
	case errors.Is(err, protocol.ErrAuthRevoked):
		return status.Error(codes.Unauthenticated, "auth_revoked: re-connect Google Keep on your profile")
	case errors.Is(err, protocol.ErrProtocolDrift):
		return status.Error(codes.Internal, "protocol_drift: Google Keep API has changed; update simple_wiki")
	case errors.Is(err, protocol.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "rate_limited")
	case errors.Is(err, protocol.ErrBoundNoteDeleted):
		return status.Error(codes.NotFound, "bound_note_deleted: the Keep note no longer exists")
	default:
		return status.Errorf(codes.Internal, "keep connector: %v", err)
	}
}
