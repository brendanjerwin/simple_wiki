package v1

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const errKeepConnectorNotConfiguredMsg = "keep_connector orchestrator not configured on this server"

var errKeepConnectorNotConfigured = status.Error(codes.Internal, errKeepConnectorNotConfiguredMsg)

// errConnectorKindRequired is returned when an RPC was invoked without
// a valid connector_kind. The unified handler dispatches on this enum,
// so leaving it unset is a programming bug in the caller.
var errConnectorKindRequired = status.Error(codes.InvalidArgument, "connector_kind required")

// requireRealUser rejects anonymous and agent identities. Mirrors the
// /profile route's posture: connector flows are for human users only.
func requireRealUser(ctx context.Context) (tailscale.IdentityValue, wikipage.PageIdentifier, error) {
	identity := tailscale.IdentityFromContext(ctx)
	if identity == nil || identity.IsAnonymous() || identity.IsAgent() {
		return nil, "", status.Error(codes.PermissionDenied, "connector requires a real user identity")
	}
	id, err := wikipage.ProfileIdentifierFor(identity.LoginName())
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "derive profile identifier: %v", err)
	}
	return identity, id, nil
}

// errUnsupportedConnectorKind returns Unimplemented for kinds that are
// reserved in the proto but not yet wired on this server. The handler
// uses Unimplemented (rather than InvalidArgument) so a future server
// version can light up the same kind without proto churn.
func errUnsupportedConnectorKind(kind apiv1.ConnectorKind) error {
	return status.Error(codes.Unimplemented, fmt.Sprintf("unsupported connector_kind: %v", kind))
}

// BeginAuth implements the BeginAuth RPC. For Keep, this is a no-op —
// Keep's flow is single-shot via CompleteAuth. For Tasks, the handler
// would return the Google authorization URL (Phase 7 wires that in).
func (s *Server) BeginAuth(_ context.Context, req *apiv1.BeginAuthRequest) (*apiv1.BeginAuthResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		// Keep doesn't use BeginAuth; the caller proceeds straight to
		// CompleteAuth with the browser-captured oauth_token.
		return &apiv1.BeginAuthResponse{}, nil
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		// Phase 7 wires the Tasks Connector into BeginAuth. Until
		// then, return Unimplemented so the frontend can detect the
		// unconfigured state cleanly.
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

// CompleteAuth implements the CompleteAuth RPC. For Keep, exchanges
// the captured oauth_token for a long-lived master token via gpsoauth.
// For Tasks, exchanges the authorization code (matched against state +
// code_verifier from BeginAuth) for refresh+access tokens.
func (s *Server) CompleteAuth(ctx context.Context, req *apiv1.CompleteAuthRequest) (*apiv1.CompleteAuthResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.completeAuthKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		// Phase 7 wires the Tasks Connector into CompleteAuth.
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) completeAuthKeep(ctx context.Context, req *apiv1.CompleteAuthRequest) (*apiv1.CompleteAuthResponse, error) {
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
			s.logger.Error("ConnectorService.CompleteAuth(GOOGLE_KEEP) failed for profile=%s: %v", string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.CompleteAuthResponse{State: connectorStateToProto(state, apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP)}, nil
}

// Disconnect implements the Disconnect RPC.
func (s *Server) Disconnect(ctx context.Context, req *apiv1.DisconnectRequest) (*apiv1.DisconnectResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.disconnectKeep(ctx)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) disconnectKeep(ctx context.Context) (*apiv1.DisconnectResponse, error) {
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
	return &apiv1.DisconnectResponse{State: connectorStateToProto(state, apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP)}, nil
}

// GetState implements the GetState RPC.
func (s *Server) GetState(ctx context.Context, req *apiv1.GetStateRequest) (*apiv1.GetStateResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.getStateKeep(ctx)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) getStateKeep(ctx context.Context) (*apiv1.GetStateResponse, error) {
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
	return &apiv1.GetStateResponse{State: connectorStateToProto(state, apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP)}, nil
}

// ListMySubscriptions implements the ListMySubscriptions RPC.
func (s *Server) ListMySubscriptions(ctx context.Context, req *apiv1.ListMySubscriptionsRequest) (*apiv1.ListMySubscriptionsResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.listMySubscriptionsKeep(ctx)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listMySubscriptionsKeep(ctx context.Context) (*apiv1.ListMySubscriptionsResponse, error) {
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
	out := make([]*apiv1.SubscriptionState, 0, len(state.Bindings))
	for _, b := range state.Bindings {
		out = append(out, keepBindingToProto(b, !state.IsConfigured()))
	}
	return &apiv1.ListMySubscriptionsResponse{Subscriptions: out}, nil
}

// ListRemoteLists implements the ListRemoteLists RPC.
func (s *Server) ListRemoteLists(ctx context.Context, req *apiv1.ListRemoteListsRequest) (*apiv1.ListRemoteListsResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.listRemoteListsKeep(ctx)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listRemoteListsKeep(ctx context.Context) (*apiv1.ListRemoteListsResponse, error) {
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
	out := make([]*apiv1.RemoteListSummary, 0, len(notes))
	for _, n := range notes {
		out = append(out, &apiv1.RemoteListSummary{
			RemoteListHandle: n.KeepNoteID,
			Title:            n.Title,
			ItemCount:        int32(n.ItemCount),
		})
	}
	return &apiv1.ListRemoteListsResponse{Lists: out}, nil
}

// Subscribe implements the Subscribe RPC.
func (s *Server) Subscribe(ctx context.Context, req *apiv1.SubscribeRequest) (*apiv1.SubscribeResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.subscribeKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) subscribeKeep(ctx context.Context, req *apiv1.SubscribeRequest) (*apiv1.SubscribeResponse, error) {
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
	// still allow Subscribe to create an empty Keep note, then the
	// user can add items on either side and the sync engine
	// reconciles.
	var initialItems []keepsync.InitialItem
	if req.GetRemoteListHandle() == "" && s.checklistMutator != nil {
		if checklist, listErr := s.checklistMutator.ListItems(ctx, req.GetPage(), req.GetListName()); listErr == nil {
			for _, it := range checklist.GetItems() {
				initialItems = append(initialItems, keepsync.InitialItem{
					UID:         it.GetUid(),
					Text:        it.GetText(),
					Tags:        it.GetTags(),
					Description: it.GetDescription(),
					Checked:     it.GetChecked(),
				})
			}
		}
	}
	binding, err := s.keepConnector.Bind(ctx, profileID, req.GetPage(), req.GetListName(), req.GetRemoteListHandle(), initialItems)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.SubscribeResponse{Subscription: keepBindingToProto(binding, false)}, nil
}

// Unsubscribe implements the Unsubscribe RPC.
func (s *Server) Unsubscribe(ctx context.Context, req *apiv1.UnsubscribeRequest) (*apiv1.UnsubscribeResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.unsubscribeKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) unsubscribeKeep(ctx context.Context, req *apiv1.UnsubscribeRequest) (*apiv1.UnsubscribeResponse, error) {
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
	return &apiv1.UnsubscribeResponse{}, nil
}

// GetChecklistSubscriptionState implements the GetChecklistSubscriptionState
// RPC. This RPC does NOT take connector_kind: at most one connector
// owns a given (page, list_name) per user, so the server walks every
// configured connector to find the owner.
func (s *Server) GetChecklistSubscriptionState(ctx context.Context, req *apiv1.GetChecklistSubscriptionStateRequest) (*apiv1.GetChecklistSubscriptionStateResponse, error) {
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
	out := &apiv1.ChecklistSubscriptionState{
		ConnectorConfigured: state.IsConfigured(),
	}
	for _, b := range state.Bindings {
		if b.Page == req.GetPage() && b.ListName == req.GetListName() {
			out.CurrentSubscription = keepBindingToProto(b, false)
			break
		}
	}
	return &apiv1.GetChecklistSubscriptionStateResponse{State: out}, nil
}

// ListDeadLetters implements the ListDeadLetters RPC.
func (s *Server) ListDeadLetters(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.listDeadLettersKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listDeadLettersKeep(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
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
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.clearDeadLetterKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) clearDeadLetterKeep(ctx context.Context, req *apiv1.ClearDeadLetterRequest) (*emptypb.Empty, error) {
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

func connectorStateToProto(state keepsync.ConnectorState, kind apiv1.ConnectorKind) *apiv1.ConnectorState {
	out := &apiv1.ConnectorState{
		Configured:          state.IsConfigured(),
		Email:               state.Email,
		PollIntervalSeconds: state.PollIntervalSeconds,
		ConnectorKind:       kind,
	}
	if !state.ConnectedAt.IsZero() {
		out.ConnectedAt = timestamppb.New(state.ConnectedAt)
	}
	if !state.LastVerifiedAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(state.LastVerifiedAt)
	}
	for _, b := range state.Bindings {
		out.Subscriptions = append(out.Subscriptions, keepBindingToProto(b, !state.IsConfigured()))
	}
	return out
}

func keepBindingToProto(b keepsync.Binding, paused bool) *apiv1.SubscriptionState {
	out := &apiv1.SubscriptionState{
		Page:             b.Page,
		ListName:         b.ListName,
		RemoteListHandle: b.KeepNoteID,
		RemoteListTitle:  b.KeepNoteTitle,
		Paused:           paused,
		ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
	}
	if !b.BoundAt.IsZero() {
		out.SubscribedAt = timestamppb.New(b.BoundAt)
	}
	return out
}

// mapKeepConnectorErr maps bridge/protocol errors to typed gRPC codes.
// Branches only on errors.Is — never on string contents.
func mapKeepConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keepsync.ErrConnectorNotConfigured):
		return status.Error(codes.FailedPrecondition, "keep_connector_not_configured: connect Google Keep on your profile first")
	case errors.Is(err, keepsync.ErrAlreadyBoundForChecklist):
		return status.Error(codes.FailedPrecondition, "already_subscribed_for_checklist: this checklist is already subscribed by you")
	case errors.Is(err, keepsync.ErrAlreadyBoundToKeepNote):
		return status.Error(codes.FailedPrecondition, "remote_list_already_subscribed_by_you: pick a different remote list or unsubscribe first")
	case errors.Is(err, keepsync.ErrBindingNotFound):
		return status.Error(codes.NotFound, "subscription_not_found")
	case errors.Is(err, keepsync.ErrDeadLetterItemNotFound):
		return status.Error(codes.NotFound, "dead_letter_item_not_found")
	case errors.Is(err, gateway.ErrInvalidCredentials):
		return status.Errorf(codes.Unauthenticated, "invalid_credentials: Google rejected the oauth_token (it may have expired — capture a fresh one): %v", err)
	case errors.Is(err, gateway.ErrAuthRevoked):
		return status.Error(codes.Unauthenticated, "auth_revoked: re-connect Google Keep on your profile")
	case errors.Is(err, gateway.ErrProtocolDrift):
		return status.Error(codes.Internal, "protocol_drift: Google Keep API has changed; update simple_wiki")
	case errors.Is(err, gateway.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "rate_limited")
	case errors.Is(err, gateway.ErrBoundNoteDeleted):
		return status.Error(codes.NotFound, "remote_list_deleted: the remote list no longer exists")
	default:
		return status.Errorf(codes.Internal, "keep connector: %v", err)
	}
}
