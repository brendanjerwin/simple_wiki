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
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	googlekeep "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errKeepConnectorNotConfigured is returned when the Keep engine path
// hasn't been wired by this wiki's operator. The frontend renders this
// as "set up Google Keep on profile" so the user understands the
// action required.
var errKeepConnectorNotConfigured = status.Error(
	codes.FailedPrecondition,
	"Google Keep connector not configured by this wiki's operator",
)

// errTasksConnectorNotConfigured is returned when the Tasks engine path
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

// errMsgBindingNotFound is the shared NotFound message returned
// by Unbind handlers when no binding matches (page, list_name).
const errMsgBindingNotFound = "binding_not_found"

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

// tasksWired reports whether the Tasks engine collaborators are all
// wired. The handlers branch on this aggregate flag so the per-RPC
// "not configured" error reads consistently.
func (s *Server) tasksWired() bool {
	return s.tasksEngine != nil &&
		s.tasksAdapter != nil &&
		s.tasksBindingStore != nil &&
		s.tasksCredentialStore != nil
}

// keepWired reports whether the Keep engine collaborators are all
// wired. Mirrors tasksWired's shape for Phase 5-A's cutover.
func (s *Server) keepWired() bool {
	return s.keepEngine != nil &&
		s.keepAdapter != nil &&
		s.keepBindingStore != nil &&
		s.keepCredentialStore != nil
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
	if !s.tasksWired() {
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
// here returns FailedPrecondition with a pointer to the right flow.
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
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	if s.keepAuthVerifier == nil {
		return nil, status.Error(codes.FailedPrecondition,
			"Google Keep auth verifier not configured by this wiki's operator")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := s.keepCredentialStore.Connect(ctx, profileID, req.GetEmail(), req.GetOauthToken(), s.keepAuthVerifier)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.CompleteAuth(GOOGLE_KEEP) failed for profile=%s: %v", string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	bindings, err := s.keepBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.CompleteAuth(GOOGLE_KEEP) load bindings for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.CompleteAuthResponse{State: keepStateToProto(bundle, bindings)}, nil
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
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := s.tasksCredentialStore.ClearCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Disconnect(GOOGLE_TASKS) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	bindings, err := s.tasksBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Disconnect(GOOGLE_TASKS) load bindings for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.DisconnectResponse{State: tasksStateToProto(bundle, bindings)}, nil
}

func (s *Server) disconnectKeep(ctx context.Context) (*apiv1.DisconnectResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := s.keepCredentialStore.ClearCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Disconnect(GOOGLE_KEEP) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	bindings, err := s.keepBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Disconnect(GOOGLE_KEEP) load bindings for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.DisconnectResponse{State: keepStateToProto(bundle, bindings)}, nil
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
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := s.tasksCredentialStore.LoadCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetState(GOOGLE_TASKS) load credentials for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	bindings, err := s.tasksBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetState(GOOGLE_TASKS) load bindings for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.GetStateResponse{State: tasksStateToProto(bundle, bindings)}, nil
}

func (s *Server) getStateKeep(ctx context.Context) (*apiv1.GetStateResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := s.keepCredentialStore.LoadCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetState(GOOGLE_KEEP) load credentials for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	bindings, err := s.keepBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetState(GOOGLE_KEEP) load bindings for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.GetStateResponse{State: keepStateToProto(bundle, bindings)}, nil
}

// ListMyBindings implements the ListMyBindings RPC.
func (s *Server) ListMyBindings(ctx context.Context, req *apiv1.ListMyBindingsRequest) (*apiv1.ListMyBindingsResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.listMyBindingsKeep(ctx)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.listMyBindingsTasks(ctx)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) listMyBindingsTasks(ctx context.Context) (*apiv1.ListMyBindingsResponse, error) {
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := s.tasksBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListMyBindings(GOOGLE_TASKS) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	out := make([]*apiv1.BindingState, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, tasksBindingToProto(b))
	}
	return &apiv1.ListMyBindingsResponse{Bindings: out}, nil
}

func (s *Server) listMyBindingsKeep(ctx context.Context) (*apiv1.ListMyBindingsResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := s.keepBindingStore.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListMyBindings(GOOGLE_KEEP) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	out := make([]*apiv1.BindingState, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, keepBindingToProto(b))
	}
	return &apiv1.ListMyBindingsResponse{Bindings: out}, nil
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
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	// Engine path: ensure the profile is configured. The credential
	// store is the authoritative "is connected?" signal.
	bundle, credErr := s.tasksCredentialStore.LoadCredentials(ctx, profileID)
	if credErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListRemoteLists(GOOGLE_TASKS) load credentials for profile=%s: %v",
				string(profileID), credErr)
		}
		return nil, mapTasksConnectorErr(credErr)
	}
	if !bundle.IsConfigured() {
		return nil, mapTasksConnectorErr(googletasks.ErrCredentialMissing)
	}
	collections, err := s.tasksEngine.ListRemoteCollections(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListRemoteLists(GOOGLE_TASKS) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	out := make([]*apiv1.RemoteListSummary, 0, len(collections))
	for _, c := range collections {
		out = append(out, &apiv1.RemoteListSummary{
			RemoteListHandle: c.Handle,
			Title:            c.Title,
		})
	}
	return &apiv1.ListRemoteListsResponse{Lists: out}, nil
}

func (s *Server) listRemoteListsKeep(ctx context.Context) (*apiv1.ListRemoteListsResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, credErr := s.keepCredentialStore.LoadCredentials(ctx, profileID)
	if credErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListRemoteLists(GOOGLE_KEEP) load credentials for profile=%s: %v",
				string(profileID), credErr)
		}
		return nil, mapKeepConnectorErr(credErr)
	}
	if !bundle.IsConfigured() {
		return nil, mapKeepConnectorErr(googlekeep.ErrCredentialMissing)
	}
	collections, err := s.keepEngine.ListRemoteCollections(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListRemoteLists(GOOGLE_KEEP) failed for profile=%s: %v",
				string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	out := make([]*apiv1.RemoteListSummary, 0, len(collections))
	for _, c := range collections {
		out = append(out, &apiv1.RemoteListSummary{
			RemoteListHandle: c.Handle,
			Title:            c.Title,
		})
	}
	return &apiv1.ListRemoteListsResponse{Lists: out}, nil
}

// Bind implements the Bind RPC.
func (s *Server) Bind(ctx context.Context, req *apiv1.BindRequest) (*apiv1.BindResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.bindKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.bindTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) bindTasks(ctx context.Context, req *apiv1.BindRequest) (*apiv1.BindResponse, error) {
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	// Engine path requires a non-empty remote_handle. Empty signals
	// "create a new tasklist named after listName" — the adapter
	// (not the engine) owns remote-list creation.
	bundle, credErr := s.tasksCredentialStore.LoadCredentials(ctx, profileID)
	if credErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Bind(GOOGLE_TASKS) load credentials for profile=%s: %v",
				string(profileID), credErr)
		}
		return nil, mapTasksConnectorErr(credErr)
	}
	if !bundle.IsConfigured() {
		return nil, mapTasksConnectorErr(googletasks.ErrCredentialMissing)
	}

	remoteHandle := req.GetRemoteListHandle()
	if remoteHandle == "" {
		newHandle, _, createErr := s.tasksAdapter.CreateRemoteCollection(ctx, profileID, req.GetListName())
		if createErr != nil {
			if s.logger != nil {
				s.logger.Error("ConnectorService.Bind(GOOGLE_TASKS) create new tasklist for profile=%s list=%s: %v",
					string(profileID), req.GetListName(), createErr)
			}
			return nil, mapTasksConnectorErr(createErr)
		}
		remoteHandle = newHandle
	}

	binding, err := s.tasksEngine.Bind(ctx, profileID, req.GetPage(), req.GetListName(), remoteHandle)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Bind(GOOGLE_TASKS) bind failed for profile=%s page=%s list=%s remote=%q: %v",
				string(profileID), req.GetPage(), req.GetListName(), remoteHandle, err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.BindResponse{Binding: tasksBindingToProto(binding)}, nil
}

func (s *Server) bindKeep(ctx context.Context, req *apiv1.BindRequest) (*apiv1.BindResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, credErr := s.keepCredentialStore.LoadCredentials(ctx, profileID)
	if credErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Bind(GOOGLE_KEEP) load credentials for profile=%s: %v",
				string(profileID), credErr)
		}
		return nil, mapKeepConnectorErr(credErr)
	}
	if !bundle.IsConfigured() {
		return nil, mapKeepConnectorErr(googlekeep.ErrCredentialMissing)
	}

	remoteHandle := req.GetRemoteListHandle()
	if remoteHandle == "" {
		// "Bind to a new Keep note" path: ask the adapter to create
		// the LIST node first, then engine.Bind takes the resulting
		// ServerID. Mirrors the Tasks path's CreateRemoteCollection
		// pre-step.
		var initialItems []connectors.WikiItem
		if s.checklistMutator != nil {
			if checklist, listErr := s.checklistMutator.ListItems(ctx, req.GetPage(), req.GetListName()); listErr == nil {
				for _, it := range checklist.GetItems() {
					initialItems = append(initialItems, connectors.WikiItem{
						UID:         it.GetUid(),
						Text:        it.GetText(),
						Tags:        it.GetTags(),
						Description: it.GetDescription(),
						Checked:     it.GetChecked(),
					})
				}
			}
		}
		newHandle, _, createErr := s.keepAdapter.CreateRemoteCollection(ctx, profileID, req.GetListName(), initialItems)
		if createErr != nil {
			if s.logger != nil {
				s.logger.Error("ConnectorService.Bind(GOOGLE_KEEP) create new note for profile=%s list=%s: %v",
					string(profileID), req.GetListName(), createErr)
			}
			return nil, mapKeepConnectorErr(createErr)
		}
		remoteHandle = newHandle
	}

	binding, err := s.keepEngine.Bind(ctx, profileID, req.GetPage(), req.GetListName(), remoteHandle)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Bind(GOOGLE_KEEP) bind failed for profile=%s page=%s list=%s remote=%q: %v",
				string(profileID), req.GetPage(), req.GetListName(), remoteHandle, err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.BindResponse{Binding: keepBindingToProto(binding)}, nil
}

// Unbind implements the Unbind RPC.
func (s *Server) Unbind(ctx context.Context, req *apiv1.UnbindRequest) (*apiv1.UnbindResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.unbindKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.unbindTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) unbindTasks(ctx context.Context, req *apiv1.UnbindRequest) (*apiv1.UnbindResponse, error) {
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	// engine.Unbind is idempotent (no error on missing binding) per
	// ADR-0011's "Unbind operation contract." The gRPC contract,
	// however, returns NotFound to a caller asking to unbind a
	// binding that doesn't exist — surface that distinction here so
	// the frontend can render the error appropriately.
	_, found, findErr := s.tasksBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleTasks, req.GetPage(), req.GetListName())
	if findErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Unbind(GOOGLE_TASKS) find binding for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), findErr)
		}
		return nil, mapTasksConnectorErr(findErr)
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := s.tasksEngine.Unbind(ctx, profileID, req.GetPage(), req.GetListName()); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Unbind(GOOGLE_TASKS) failed for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.UnbindResponse{}, nil
}

func (s *Server) unbindKeep(ctx context.Context, req *apiv1.UnbindRequest) (*apiv1.UnbindResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	// engine.Unbind is idempotent; preserve the legacy "NotFound on
	// unknown binding" gRPC contract by checking existence first.
	_, found, findErr := s.keepBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleKeep, req.GetPage(), req.GetListName())
	if findErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Unbind(GOOGLE_KEEP) find binding for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), findErr)
		}
		return nil, mapKeepConnectorErr(findErr)
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := s.keepEngine.Unbind(ctx, profileID, req.GetPage(), req.GetListName()); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Unbind(GOOGLE_KEEP) failed for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.UnbindResponse{}, nil
}

// SyncNow triggers an immediate sync for the calling user's binding
// on (page, list_name). Routes through the same Engine.Sync method the
// scheduler / debouncer / adaptive ticker use, so concurrent calls
// serialize on the per-checklist lease (§16.6). NOT a force-full-
// resync — adapter state is preserved.
func (s *Server) SyncNow(ctx context.Context, req *apiv1.SyncNowRequest) (*apiv1.SyncNowResponse, error) {
	switch req.GetConnectorKind() {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		return s.syncNowKeep(ctx, req)
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		return s.syncNowTasks(ctx, req)
	default:
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
	}
}

func (s *Server) syncNowTasks(ctx context.Context, req *apiv1.SyncNowRequest) (*apiv1.SyncNowResponse, error) {
	if !s.tasksWired() {
		return nil, errTasksConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	_, found, findErr := s.tasksBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleTasks, req.GetPage(), req.GetListName())
	if findErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.SyncNow(GOOGLE_TASKS) find binding for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), findErr)
		}
		return nil, mapTasksConnectorErr(findErr)
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := s.tasksEngine.Sync(ctx, connectors.BindingKey{
		ProfileID: string(profileID),
		Page:      req.GetPage(),
		ListName:  req.GetListName(),
	}); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.SyncNow(GOOGLE_TASKS) failed for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, mapTasksConnectorErr(err)
	}
	return &apiv1.SyncNowResponse{}, nil
}

func (s *Server) syncNowKeep(ctx context.Context, req *apiv1.SyncNowRequest) (*apiv1.SyncNowResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	_, found, findErr := s.keepBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleKeep, req.GetPage(), req.GetListName())
	if findErr != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.SyncNow(GOOGLE_KEEP) find binding for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), findErr)
		}
		return nil, mapKeepConnectorErr(findErr)
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := s.keepEngine.Sync(ctx, connectors.BindingKey{
		ProfileID: string(profileID),
		Page:      req.GetPage(),
		ListName:  req.GetListName(),
	}); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.SyncNow(GOOGLE_KEEP) failed for profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	return &apiv1.SyncNowResponse{}, nil
}

// GetChecklistBindingState implements the GetChecklistBindingState
// RPC. This RPC does NOT take connector_kind: at most one connector
// owns a given (page, list_name) per user, so the server walks every
// configured connector to find the owner.
//
// Resolution order: Tasks first, then Keep. The connector_configured
// flag is OR'd across all connectors — if any of the user's connectors
// is configured, the UI should not show the "set up a connector" prompt.
//
// At most one connector owns a (page, list_name) per profile (the
// LeaseTable enforces this at bind time), so the first match wins
// and short-circuits the walk.
func (s *Server) GetChecklistBindingState(ctx context.Context, req *apiv1.GetChecklistBindingStateRequest) (*apiv1.GetChecklistBindingStateResponse, error) {
	if !s.keepWired() && !s.tasksWired() {
		return nil, status.Error(codes.Internal, "connector orchestrators not configured on this server")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	out := &apiv1.ChecklistBindingState{}

	if hit, err := s.checklistBindingFromTasks(ctx, profileID, req, out); err != nil {
		return nil, err
	} else if hit {
		return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
	}

	if hit, err := s.checklistBindingFromKeep(ctx, profileID, req, out); err != nil {
		return nil, err
	} else if hit {
		return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
	}

	return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
}

// checklistBindingFromTasks fills out's CurrentBinding if
// the Tasks engine path owns (page, list_name) for this profile.
// Returns hit=true when a match was written; hit=false on no match
// (caller falls through to Keep). ConnectorConfigured is set
// whenever Tasks is configured for this profile.
func (s *Server) checklistBindingFromTasks(ctx context.Context, profileID wikipage.PageIdentifier, req *apiv1.GetChecklistBindingStateRequest, out *apiv1.ChecklistBindingState) (bool, error) {
	if !s.tasksWired() {
		return false, nil
	}
	bundle, err := s.tasksCredentialStore.LoadCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetChecklistBindingState(GOOGLE_TASKS) load credentials profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return false, mapTasksConnectorErr(err)
	}
	if bundle.IsConfigured() {
		out.ConnectorConfigured = true
	}
	binding, found, err := s.tasksBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleTasks, req.GetPage(), req.GetListName())
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetChecklistBindingState(GOOGLE_TASKS) find binding profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return false, mapTasksConnectorErr(err)
	}
	if !found {
		return false, nil
	}
	out.CurrentBinding = tasksBindingToProto(binding)
	return true, nil
}

// checklistBindingFromKeep is the Keep counterpart of
// checklistBindingFromTasks; same semantics.
func (s *Server) checklistBindingFromKeep(ctx context.Context, profileID wikipage.PageIdentifier, req *apiv1.GetChecklistBindingStateRequest, out *apiv1.ChecklistBindingState) (bool, error) {
	if !s.keepWired() {
		return false, nil
	}
	bundle, err := s.keepCredentialStore.LoadCredentials(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetChecklistBindingState(GOOGLE_KEEP) load credentials profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return false, mapKeepConnectorErr(err)
	}
	if bundle.IsConfigured() {
		out.ConnectorConfigured = true
	}
	binding, found, err := s.keepBindingStore.FindBinding(profileID, connectors.ConnectorKindGoogleKeep, req.GetPage(), req.GetListName())
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetChecklistBindingState(GOOGLE_KEEP) find binding profile=%s page=%s list=%s: %v",
				string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return false, mapKeepConnectorErr(err)
	}
	if !found {
		return false, nil
	}
	out.CurrentBinding = keepBindingToProto(binding)
	return true, nil
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

// listDeadLettersTasks returns an empty list. The Tasks engine path
// inherits the dead-letter semantics from MATRIX row 8 (engine-owned),
// but the BindingStore does not yet expose a per-binding ledger query
// for the gRPC layer. When/if that lands, replace this stub with real
// delegation through engine.ListDeadLetters or similar.
func (s *Server) listDeadLettersTasks(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if !s.tasksWired() {
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

// listDeadLettersKeep is the engine-path Keep counterpart of
// listDeadLettersTasks. The engine owns dead-letter semantics
// (MATRIX row 8); the BindingStore does not yet expose a per-binding
// ledger query for the gRPC layer. Returns an empty list — when the
// engine grows a ledger query, replace this stub with delegation
// through engine.ListDeadLetters or similar.
func (s *Server) listDeadLettersKeep(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	if _, _, err := requireRealUser(ctx); err != nil {
		return nil, err
	}
	return &apiv1.ListDeadLettersResponse{}, nil
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
// Tasks engine path keeps no per-binding dead-letter state to clear
// at this surface yet.
func (s *Server) clearDeadLetterTasks(ctx context.Context, req *apiv1.ClearDeadLetterRequest) (*emptypb.Empty, error) {
	if !s.tasksWired() {
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

// clearDeadLetterKeep is a no-op for now — see listDeadLettersKeep
// for why. Returns NotFound for any item_uid the caller provides
// because the Keep engine path keeps no per-binding dead-letter state
// to clear at this surface yet.
func (s *Server) clearDeadLetterKeep(ctx context.Context, req *apiv1.ClearDeadLetterRequest) (*emptypb.Empty, error) {
	if !s.keepWired() {
		return nil, errKeepConnectorNotConfigured
	}
	if req.GetPage() == "" || req.GetListName() == "" || req.GetItemUid() == "" {
		return nil, status.Error(codes.InvalidArgument, "page, list_name, and item_uid are required")
	}
	if _, _, err := requireRealUser(ctx); err != nil {
		return nil, err
	}
	return nil, status.Error(codes.NotFound, "dead_letter_item_not_found")
}

// --- helpers --------------------------------------------------------------

// keepStateToProto builds the gRPC ConnectorState response from the
// engine-path collaborators (CredentialStore + BindingStore). Mirrors
// tasksStateToProto: connected_at, last_verified_at land verbatim;
// bindings are projected through keepBindingToProto.
//
// Keep does not expose a per-connector poll cadence (uses the unified
// 30s scheduler tick), so PollIntervalSeconds stays zero.
func keepStateToProto(bundle googlekeep.CredentialBundle, bindings []connectors.Binding) *apiv1.ConnectorState {
	out := &apiv1.ConnectorState{
		Configured:    bundle.IsConfigured(),
		Email:         bundle.Email,
		ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
	}
	if !bundle.ConnectedAt.IsZero() {
		out.ConnectedAt = timestamppb.New(bundle.ConnectedAt)
	}
	if !bundle.LastVerifiedAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(bundle.LastVerifiedAt)
	}
	for _, b := range bindings {
		out.Bindings = append(out.Bindings, keepBindingToProto(b))
	}
	return out
}

// keepBindingToProto converts an engine Binding into the proto-shaped
// BindingState. Mirrors tasksBindingToProto.
func keepBindingToProto(b connectors.Binding) *apiv1.BindingState {
	out := &apiv1.BindingState{
		Page:             b.Page,
		ListName:         b.ListName,
		RemoteListHandle: b.RemoteHandle,
		RemoteListTitle:  b.RemoteListTitle,
		Paused:           b.IsPaused(),
		PausedReason:     b.PausedReason,
		ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
	}
	if !b.BoundAt.IsZero() {
		out.BoundAt = timestamppb.New(b.BoundAt)
	}
	if !b.LastSuccessfulSyncAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(b.LastSuccessfulSyncAt)
	}
	return out
}

// tasksStateToProto builds the gRPC ConnectorState response from the
// engine-path collaborators (CredentialStore + BindingStore). Mirrors
// the legacy connector's tasksStateToProto: connected_at, last_verified_at
// land verbatim; bindings are projected through tasksBindingToProto.
//
// Tasks does not expose a per-connector poll cadence (uses the unified
// 30s scheduler tick), so PollIntervalSeconds stays zero.
func tasksStateToProto(bundle googletasks.CredentialBundle, bindings []connectors.Binding) *apiv1.ConnectorState {
	out := &apiv1.ConnectorState{
		Configured:    bundle.IsConfigured(),
		Email:         bundle.Email,
		ConnectorKind: apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
	}
	if !bundle.ConnectedAt.IsZero() {
		out.ConnectedAt = timestamppb.New(bundle.ConnectedAt)
	}
	if !bundle.LastVerifiedAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(bundle.LastVerifiedAt)
	}
	for _, b := range bindings {
		out.Bindings = append(out.Bindings, tasksBindingToProto(b))
	}
	return out
}

// tasksBindingToProto converts an engine Binding into the proto-shaped
// BindingState. Mirrors keepBindingToProto.
func tasksBindingToProto(b connectors.Binding) *apiv1.BindingState {
	out := &apiv1.BindingState{
		Page:             b.Page,
		ListName:         b.ListName,
		RemoteListHandle: b.RemoteHandle,
		RemoteListTitle:  b.RemoteListTitle,
		Paused:           b.IsPaused(),
		PausedReason:     b.PausedReason,
		ConnectorKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS,
	}
	if !b.BoundAt.IsZero() {
		out.BoundAt = timestamppb.New(b.BoundAt)
	}
	if !b.LastSuccessfulSyncAt.IsZero() {
		out.LastVerifiedAt = timestamppb.New(b.LastSuccessfulSyncAt)
	}
	return out
}

// mapTasksConnectorErr maps Tasks engine + adapter errors to typed gRPC
// codes. Branches only on errors.Is — never on string contents.
func mapTasksConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, googletasks.ErrCredentialMissing):
		return status.Error(codes.FailedPrecondition, "tasks_connector_not_configured: connect Google Tasks on your profile first")
	case errors.Is(err, engine.ErrAlreadyBoundForChecklist),
		errors.Is(err, connectors.ErrChecklistAlreadyLeased):
		return status.Error(codes.AlreadyExists, "already_bound_for_checklist: this checklist is already bound by you")
	case errors.Is(err, engine.ErrBindingNotFound):
		return status.Error(codes.NotFound, errMsgBindingNotFound)
	case errors.Is(err, googletasks.ErrTasksListHasSubtasks):
		return status.Error(codes.FailedPrecondition, "tasks_list_has_subtasks: pick a flat tasks list (subtasks are not supported by the wiki's checklist model)")
	case errors.Is(err, tasksgateway.ErrAuthRevoked), errors.Is(err, tasksgateway.ErrInvalidGrant):
		return status.Error(codes.Unauthenticated, "auth_revoked: re-connect Google Tasks on your profile")
	case errors.Is(err, tasksgateway.ErrServiceDisabled):
		// Operator setup error: the Tasks API is not enabled on the
		// GCP project. The activation URL (when present) is already
		// embedded in the gateway error's message — surface it
		// verbatim so the user can click through.
		return status.Errorf(codes.FailedPrecondition, "tasks_api_not_enabled: %v", err)
	case errors.Is(err, tasksgateway.ErrPermissionDenied):
		return status.Errorf(codes.PermissionDenied, "permission_denied: %v", err)
	case errors.Is(err, tasksgateway.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "rate_limited")
	default:
		return status.Errorf(codes.Internal, "tasks connector: %v", err)
	}
}

// mapKeepConnectorErr maps Keep engine + adapter errors to typed gRPC
// codes. Branches only on errors.Is — never on string contents.
func mapKeepConnectorErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, googlekeep.ErrCredentialMissing):
		return status.Error(codes.FailedPrecondition, "keep_connector_not_configured: connect Google Keep on your profile first")
	case errors.Is(err, engine.ErrAlreadyBoundForChecklist),
		errors.Is(err, connectors.ErrChecklistAlreadyLeased):
		return status.Error(codes.AlreadyExists, "already_bound_for_checklist: this checklist is already bound by you")
	case errors.Is(err, engine.ErrBindingNotFound):
		return status.Error(codes.NotFound, errMsgBindingNotFound)
	case errors.Is(err, googlekeep.ErrKeepNoteNotAList):
		return status.Error(codes.FailedPrecondition, "remote_list_not_a_checklist: pick a Keep checklist note (LIST node)")
	case errors.Is(err, gateway.ErrInvalidCredentials):
		return status.Errorf(codes.Unauthenticated, "invalid_credentials: Google rejected the oauth_token (it may have expired — capture a fresh one): %v", err)
	case errors.Is(err, gateway.ErrAuthRevoked):
		return status.Error(codes.Unauthenticated, "auth_revoked: re-connect Google Keep on your profile")
	case errors.Is(err, gateway.ErrProtocolDrift):
		return status.Error(codes.Internal, "protocol_drift: Google Keep API has changed; update simple_wiki")
	case errors.Is(err, gateway.ErrServiceDisabled):
		return status.Errorf(codes.FailedPrecondition, "keep_api_not_enabled: %v", err)
	case errors.Is(err, gateway.ErrPermissionDenied):
		return status.Errorf(codes.PermissionDenied, "permission_denied: %v", err)
	case errors.Is(err, gateway.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, "rate_limited")
	case errors.Is(err, gateway.ErrBoundNoteDeleted):
		return status.Error(codes.NotFound, "remote_list_deleted: the remote list no longer exists")
	default:
		return status.Errorf(codes.Internal, "keep connector: %v", err)
	}
}
