package v1

import (
	"context"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
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
}

// AgentPageBindingStore is the contract for reading and writing the per-page
// agent binding stored in agent.selected_agent_id frontmatter. The concrete
// implementation lives in the server package (AgentPageBindingStore).
type AgentPageBindingStore interface {
	GetSelectedAgentID(page string) (string, error)
	SetSelectedAgentID(page, agentID string) error
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
	agents := s.agentInventory.List()
	return &apiv1.ListAgentsResponse{Agents: agents}, nil
}

// GetPageAgent implements AgentInventoryService.GetPageAgent.
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

	selectedID, err := s.agentPageBindingStore.GetSelectedAgentID(req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get page agent: %v", err)
	}

	explicitlySet := selectedID != ""
	agentID := selectedID
	if agentID == "" {
		agentID = s.agentInventory.DefaultAgentID()
	}

	agent := s.agentInventory.FindByID(agentID)
	if agent == nil {
		// The stored ID no longer exists — fall back to default.
		agentID = s.agentInventory.DefaultAgentID()
		agent = s.agentInventory.FindByID(agentID)
		explicitlySet = false
	}
	if agent == nil {
		return nil, status.Error(codes.Internal, "no default agent configured")
	}

	return &apiv1.GetPageAgentResponse{
		Agent:        agent,
		ExplicitlySet: explicitlySet,
	}, nil
}

// SetPageAgent implements AgentInventoryService.SetPageAgent.
func (s *Server) SetPageAgent(_ context.Context, req *apiv1.SetPageAgentRequest) (*apiv1.SetPageAgentResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
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

	if err := s.agentPageBindingStore.SetSelectedAgentID(req.GetPage(), req.GetAgentId()); err != nil {
		return nil, status.Errorf(codes.Internal, "set page agent: %v", err)
	}

	return &apiv1.SetPageAgentResponse{Agent: agent}, nil
}
