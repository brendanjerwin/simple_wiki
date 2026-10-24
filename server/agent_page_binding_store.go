package server

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const agentSelectedAgentIDKey = "selected_agent_id"

// AgentPageBindingStore reads and writes the agent.selected_agent_id subtree
// on each page. It is the exclusive write-path for that field; the generic
// Frontmatter API rejects writes to the agent.* namespace.
type AgentPageBindingStore struct {
	pages agentSchedulePagesStore
}

// NewAgentPageBindingStore constructs a store on top of the given page accessor.
func NewAgentPageBindingStore(pages agentSchedulePagesStore) *AgentPageBindingStore {
	return &AgentPageBindingStore{pages: pages}
}

// GetSelectedAgentID returns the agent ID stored in agent.selected_agent_id
// for a page, or "" if none has been set.
func (s *AgentPageBindingStore) GetSelectedAgentID(page string) (string, error) {
	_, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return "", fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	return extractSelectedAgentID(fm), nil
}

// SetSelectedAgentID writes agentID into agent.selected_agent_id for a page.
// An empty agentID removes the binding.
func (s *AgentPageBindingStore) SetSelectedAgentID(page, agentID string) error {
	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}

	agentNS := ensureAgentNamespace(fm)
	if agentID == "" {
		delete(agentNS, agentSelectedAgentIDKey)
	} else {
		agentNS[agentSelectedAgentIDKey] = agentID
	}

	if err := s.pages.WriteFrontMatter(id, fm, wikipage.AnonymousIdentity); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// extractSelectedAgentID reads agent.selected_agent_id from a frontmatter map.
func extractSelectedAgentID(fm wikipage.FrontMatter) string {
	if fm == nil {
		return ""
	}
	agentNS, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := agentNS[agentSelectedAgentIDKey].(string)
	return id
}

// ensureAgentNamespace returns the agent.* sub-map from fm, creating it if
// absent. The caller must write fm back to persist the change.
func ensureAgentNamespace(fm wikipage.FrontMatter) map[string]any {
	raw, ok := fm[AgentNamespaceKey]
	if ok {
		if ns, ok2 := raw.(map[string]any); ok2 {
			return ns
		}
	}
	ns := make(map[string]any)
	fm[AgentNamespaceKey] = ns
	return ns
}
