package server

import (
	"fmt"
	"sync"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"google.golang.org/protobuf/proto"
)

// LocalACPAgentID is the stable identifier for the local ACP agent pool.
// This ID is what gets stored in the agent.chat_agent binding to refer to
// the bundled pool daemon.
const LocalACPAgentID = "local-acp"

// AgentRegistry holds every agent page chat can route to. Availability comes
// from two authorities (issue #1276 design, 2026-10-24):
//
//   - ACP_LOCAL entries exist only after the pool daemon registers itself
//     (RegisterPoolAgent) and become unavailable again when the pool
//     connection drops (PoolDisconnected). The pool is the sole authority on
//     local agents.
//   - AG_UI_REMOTE entries come from the wiki server's static configuration
//     (SetConfiguredAgents); they advertise as unavailable until #1275 adds
//     the AG-UI pool backend.
//
// Config cannot overwrite a pool-registered descriptor: the pool is the
// authority for its own agents.
type AgentRegistry struct {
	mu sync.RWMutex

	// poolRegistered tracks whether the pool currently has its agent
	// registered (available).
	poolRegistered bool

	// poolAgent is the pool-registered descriptor (nil until registered).
	poolAgent *apiv1.AgentInfo

	// configuredAgents are the AG-UI entries loaded from server config,
	// keyed by id.
	configuredAgents map[string]*apiv1.AgentInfo
}

// NewAgentRegistry constructs an empty registry; the local ACP agent lists
// (as unavailable) as soon as the pool registers it.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		configuredAgents: make(map[string]*apiv1.AgentInfo),
	}
}

// RegisterPoolAgent idempotently upserts the pool daemon's local agent
// descriptor and marks it available. Only ACP_LOCAL agents under
// LocalACPAgentID may register here — AG-UI entries come exclusively from
// server configuration.
func (r *AgentRegistry) RegisterPoolAgent(req *apiv1.RegisterPoolAgentRequest) error {
	if req.GetProtocol() != apiv1.AgentProtocol_ACP_LOCAL {
		return fmt.Errorf("RegisterPoolAgent: protocol must be ACP_LOCAL, got %s", req.GetProtocol())
	}
	if req.GetId() != LocalACPAgentID {
		return fmt.Errorf("RegisterPoolAgent: %q is not a registered pool agent id (expected %s)", req.GetId(), LocalACPAgentID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.poolAgent = &apiv1.AgentInfo{
		Id:          req.GetId(),
		DisplayName: req.GetDisplayName(),
		Protocol:    req.GetProtocol(),
		Description: req.GetDescription(),
		Route:       req.GetRoute(),
		Available:   true,
	}
	r.poolRegistered = true
	return nil
}

// PoolDisconnected marks the pool-registered agent unavailable while keeping
// the entry listed.
func (r *AgentRegistry) PoolDisconnected() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.poolRegistered = false
	if r.poolAgent != nil {
		r.poolAgent.Available = false
	}
}

// SetConfiguredAgents replaces the configuration-supplied AG-UI entries.
// Entries whose id matches the pool agent are ignored (the pool is the
// authority for its own agents).
func (r *AgentRegistry) SetConfiguredAgents(agents []*apiv1.AgentInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	replacement := make(map[string]*apiv1.AgentInfo, len(agents))
	for _, a := range agents {
		if a == nil || a.GetId() == "" || a.GetId() == LocalACPAgentID {
			continue
		}
		entry, ok := proto.Clone(a).(*apiv1.AgentInfo)
		if !ok {
			continue
		}
		if entry.GetProtocol() == apiv1.AgentProtocol_ACP_LOCAL {
			continue // config never declares local-protocol entries
		}
		entry.Available = false // routing awaits #1275's AG-UI backend
		replacement[entry.GetId()] = entry
	}
	r.configuredAgents = replacement
}

// List returns all registered agents, pool agent first.
func (r *AgentRegistry) List() []*apiv1.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*apiv1.AgentInfo, 0, 1+len(r.configuredAgents))
	if r.poolAgent != nil {
		clone, ok := proto.Clone(r.poolAgent).(*apiv1.AgentInfo)
		if ok {
			out = append(out, clone)
		}
	}
	for _, a := range r.configuredAgents {
		clone, ok := proto.Clone(a).(*apiv1.AgentInfo)
		if ok {
			out = append(out, clone)
		}
	}
	return out
}

// FindByID returns the AgentInfo for the given id, or nil if not found.
func (r *AgentRegistry) FindByID(id string) *apiv1.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.poolAgent != nil && r.poolAgent.GetId() == id {
		if clone, ok := proto.Clone(r.poolAgent).(*apiv1.AgentInfo); ok {
			return clone
		}
		return nil
	}
	if a, ok := r.configuredAgents[id]; ok {
		if clone, ok := proto.Clone(a).(*apiv1.AgentInfo); ok {
			return clone
		}
		return nil
	}
	return nil
}

// DefaultAgentID returns the ID of the agent used when no per-page binding
// has been set — the bundled local ACP pool agent.
func (*AgentRegistry) DefaultAgentID() string {
	return LocalACPAgentID
}
