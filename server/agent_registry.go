package server

import (
	"errors"
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
// from two authorities (issue #1276 design, 2026-10-24; extended by #1275 P2):
//
//   - ACP_LOCAL entries exist only after the pool daemon registers itself
//     (RegisterPoolAgent) and become unavailable again when the pool
//     connection drops (PoolDisconnected). The pool is the sole authority on
//     local agents.
//   - AG_UI_REMOTE entries come either from the wiki server's static
//     configuration (SetConfiguredAgents) or from pool registration
//     (RegisterPoolAgent with AG_UI_REMOTE). Server-config entries start
//     unavailable and become available when the pool registers them; the pool
//     may also register AG-UI agents not in server config. PoolDisconnected
//     marks all pool-registered AG-UI agents unavailable.
//
// Config cannot overwrite a pool-registered descriptor: the pool is the
// authority for its own agents.
type AgentRegistry struct {
	mu sync.RWMutex

	// poolRegistered tracks whether the pool currently has its ACP agent
	// registered (available).
	poolRegistered bool

	// poolAgent is the pool-registered ACP_LOCAL descriptor (nil until registered).
	poolAgent *apiv1.AgentInfo

	// configuredAgents are the AG-UI entries keyed by id; populated by
	// SetConfiguredAgents (server static config) and updated by pool registration.
	configuredAgents map[string]*apiv1.AgentInfo

	// agUIPoolIDs holds the ids of AG-UI agents registered by the pool
	// (via RegisterPoolAgent with AG_UI_REMOTE), so PoolDisconnected can
	// mark them unavailable alongside the ACP agent.
	agUIPoolIDs []string
}

// NewAgentRegistry constructs an empty registry; the local ACP agent lists
// (as unavailable) as soon as the pool registers it.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		configuredAgents: make(map[string]*apiv1.AgentInfo),
	}
}

// RegisterPoolAgent idempotently upserts a pool-managed agent descriptor and
// marks it available. Two protocols are accepted:
//
//   - ACP_LOCAL: only LocalACPAgentID may register; replaces the pool agent.
//   - AG_UI_REMOTE: any non-empty id is accepted. If the id already exists in
//     configuredAgents (server static config), that entry is marked available.
//     If not, a new entry is created. All AG-UI ids registered this way are
//     tracked in agUIPoolIDs so PoolDisconnected can clear them.
func (r *AgentRegistry) RegisterPoolAgent(req *apiv1.RegisterPoolAgentRequest) error {
	switch req.GetProtocol() {
	case apiv1.AgentProtocol_ACP_LOCAL:
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

	case apiv1.AgentProtocol_AG_UI_REMOTE:
		if req.GetId() == "" {
			return errors.New("RegisterPoolAgent: id is required for AG_UI_REMOTE agents")
		}

		r.mu.Lock()
		defer r.mu.Unlock()

		r.registerAGUIAgent(req)
		return nil

	default:
		return fmt.Errorf("RegisterPoolAgent: unsupported protocol %s; expected ACP_LOCAL or AG_UI_REMOTE", req.GetProtocol())
	}
}

// registerAGUIAgent marks a pool-reported AG-UI agent available, creating the
// entry if it does not yet exist in configuredAgents. Must be called with
// r.mu held.
func (r *AgentRegistry) registerAGUIAgent(req *apiv1.RegisterPoolAgentRequest) {
	id := req.GetId()

	// Track for PoolDisconnected — add only if not already tracked.
	tracked := false
	for _, existing := range r.agUIPoolIDs {
		if existing == id {
			tracked = true
			break
		}
	}
	if !tracked {
		r.agUIPoolIDs = append(r.agUIPoolIDs, id)
	}

	if entry, ok := r.configuredAgents[id]; ok {
		// Update the existing server-config entry: mark available and refresh
		// mutable fields the pool knows best (display name, description, route).
		entry.Available = true
		if req.GetDisplayName() != "" {
			entry.DisplayName = req.GetDisplayName()
		}
		if req.GetDescription() != "" {
			entry.Description = req.GetDescription()
		}
		if req.GetRoute() != "" {
			entry.Route = req.GetRoute()
		}
	} else {
		// Pool knows about an agent the server config doesn't — add it.
		r.configuredAgents[id] = &apiv1.AgentInfo{
			Id:          id,
			DisplayName: req.GetDisplayName(),
			Protocol:    apiv1.AgentProtocol_AG_UI_REMOTE,
			Description: req.GetDescription(),
			Route:       req.GetRoute(),
			Available:   true,
		}
	}
}

// PoolDisconnected marks the pool-registered ACP agent and all pool-registered
// AG-UI agents unavailable while keeping their entries listed.
func (r *AgentRegistry) PoolDisconnected() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.poolRegistered = false
	if r.poolAgent != nil {
		r.poolAgent.Available = false
	}
	for _, id := range r.agUIPoolIDs {
		if entry, ok := r.configuredAgents[id]; ok {
			entry.Available = false
		}
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
