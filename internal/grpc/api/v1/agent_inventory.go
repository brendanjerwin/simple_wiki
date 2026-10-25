package v1

import (
	"context"
	"errors"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentInventory is the contract for listing available agents and resolving
// default/fallback agent selection. The concrete implementation lives in the
// server package (AgentRegistry).
type AgentInventory interface {
	List() []*apiv1.AgentInfo
	FindByID(id string) *apiv1.AgentInfo
	DefaultAgentID() string

	// RegisterPoolAgent idempotently upserts a pool-managed agent descriptor
	// and marks it available. Accepts ACP_LOCAL and AG_UI_REMOTE protocols.
	RegisterPoolAgent(req *apiv1.RegisterPoolAgentRequest) error
}

// AgentPageBindingStore is the contract for reading and writing the per-page
// agent binding stored in the agent.chat_agent frontmatter subtree. The
// concrete implementation lives in the server package (AgentPageBindingStore).
type AgentPageBindingStore interface {
	GetBinding(page string) (*server.PageAgentBinding, error)
	SetBinding(page string, input *server.AgentBindingInput) (updatedAt string, err error)
}

// ChatAgentChangeNotifier is the contract for recording a binding switch into
// the page's chat buffer (replayed to all participants).
type ChatAgentChangeNotifier interface {
	NotifyAgentChanged(page, agentID, agentDisplayName, switchedBy string)
}

// errAgentInventoryNotConfigured is returned when an AgentInventoryService RPC
// is called on a Server that wasn't constructed with an AgentInventory.
var errAgentInventoryNotConfigured = status.Error(
	codes.FailedPrecondition,
	"agent inventory not configured on server",
)

// errAgentPageBindingStoreNotConfigured is returned when an
// AgentInventoryService RPC that needs the binding store is called on a Server
// that wasn't constructed with one.
var errAgentPageBindingStoreNotConfigured = status.Error(
	codes.FailedPrecondition,
	"agent page binding store not configured on server",
)

// ListAgents implements AgentInventoryService.ListAgents.
func (s *Server) ListAgents(_ context.Context, _ *apiv1.ListAgentsRequest) (*apiv1.ListAgentsResponse, error) {
	if s.agentInventory == nil {
		return nil, errAgentInventoryNotConfigured
	}
	return &apiv1.ListAgentsResponse{Agents: s.agentInventory.List()}, nil
}

// GetPageAgent implements AgentInventoryService.GetPageAgent.
// Read-side protection (who may read which pages) is enforced by middleware;
// this RPC intentionally has no page guard so the chat panel can display the
// agent binding on any page it can already view.
func (s *Server) GetPageAgent(_ context.Context, req *apiv1.GetPageAgentRequest) (*apiv1.GetPageAgentResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if s.agentInventory == nil {
		return nil, errAgentInventoryNotConfigured
	}
	if s.agentPageBindingStore == nil {
		return nil, errAgentPageBindingStoreNotConfigured
	}

	binding, err := s.agentPageBindingStore.GetBinding(req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get page agent: %v", err)
	}

	if binding == nil {
		agent := s.agentInventory.FindByID(s.agentInventory.DefaultAgentID())
		if agent == nil {
			return nil, status.Error(codes.Internal, "no default agent configured")
		}
		return &apiv1.GetPageAgentResponse{
			Agent:         agent,
			ExplicitlySet: false,
		}, nil
	}

	agent := s.agentInventory.FindByID(binding.AgentID)
	if agent == nil {
		// Orphaned binding: report the desired agent as unavailable instead of
		// silently falling back — the persisted intent stays visible.
		return &apiv1.GetPageAgentResponse{
			Agent: &apiv1.AgentInfo{
				Id:          binding.AgentID,
				DisplayName: binding.DisplayName,
				Available:   false,
			},
			ExplicitlySet: true,
			UpdatedAt:     binding.UpdatedAt,
		}, nil
	}

	return &apiv1.GetPageAgentResponse{
		Agent:         agent,
		ExplicitlySet: true,
		UpdatedAt:     binding.UpdatedAt,
	}, nil
}

// SetPageAgent implements AgentInventoryService.SetPageAgent.
func (s *Server) SetPageAgent(ctx context.Context, req *apiv1.SetPageAgentRequest) (*apiv1.SetPageAgentResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	// Guard runs before the store-not-configured check so a request against a
	// system page reports the actionable "system page" error instead of the
	// internal-wiring error (same ordering as AgentMetadataService mutations).
	if guardErr := requireUserMutable(s.pageReaderMutator, wikipage.PageIdentifier(req.GetPage())); guardErr != nil {
		return nil, guardErr
	}
	if s.agentInventory == nil {
		return nil, errAgentInventoryNotConfigured
	}
	if s.agentPageBindingStore == nil {
		return nil, errAgentPageBindingStoreNotConfigured
	}

	agent := s.agentInventory.FindByID(req.GetAgentId())
	if agent == nil {
		return nil, status.Errorf(codes.NotFound, "agent %q not found in inventory", req.GetAgentId())
	}

	if req.GetExpectedUpdatedAt() != "" && !server.IsValidRFC3339String(req.GetExpectedUpdatedAt()) {
		return nil, status.Error(codes.InvalidArgument, "expected_updated_at must be an RFC3339 timestamp")
	}

	updatedAt, err := s.agentPageBindingStore.SetBinding(req.GetPage(), &server.AgentBindingInput{
		AgentID:           req.GetAgentId(),
		DisplayName:       agent.GetDisplayName(),
		UpdatedBy:         s.identityName(ctx),
		ExpectedUpdatedAt: req.GetExpectedUpdatedAt(),
	})
	if err != nil {
		if errors.Is(err, server.ErrBindingConflict) {
			return nil, status.Error(codes.FailedPrecondition, "agent binding changed concurrently; refetch and retry")
		}
		return nil, status.Errorf(codes.Internal, "set page agent: %v", err)
	}

	if s.chatAgentChangeNotifier != nil {
		s.chatAgentChangeNotifier.NotifyAgentChanged(req.GetPage(), agent.GetId(), agent.GetDisplayName(), s.identityName(ctx))
	}

	return &apiv1.SetPageAgentResponse{
		Agent:     agent,
		UpdatedAt: updatedAt,
	}, nil
}

// RegisterPoolAgent implements AgentInventoryService.RegisterPoolAgent.
// Called by the pool daemon at startup and on every reconnect.
func (s *Server) RegisterPoolAgent(_ context.Context, req *apiv1.RegisterPoolAgentRequest) (*apiv1.RegisterPoolAgentResponse, error) {
	if s.agentInventory == nil {
		return nil, errAgentInventoryNotConfigured
	}
	if err := s.agentInventory.RegisterPoolAgent(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "register pool agent: %v", err)
	}
	return &apiv1.RegisterPoolAgentResponse{}, nil
}

// NotifyAgentAttached implements ChatService.NotifyAgentAttached: the pool's
// report of which agent is EFFECTIVELY attached to a page.
func (s *Server) NotifyAgentAttached(_ context.Context, req *apiv1.NotifyAgentAttachedRequest) (*apiv1.NotifyAgentAttachedResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if s.chatBufferManager == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat buffer manager not configured on server")
	}
	s.chatBufferManager.NotifyAgentAttached(req.GetPage(), req.GetAgentId(), req.GetAgentDisplayName())
	return &apiv1.NotifyAgentAttachedResponse{}, nil
}
