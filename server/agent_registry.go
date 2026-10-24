package server

import (
	"sync"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"google.golang.org/protobuf/proto"
)

// LocalACPAgentID is the stable identifier for the local ACP agent pool.
// This ID is what gets stored in agent.selected_agent_id to refer to the
// built-in pool daemon.
const LocalACPAgentID = "local-acp"

// AgentAvailabilityChecker reports live availability of the ACP pool.
// Implemented by ChatBufferManager (which already tracks pool connectivity).
type AgentAvailabilityChecker interface {
	HasInstanceRequestSubscribers() bool
}

// AgentRegistry holds every agent page chat can route to. It starts with only
// the local ACP agent and is designed for AG-UI remote endpoints to be
// registered later without API changes.
type AgentRegistry struct {
	mu         sync.RWMutex
	agents     []*apiv1.AgentInfo
	availCheck AgentAvailabilityChecker
}

// NewAgentRegistry constructs a registry pre-populated with the local ACP agent.
func NewAgentRegistry(availCheck AgentAvailabilityChecker) *AgentRegistry {
	r := &AgentRegistry{
		availCheck: availCheck,
	}
	r.agents = []*apiv1.AgentInfo{
		{
			Id:          LocalACPAgentID,
			DisplayName: "Local Agent",
			Protocol:    apiv1.AgentProtocol_ACP_LOCAL,
			Description: "The local ACP agent pool managed by this wiki instance.",
		},
	}
	return r
}

// List returns all registered agents, with live availability status set.
func (r *AgentRegistry) List() []*apiv1.AgentInfo {
	poolConnected := r.availCheck != nil && r.availCheck.HasInstanceRequestSubscribers()

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*apiv1.AgentInfo, len(r.agents))
	for i, a := range r.agents {
		cloned, ok := proto.Clone(a).(*apiv1.AgentInfo)
		if !ok {
			continue
		}
		if cloned.Protocol == apiv1.AgentProtocol_ACP_LOCAL {
			cloned.Available = poolConnected
		}
		out[i] = cloned
	}
	return out
}

// FindByID returns the AgentInfo for the given id, or nil if not found.
func (r *AgentRegistry) FindByID(id string) *apiv1.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, a := range r.agents {
		if a.Id == id {
			cloned, ok := proto.Clone(a).(*apiv1.AgentInfo)
			if !ok {
				return nil
			}
			return cloned
		}
	}
	return nil
}

// DefaultAgentID returns the ID of the agent that should be used when no
// per-page binding has been set. Currently always returns LocalACPAgentID.
func (*AgentRegistry) DefaultAgentID() string {
	return LocalACPAgentID
}
