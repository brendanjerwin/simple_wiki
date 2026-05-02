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
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const errKeepConnectorNotConfiguredMsg = "keep_connector orchestrator not configured on this server"

var errKeepConnectorNotConfigured = status.Error(codes.Internal, errKeepConnectorNotConfiguredMsg)

// errTasksConnectorNotConfigured is returned when the Tasks connector
// hasn't been wired by this wiki's operator (env vars unset). The
// frontend renders this as "set up Google Tasks on profile" so the
// user understands the action required.
var errTasksConnectorNotConfigured = status.Error(
	codes.FailedPrecondition,
	"Google Tasks connector not configured by this wiki's operator",
)

// errTasksAuthURLBuilderUnavailable is returned by BeginAuth(GOOGLE_TASKS)
// when the OAuth client config (client_id/secret/redirect) is unset.
// The connector itself may still be wired (for inspection RPCs), but
// minting a fresh auth URL requires the operator to set the env vars.
var errTasksAuthURLBuilderUnavailable = status.Error(
	codes.FailedPrecondition,
	"Google Tasks OAuth client not configured by this wiki's operator",
)

// errConnectorKindRequired is returned when an RPC was invoked without
// a valid connector_kind. The unified handler dispatches on this enum,
// so leaving it unset is a programming bug in the caller.
var errConnectorKindRequired = status.Error(codes.InvalidArgument, "connector_kind required")

// errMsgPageAndListRequired is the shared InvalidArgument message used when
// an RPC requires both page and list_name but one or both are empty.
const errMsgPageAndListRequired = "page and list_name are required"

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
// returns the Google authorization URL (with PKCE + state baked in)
// for the frontend to redirect the user to.
func (s *Server) BeginAuth(ctx context.Context, req *apiv1.BeginAuthRequest) (*apiv1.BeginAuthResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		// Keep doesn't use BeginAuth; the caller proceeds straight to
		// CompleteAuth with the browser-captured oauth_token.
		return &apiv1.BeginAuthResponse{}, nil
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.beginAuthTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) beginAuthTasks(ctx context.Context, req *apiv1.BeginAuthRequest) (*apiv1.BeginAuthResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	if s.tasksAuthURLBuilder == nil {
		return nil, errTasksAuthURLBuilderUnavailable
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	authURL, stateToken, err := s.tasksAuthURLBuilder.BuildAuthURL(ctx, string(profileID), req.GetAccountEmail())
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.BeginAuth(GOOGLE_TASKS) failed for profile=%s: %v", string(profileID), err)
		}
		return nil, status.Errorf(codes.Internal, "build Google Tasks auth URL: %v", err)
	}
	return &apiv1.BeginAuthResponse{
		AuthorizationUrl: authURL,
		State:            stateToken,
	}, nil
}

// CompleteAuth implements the CompleteAuth RPC. For Keep, exchanges
// the captured oauth_token for a long-lived master token via gpsoauth.
// For Tasks, this RPC is unused at runtime — the OAuth callback handler
// at /oauth/google/callback owns the code-for-token exchange (it must,
// to honor RFC 9700 §2.1.2 "validate state before exchanging code"
// and to capture the redirect-supplied state token). The Tasks branch
// here returns FailedPrecondition with a pointer to the right flow,
// so a frontend that mistakenly uses it gets a clear signal.
func (s *Server) CompleteAuth(ctx context.Context, req *apiv1.CompleteAuthRequest) (*apiv1.CompleteAuthResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.completeAuthKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return nil, status.Error(
			codes.FailedPrecondition,
			"Google Tasks completes auth via the /oauth/google/callback redirect handler, not via this RPC; call BeginAuth, redirect the user, then poll GetState",
		)
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
		return s.disconnectTasks(ctx)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) disconnectTasks(ctx context.Context) (*apiv1.DisconnectResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.tasksConnector.Disconnect(ctx, profileID)
	if err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.DisconnectResponse{State: tasksStateToProto(state)}, nil
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
		return s.getStateTasks(ctx)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) getStateTasks(ctx context.Context) (*apiv1.GetStateResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.tasksConnector.GetState(ctx, profileID)
	if err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.GetStateResponse{State: tasksStateToProto(state)}, nil
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
		return s.listMySubscriptionsTasks(ctx)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listMySubscriptionsTasks(ctx context.Context) (*apiv1.ListMySubscriptionsResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.tasksConnector.GetState(ctx, profileID)
	if err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	out := make([]*apiv1.SubscriptionState, 0, len(state.Subscriptions))
	for _, sub := range state.Subscriptions {
		out = append(out, tasksSubscriptionToProto(sub))
	}
	return &apiv1.ListMySubscriptionsResponse{Subscriptions: out}, nil
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
	out := make([]*apiv1.SubscriptionState, 0, len(state.Subscriptions))
	for _, b := range state.Subscriptions {
		out = append(out, keepSubscriptionToProto(b, !state.IsConfigured()))
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
		return s.listRemoteListsTasks(ctx)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listRemoteListsTasks(ctx context.Context) (*apiv1.ListRemoteListsResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	lists, err := s.tasksConnector.ListRemoteLists(ctx, profileID)
	if err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	out := make([]*apiv1.RemoteListSummary, 0, len(lists))
	for _, l := range lists {
		out = append(out, &apiv1.RemoteListSummary{
			RemoteListHandle: l.ID,
			Title:            l.Title,
		})
	}
	return &apiv1.ListRemoteListsResponse{Lists: out}, nil
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
		return s.subscribeTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) subscribeTasks(ctx context.Context, req *apiv1.SubscribeRequest) (*apiv1.SubscribeResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	if req.GetRemoteListHandle() == "" {
		return nil, status.Error(codes.InvalidArgument, "remote_list_handle is required for Tasks Subscribe; pick an existing tasklist from ListRemoteLists")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	sub, err := s.tasksConnector.Subscribe(ctx, profileID, req.GetPage(), req.GetListName(), req.GetRemoteListHandle())
	if err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.SubscribeResponse{Subscription: tasksSubscriptionToProto(sub)}, nil
}

func (s *Server) subscribeKeep(ctx context.Context, req *apiv1.SubscribeRequest) (*apiv1.SubscribeResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
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
	subscription, err := s.keepConnector.Bind(ctx, profileID, req.GetPage(), req.GetListName(), req.GetRemoteListHandle(), initialItems)
	if err != nil {
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.SubscribeResponse{Subscription: keepSubscriptionToProto(subscription, false)}, nil
}

// Unsubscribe implements the Unsubscribe RPC.
func (s *Server) Unsubscribe(ctx context.Context, req *apiv1.UnsubscribeRequest) (*apiv1.UnsubscribeResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.unsubscribeKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.unsubscribeTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) unsubscribeTasks(ctx context.Context, req *apiv1.UnsubscribeRequest) (*apiv1.UnsubscribeResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.tasksConnector.Unsubscribe(ctx, profileID, req.GetPage(), req.GetListName()); err != nil {
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.UnsubscribeResponse{}, nil
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
//
// Resolution order: Tasks first, then Keep. The connector_configured
// flag is OR'd across all connectors — if any of the user's connectors
// is configured, the UI should not show the "set up a connector" prompt.
//
// At most one connector owns a (page, list_name) per profile (the
// LeaseTable enforces this at subscribe time), so the first match wins
// and short-circuits the walk.
func (s *Server) GetChecklistSubscriptionState(ctx context.Context, req *apiv1.GetChecklistSubscriptionStateRequest) (*apiv1.GetChecklistSubscriptionStateResponse, error) {
	if s.keepConnector == nil && s.tasksConnector == nil {
		return nil, status.Error(codes.Internal, "connector orchestrators not configured on this server")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	out := &apiv1.ChecklistSubscriptionState{}

	if s.tasksConnector != nil {
		tasksState, err := s.tasksConnector.GetState(ctx, profileID)
		if err != nil {
			return nil, mapTasksConnectorErr(err)
		}
		if tasksState.IsConfigured() {
			out.ConnectorConfigured = true
		}
		for _, sub := range tasksState.Subscriptions {
			if sub.Page == req.GetPage() && sub.ListName == req.GetListName() {
				out.CurrentSubscription = tasksSubscriptionToProto(sub)
				return &apiv1.GetChecklistSubscriptionStateResponse{State: out}, nil
			}
		}
	}

	if s.keepConnector != nil {
		keepState, err := s.keepConnector.GetState(ctx, profileID)
		if err != nil {
			return nil, mapKeepConnectorErr(err)
		}
		if keepState.IsConfigured() {
			out.ConnectorConfigured = true
		}
		for _, b := range keepState.Subscriptions {
			if b.Page == req.GetPage() && b.ListName == req.GetListName() {
				out.CurrentSubscription = keepSubscriptionToProto(b, false)
				return &apiv1.GetChecklistSubscriptionStateResponse{State: out}, nil
			}
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
		return s.listDeadLettersTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

// listDeadLettersTasks returns an empty list. The Tasks connector does
// not yet implement push-failure dead-lettering — Tasks's API is
// idempotent enough that we haven't needed a per-item failure ledger
// (unlike Keep, where stale targetVersion races land in the dead-letter
// pile). When/if we add it, replace this stub with real delegation.
func (s *Server) listDeadLettersTasks(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	if _, _, err := requireRealUser(ctx); err != nil {
		return nil, err
	}
	return &apiv1.ListDeadLettersResponse{}, nil
}

func (s *Server) listDeadLettersKeep(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if s.keepConnector == nil {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
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
		return s.clearDeadLetterTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

// clearDeadLetterTasks is a no-op — see listDeadLettersTasks for why.
// Returns NotFound for any item_uid the caller provides because the
// Tasks connector keeps no dead-letter state to clear.
func (s *Server) clearDeadLetterTasks(ctx context.Context, req *apiv1.ClearDeadLetterRequest) (*emptypb.Empty, error) {
	if s.tasksConnector == nil {
		return nil, errTasksConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" || req.GetItemUid() == "" {
		return nil, status.Error(codes.InvalidArgument, "page, list_name, and item_uid are required")
	}
	if _, _, err := requireRealUser(ctx); err != nil {
		return nil, err
	}
	return nil, status.Error(codes.NotFound, "dead_letter_item_not_found")
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
	for _, b := range state.Subscriptions {
		out.Subscriptions = append(out.Subscriptions, keepSubscriptionToProto(b, !state.IsConfigured()))
	}
	return out
}

func keepSubscriptionToProto(b keepsync.Subscription, paused bool) *apiv1.SubscriptionState {
	out := &apiv1.SubscriptionState{
		Page:             b.Page,
		ListName:         b.ListName,
		RemoteListHandle: b.KeepNoteID,
		RemoteListTitle:  b.KeepNoteTitle,
		Paused:           paused,
		ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
	}
	if !b.SubscribedAt.IsZero() {
		out.SubscribedAt = timestamppb.New(b.SubscribedAt)
	}
	return out
}

// tasksStateToProto converts the Tasks connector's per-user state into
// the proto-shaped ConnectorState the gRPC layer returns. Mirrors
// connectorStateToProto for Keep but reads from the Tasks-typed
// ConnectorState (no PollIntervalSeconds — Tasks uses the unified 30s
// scheduler tick rather than a per-connector poll cadence).
func tasksStateToProto(state taskssync.ConnectorState) *apiv1.ConnectorState {
	out := &apiv1.ConnectorState{
		Configured:    state.IsConfigured(),
		Email:         state.Email,
		ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
	}
	if !state.ConnectedAt.IsZero() {
		out.ConnectedAt = timestamppb.New(state.ConnectedAt)
	}
	if !state.LastVerifiedAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(state.LastVerifiedAt)
	}
	for _, sub := range state.Subscriptions {
		out.Subscriptions = append(out.Subscriptions, tasksSubscriptionToProto(sub))
	}
	return out
}

// tasksSubscriptionToProto converts the Tasks connector's per-binding
// state into the proto-shaped SubscriptionState. Paused state and
// remote_list_handle (Tasks tasklist id) ride through unchanged.
func tasksSubscriptionToProto(sub taskssync.Subscription) *apiv1.SubscriptionState {
	out := &apiv1.SubscriptionState{
		Page:             sub.Page,
		ListName:         sub.ListName,
		RemoteListHandle: sub.RemoteListID,
		RemoteListTitle:  sub.RemoteListTitle,
		Paused:           sub.IsPaused(),
		ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
	}
	if !sub.SubscribedAt.IsZero() {
		out.SubscribedAt = timestamppb.New(sub.SubscribedAt)
	}
	if !sub.LastSuccessfulSyncAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(sub.LastSuccessfulSyncAt)
	}
	return out
}

// mapTasksConnectorErr maps Tasks bridge/protocol errors to typed gRPC
// codes. Branches only on errors.Is — never on string contents.
func mapTasksConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, taskssync.ErrConnectorNotConfigured):
		return status.Error(codes.FailedPrecondition, "tasks_connector_not_configured: connect Google Tasks on your profile first")
	case errors.Is(err, taskssync.ErrAlreadySubscribedForChecklist):
		return status.Error(codes.AlreadyExists, "already_subscribed_for_checklist: this checklist is already subscribed by you")
	case errors.Is(err, taskssync.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, "subscription_not_found")
	case errors.Is(err, taskssync.ErrTasksListHasSubtasks):
		return status.Error(codes.FailedPrecondition, "tasks_list_has_subtasks: pick a flat tasks list (subtasks are not supported by the wiki's checklist model)")
	case errors.Is(err, tasksgateway.ErrAuthRevoked), errors.Is(err, tasksgateway.ErrInvalidGrant):
		return status.Error(codes.Unauthenticated, "auth_revoked: re-connect Google Tasks on your profile")
	case errors.Is(err, tasksgateway.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "rate_limited")
	default:
		return status.Errorf(codes.Internal, "tasks connector: %v", err)
	}
}

// mapKeepConnectorErr maps bridge/protocol errors to typed gRPC codes.
// Branches only on errors.Is — never on string contents.
func mapKeepConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keepsync.ErrConnectorNotConfigured):
		return status.Error(codes.FailedPrecondition, "keep_connector_not_configured: connect Google Keep on your profile first")
	case errors.Is(err, keepsync.ErrAlreadySubscribedForChecklist):
		return status.Error(codes.AlreadyExists, "already_subscribed_for_checklist: this checklist is already subscribed by you")
	case errors.Is(err, keepsync.ErrKeepNoteAlreadyClaimed):
		return status.Error(codes.AlreadyExists, "remote_list_already_subscribed_by_you: pick a different remote list or unsubscribe first")
	case errors.Is(err, connectors.ErrChecklistAlreadyLeased):
		return status.Error(codes.AlreadyExists, "already_subscribed_for_checklist: this checklist is already subscribed (cross-connector)")
	case errors.Is(err, keepsync.ErrSubscriptionNotFound):
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
