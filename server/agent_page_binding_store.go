package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	agentChatAgentKey = "chat_agent"

	bindingAgentIDKey     = "agent_id"
	bindingDisplayNameKey = "display_name"
	bindingUpdatedByKey   = "updated_by"
	bindingUpdatedAtKey   = "updated_at"
)

// IsValidRFC3339 reports whether s parses as an RFC3339 timestamp. Exported
// for OCC validation at the API edge.
func IsValidRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

// ErrBindingConflict is returned by SetBinding when the expected_updated_at
// OCC guard does not match the persisted binding.
var ErrBindingConflict = errors.New("agent binding changed concurrently")

// AgentBindingInput is the user-intent portion of a page agent binding.
type AgentBindingInput struct {
	AgentID     string
	DisplayName string

	// UpdatedBy is the switcher's Tailscale identity, derived from auth
	// context by the funnel (ADR-0009 §5) — never from input on
	// user-reachable paths.
	UpdatedBy string

	// ExpectedUpdatedAt is an optional OCC guard (RFC3339). Empty means
	// unconditional write.
	ExpectedUpdatedAt string
}

// PageAgentBinding is the persisted per-page agent binding.
type PageAgentBinding struct {
	AgentID     string
	DisplayName string
	UpdatedBy   string
	UpdatedAt   string // RFC3339, server-stamped
}

// AgentPageBindingStore reads and writes the agent.chat_agent subtree on each
// page. Per ADR-0009 it is the exclusive write-path for that subtree; the
// generic Frontmatter API rejects writes to the reserved agent.* namespace.
type AgentPageBindingStore struct {
	pages agentSchedulePagesStore
}

// NewAgentPageBindingStore constructs a store on top of the given page accessor.
func NewAgentPageBindingStore(pages agentSchedulePagesStore) *AgentPageBindingStore {
	return &AgentPageBindingStore{pages: pages}
}

// GetBinding returns the page's persisted agent binding, or nil when the page
// has none.
func (s *AgentPageBindingStore) GetBinding(page string) (*PageAgentBinding, error) {
	_, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return nil, fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	return extractAgentBinding(fm), nil
}

// SetBinding writes the binding into agent.chat_agent with server-stamped
// updated_at. Returns the stamped timestamp. An empty AgentID with no OCC
// guard removes the binding.
func (s *AgentPageBindingStore) SetBinding(page string, input *AgentBindingInput) (string, error) {
	if input == nil {
		return "", errors.New("binding input is required")
	}

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return "", fmt.Errorf(errReadFrontmatterFmt, page, err)
	}

	agentNS := ensureAgentNamespace(fm)

	existing := extractAgentBinding(fm)
	if input.ExpectedUpdatedAt != "" {
		if existing == nil || existing.UpdatedAt != input.ExpectedUpdatedAt {
			return "", ErrBindingConflict
		}
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)

	if input.AgentID == "" && input.ExpectedUpdatedAt == "" {
		delete(agentNS, agentChatAgentKey)
	} else {
		agentNS[agentChatAgentKey] = map[string]any{
			bindingAgentIDKey:     input.AgentID,
			bindingDisplayNameKey: input.DisplayName,
			bindingUpdatedByKey:   input.UpdatedBy,
			bindingUpdatedAtKey:   updatedAt,
		}
	}

	if err := s.pages.WriteFrontMatter(id, fm, wikipage.AnonymousIdentity); err != nil {
		return "", fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return updatedAt, nil
}

// extractAgentBinding reads agent.chat_agent from a frontmatter map.
func extractAgentBinding(fm wikipage.FrontMatter) *PageAgentBinding {
	if fm == nil {
		return nil
	}
	agentNS, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		return nil
	}
	sub, ok := agentNS[agentChatAgentKey].(map[string]any)
	if !ok {
		return nil
	}
	agentID, _ := sub[bindingAgentIDKey].(string)
	if agentID == "" {
		return nil
	}
	displayName, _ := sub[bindingDisplayNameKey].(string)
	updatedBy, _ := sub[bindingUpdatedByKey].(string)
	updatedAt, _ := sub[bindingUpdatedAtKey].(string)
	return &PageAgentBinding{
		AgentID:     agentID,
		DisplayName: displayName,
		UpdatedBy:   updatedBy,
		UpdatedAt:   updatedAt,
	}
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
