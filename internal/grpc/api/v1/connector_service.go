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
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks/gateway"
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

// collectInitialItems pulls the wiki checklist's items into the engine
// boundary's WikiItem shape, used by Bind's "create a new remote
// collection" pre-step. Pre-seed-capable adapters (Keep) consume the
// slice; the others (Tasks) discard it. The mutator is optional: tests
// without it get a nil slice, which the adapters tolerate.
func (s *Server) collectInitialItems(ctx context.Context, page, listName string) []connectors.WikiItem {
	if s.checklistMutator == nil {
		return nil
	}
	checklist, err := s.checklistMutator.ListItems(ctx, page, listName)
	if err != nil {
		return nil
	}
	items := checklist.GetItems()
	out := make([]connectors.WikiItem, 0, len(items))
	for _, it := range items {
		out = append(out, connectors.WikiItem{
			UID:         it.GetUid(),
			Text:        it.GetText(),
			Tags:        it.GetTags(),
			Description: it.GetDescription(),
			Checked:     it.GetChecked(),
		})
	}
	return out
}

type connectorCredentials interface {
	LoadState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error)
	ClearState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error)
	IsConfigured(ctx context.Context, profileID wikipage.PageIdentifier) (bool, error)
	MissingCredentialError() error
}

type connectorRuntime struct {
	protoKind      apiv1.ConnectorKind
	kind           connectors.ConnectorKind
	engine         *engine.Engine
	adapter        connectors.BackendAdapter
	bindingStore   engine.BindingStore
	credentials    connectorCredentials
	mapErr         func(error) error
	bindingToProto func(connectors.Binding) *apiv1.BindingState
}

func (c *connectorRuntime) wired() bool {
	return c != nil &&
		c.engine != nil &&
		c.adapter != nil &&
		c.bindingStore != nil &&
		c.credentials != nil &&
		c.mapErr != nil &&
		c.bindingToProto != nil
}

type tasksConnectorCredentials struct {
	store *googletasks.FrontmatterCredentialStore
}

func (c *tasksConnectorCredentials) LoadState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error) {
	bundle, err := c.store.LoadCredentials(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return tasksStateToProto(bundle, bindings), nil
}

func (c *tasksConnectorCredentials) ClearState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error) {
	bundle, err := c.store.ClearCredentials(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return tasksStateToProto(bundle, bindings), nil
}

func (c *tasksConnectorCredentials) IsConfigured(ctx context.Context, profileID wikipage.PageIdentifier) (bool, error) {
	bundle, err := c.store.LoadCredentials(ctx, profileID)
	if err != nil {
		return false, err
	}
	return bundle.IsConfigured(), nil
}

func (*tasksConnectorCredentials) MissingCredentialError() error {
	return googletasks.ErrCredentialMissing
}

type keepConnectorCredentials struct {
	store *googlekeep.FrontmatterCredentialStore
}

func (c *keepConnectorCredentials) LoadState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error) {
	bundle, err := c.store.LoadCredentials(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return keepStateToProto(bundle, bindings), nil
}

func (c *keepConnectorCredentials) ClearState(ctx context.Context, profileID wikipage.PageIdentifier, bindings []connectors.Binding) (*apiv1.ConnectorState, error) {
	bundle, err := c.store.ClearCredentials(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return keepStateToProto(bundle, bindings), nil
}

func (c *keepConnectorCredentials) IsConfigured(ctx context.Context, profileID wikipage.PageIdentifier) (bool, error) {
	bundle, err := c.store.LoadCredentials(ctx, profileID)
	if err != nil {
		return false, err
	}
	return bundle.IsConfigured(), nil
}

func (*keepConnectorCredentials) MissingCredentialError() error {
	return googlekeep.ErrCredentialMissing
}

// connectorFor resolves a connector kind to its engine-path runtime.
// Adding a new connector kind should add one registration case here,
// then every gRPC handler can keep using the shared runtime methods.
func (s *Server) connectorFor(kind apiv1.ConnectorKind) (*connectorRuntime, error) {
	switch kind {
	case apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		return nil, errConnectorKindRequired
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP:
		connector := s.keepConnector
		if !connector.wired() {
			return nil, errKeepConnectorNotConfigured
		}
		return connector, nil
	case apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS:
		connector := s.tasksConnector
		if !connector.wired() {
			return nil, errTasksConnectorNotConfigured
		}
		return connector, nil
	default:
		return nil, errUnsupportedConnectorKind(kind)
	}
}

// tasksWired reports whether the Tasks engine collaborators are all
// wired. The handlers branch on this aggregate flag so the per-RPC
// "not configured" error reads consistently.
func (s *Server) tasksWired() bool {
	return s.tasksConnector.wired()
}

// keepWired reports whether the Keep engine collaborators are all
// wired. Mirrors tasksWired's shape for Phase 5-A's cutover.
func (s *Server) keepWired() bool {
	return s.keepConnector.wired()
}

func (s *Server) loadConnectorBindings(connector *connectorRuntime, profileID wikipage.PageIdentifier, operation string) ([]connectors.Binding, error) {
	bindings, err := connector.bindingStore.LoadBindings(profileID, connector.kind)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.%s(%s) load bindings for profile=%s: %v",
				operation, connector.kind, string(profileID), err)
		}
		return nil, connector.mapErr(err)
	}
	return bindings, nil
}

func (s *Server) loadConnectorState(ctx context.Context, connector *connectorRuntime, profileID wikipage.PageIdentifier, operation string) (*apiv1.ConnectorState, error) {
	bindings, err := s.loadConnectorBindings(connector, profileID, operation)
	if err != nil {
		return nil, err
	}
	state, err := connector.credentials.LoadState(ctx, profileID, bindings)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.%s(%s) load credentials for profile=%s: %v",
				operation, connector.kind, string(profileID), err)
		}
		return nil, connector.mapErr(err)
	}
	return state, nil
}

func (s *Server) clearConnectorState(ctx context.Context, connector *connectorRuntime, profileID wikipage.PageIdentifier) (*apiv1.ConnectorState, error) {
	bindings, err := s.loadConnectorBindings(connector, profileID, "Disconnect")
	if err != nil {
		return nil, err
	}
	state, err := connector.credentials.ClearState(ctx, profileID, bindings)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Disconnect(%s) failed for profile=%s: %v",
				connector.kind, string(profileID), err)
		}
		return nil, connector.mapErr(err)
	}
	return state, nil
}

func (s *Server) requireConnectorConfigured(ctx context.Context, connector *connectorRuntime, profileID wikipage.PageIdentifier, operation string) error {
	configured, err := connector.credentials.IsConfigured(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.%s(%s) load credentials for profile=%s: %v",
				operation, connector.kind, string(profileID), err)
		}
		return connector.mapErr(err)
	}
	if !configured {
		return connector.mapErr(connector.credentials.MissingCredentialError())
	}
	return nil
}

func (s *Server) findConnectorBinding(connector *connectorRuntime, profileID wikipage.PageIdentifier, page, listName, operation string) (connectors.Binding, bool, error) {
	binding, found, err := connector.bindingStore.FindBinding(profileID, connector.kind, page, listName)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.%s(%s) find binding for profile=%s page=%s list=%s: %v",
				operation, connector.kind, string(profileID), page, listName, err)
		}
		return connectors.Binding{}, false, connector.mapErr(err)
	}
	return binding, found, nil
}

// BeginAuth implements the BeginAuth RPC. For Keep, this is a no-op —
// Keep's flow is single-shot via CompleteAuth. For Tasks, the handler
// returns the Google authorization URL (with PKCE + state baked in)
// for the frontend to redirect the user to.
func (s *Server) BeginAuth(ctx context.Context, req *apiv1.BeginAuthRequest) (*apiv1.BeginAuthResponse, error) {
	kind := req.GetConnectorKind()
	if kind == apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED {
		return nil, errConnectorKindRequired
	}
	if kind == apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP {
		// Keep doesn't use BeginAuth; the caller proceeds straight to
		// CompleteAuth with the browser-captured oauth_token.
		return &apiv1.BeginAuthResponse{}, nil
	}
	connector, err := s.connectorFor(kind)
	if err != nil {
		return nil, err
	}
	return s.beginAuthTasks(ctx, connector, req)
}

func (s *Server) beginAuthTasks(ctx context.Context, connector *connectorRuntime, req *apiv1.BeginAuthRequest) (*apiv1.BeginAuthResponse, error) {
	if connector.protoKind != apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS {
		return nil, errUnsupportedConnectorKind(req.GetConnectorKind())
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
	kind := req.GetConnectorKind()
	if kind == apiv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED {
		return nil, errConnectorKindRequired
	}
	if kind == apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_TASKS {
		return nil, status.Error(
			codes.FailedPrecondition,
			"Google Tasks completes auth via the /oauth/google/callback redirect handler, not via this RPC; call BeginAuth, redirect the user, then poll GetState",
		)
	}
	connector, err := s.connectorFor(kind)
	if err != nil {
		return nil, err
	}
	return s.completeAuthKeep(ctx, connector, req)
}

func (s *Server) completeAuthKeep(ctx context.Context, connector *connectorRuntime, req *apiv1.CompleteAuthRequest) (*apiv1.CompleteAuthResponse, error) {
	credentials, ok := connector.credentials.(*keepConnectorCredentials)
	if !ok {
		return nil, status.Error(codes.Internal, "Google Keep credentials not wired for auth flow")
	}
	if s.keepAuthVerifier == nil {
		return nil, status.Error(codes.FailedPrecondition,
			"Google Keep auth verifier not configured by this wiki's operator")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := credentials.store.Connect(ctx, profileID, req.GetEmail(), req.GetOauthToken(), s.keepAuthVerifier)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.CompleteAuth(GOOGLE_KEEP) failed for profile=%s: %v", string(profileID), err)
		}
		return nil, mapKeepConnectorErr(err)
	}
	bindings, err := s.loadConnectorBindings(connector, profileID, "CompleteAuth")
	if err != nil {
		return nil, err
	}
	return &apiv1.CompleteAuthResponse{State: keepStateToProto(bundle, bindings)}, nil
}

// Disconnect implements the Disconnect RPC.
func (s *Server) Disconnect(ctx context.Context, req *apiv1.DisconnectRequest) (*apiv1.DisconnectResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.clearConnectorState(ctx, connector, profileID)
	if err != nil {
		return nil, err
	}
	return &apiv1.DisconnectResponse{State: state}, nil
}

// GetState implements the GetState RPC.
func (s *Server) GetState(ctx context.Context, req *apiv1.GetStateRequest) (*apiv1.GetStateResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.loadConnectorState(ctx, connector, profileID, "GetState")
	if err != nil {
		return nil, err
	}
	return &apiv1.GetStateResponse{State: state}, nil
}

// ListMyBindings implements the ListMyBindings RPC.
func (s *Server) ListMyBindings(ctx context.Context, req *apiv1.ListMyBindingsRequest) (*apiv1.ListMyBindingsResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := s.loadConnectorBindings(connector, profileID, "ListMyBindings")
	if err != nil {
		return nil, err
	}
	out := make([]*apiv1.BindingState, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, connector.bindingToProto(b))
	}
	return &apiv1.ListMyBindingsResponse{Bindings: out}, nil
}

// ListRemoteLists implements the ListRemoteLists RPC.
func (s *Server) ListRemoteLists(ctx context.Context, req *apiv1.ListRemoteListsRequest) (*apiv1.ListRemoteListsResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireConnectorConfigured(ctx, connector, profileID, "ListRemoteLists"); err != nil {
		return nil, err
	}
	collections, err := connector.engine.ListRemoteCollections(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.ListRemoteLists(%s) failed for profile=%s: %v",
				connector.kind, string(profileID), err)
		}
		return nil, connector.mapErr(err)
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
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireConnectorConfigured(ctx, connector, profileID, "Bind"); err != nil {
		return nil, err
	}

	remoteHandle := req.GetRemoteListHandle()
	if remoteHandle == "" {
		initialItems := s.collectInitialItems(ctx, req.GetPage(), req.GetListName())
		newHandle, _, createErr := connector.adapter.CreateRemoteCollection(ctx, profileID, req.GetListName(), initialItems)
		if createErr != nil {
			if s.logger != nil {
				s.logger.Error("ConnectorService.Bind(%s) create remote collection for profile=%s list=%s: %v",
					connector.kind, string(profileID), req.GetListName(), createErr)
			}
			return nil, connector.mapErr(createErr)
		}
		remoteHandle = newHandle
	}

	binding, err := connector.engine.Bind(ctx, profileID, req.GetPage(), req.GetListName(), remoteHandle)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Bind(%s) bind failed for profile=%s page=%s list=%s remote=%q: %v",
				connector.kind, string(profileID), req.GetPage(), req.GetListName(), remoteHandle, err)
		}
		return nil, connector.mapErr(err)
	}
	return &apiv1.BindResponse{Binding: connector.bindingToProto(binding)}, nil
}

// Unbind implements the Unbind RPC.
func (s *Server) Unbind(ctx context.Context, req *apiv1.UnbindRequest) (*apiv1.UnbindResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
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
	_, found, findErr := s.findConnectorBinding(connector, profileID, req.GetPage(), req.GetListName(), "Unbind")
	if findErr != nil {
		return nil, findErr
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := connector.engine.Unbind(ctx, profileID, req.GetPage(), req.GetListName()); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.Unbind(%s) failed for profile=%s page=%s list=%s: %v",
				connector.kind, string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, connector.mapErr(err)
	}
	return &apiv1.UnbindResponse{}, nil
}

// SyncNow triggers an immediate sync for the calling user's binding
// on (page, list_name). Routes through the same Engine.Sync method the
// scheduler / debouncer / adaptive ticker use, so concurrent calls
// serialize on the per-checklist lease (§16.6). NOT a force-full-
// resync — adapter state is preserved.
func (s *Server) SyncNow(ctx context.Context, req *apiv1.SyncNowRequest) (*apiv1.SyncNowResponse, error) {
	connector, err := s.connectorFor(req.GetConnectorKind())
	if err != nil {
		return nil, err
	}
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	_, found, findErr := s.findConnectorBinding(connector, profileID, req.GetPage(), req.GetListName(), "SyncNow")
	if findErr != nil {
		return nil, findErr
	}
	if !found {
		return nil, status.Error(codes.NotFound, errMsgBindingNotFound)
	}
	if err := connector.engine.Sync(ctx, connectors.BindingKey{
		ProfileID: string(profileID),
		Page:      req.GetPage(),
		ListName:  req.GetListName(),
	}); err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.SyncNow(%s) failed for profile=%s page=%s list=%s: %v",
				connector.kind, string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return nil, connector.mapErr(err)
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
	if req.GetPage() == "" || req.GetListName() == "" {
		return nil, status.Error(codes.InvalidArgument, errMsgPageAndListRequired)
	}
	if !s.keepWired() && !s.tasksWired() {
		return nil, status.Error(codes.Internal, "connector orchestrators not configured on this server")
	}
	_, profileID, err := requireRealUser(ctx)
	if err != nil {
		return nil, err
	}
	out := &apiv1.ChecklistBindingState{}

	for _, connector := range []*connectorRuntime{s.tasksConnector, s.keepConnector} {
		if hit, err := s.checklistBindingFromConnector(ctx, connector, profileID, req, out); err != nil {
			return nil, err
		} else if hit {
			return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
		}
	}

	return &apiv1.GetChecklistBindingStateResponse{State: out}, nil
}

// checklistBindingFromConnector fills out's CurrentBinding if the
// connector owns (page, list_name) for this profile.
// Returns hit=true when a match was written; hit=false on no match
// (caller falls through to the next connector). ConnectorConfigured is
// set whenever any connector is configured for this profile.
func (s *Server) checklistBindingFromConnector(ctx context.Context, connector *connectorRuntime, profileID wikipage.PageIdentifier, req *apiv1.GetChecklistBindingStateRequest, out *apiv1.ChecklistBindingState) (bool, error) {
	if !connector.wired() {
		return false, nil
	}
	configured, err := connector.credentials.IsConfigured(ctx, profileID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("ConnectorService.GetChecklistBindingState(%s) load credentials profile=%s page=%s list=%s: %v",
				connector.kind, string(profileID), req.GetPage(), req.GetListName(), err)
		}
		return false, connector.mapErr(err)
	}
	if configured {
		out.ConnectorConfigured = true
	}
	binding, found, err := s.findConnectorBinding(connector, profileID, req.GetPage(), req.GetListName(), "GetChecklistBindingState")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	out.CurrentBinding = connector.bindingToProto(binding)
	return true, nil
}

// ListDeadLetters implements the ListDeadLetters RPC.
func (s *Server) ListDeadLetters(ctx context.Context, req *apiv1.ListDeadLettersRequest) (*apiv1.ListDeadLettersResponse, error) {
	if _, err := s.connectorFor(req.GetConnectorKind()); err != nil {
		return nil, err
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
	if _, err := s.connectorFor(req.GetConnectorKind()); err != nil {
		return nil, err
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
