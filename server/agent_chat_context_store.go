package server

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// BackgroundActivityMax is the maximum number of background-activity entries
// retained per page. The list is a ring buffer; older entries drop when the
// cap is exceeded so the chat preamble stays bounded.
const BackgroundActivityMax = 50

const agentChatContextKey = "chat_context"

// SummaryTargetNotFoundError is returned by AppendBackgroundActivitySummary
// when no recent background-activity entry matches the supplied schedule_id.
// Indicates the agent called the summary tool out of order (e.g. before the
// schedule fired) or after the entry rolled out of the ring buffer.
type SummaryTargetNotFoundError struct {
	Page       string
	ScheduleID string
}

// Error implements error.
func (e *SummaryTargetNotFoundError) Error() string {
	return fmt.Sprintf("no recent background-activity entry for schedule %q on page %q", e.ScheduleID, e.Page)
}

// AgentChatContextStore owns reads and writes of the agent.chat_context
// subtree on each page. Like AgentScheduleStore, it is the only legal
// write-path for that namespace; the generic Frontmatter API rejects writes.
type AgentChatContextStore struct {
	pages agentSchedulePagesStore
	mu    sync.Mutex
}

// NewAgentChatContextStore constructs a chat-context store on top of the
// supplied page accessor.
func NewAgentChatContextStore(pages agentSchedulePagesStore) *AgentChatContextStore {
	return &AgentChatContextStore{pages: pages}
}

// Read returns the stored ChatContext for a page. A page with no chat context
// returns an empty (non-nil) ChatContext and no error so callers can always
// dereference fields safely.
func (s *AgentChatContextStore) Read(page string) (*apiv1.ChatContext, error) {
	_, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return nil, fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	return decodeChatContext(fm)
}

// UpdateMerge deep-merges the supplied ChatContext into the stored one and
// returns the merged result.
//
// Merge semantics:
//   - Scalar string fields (last_conversation_summary, key_context) replace
//     when the update sets them; an empty update string leaves the prior
//     value alone.
//   - Repeated string fields (user_goals, pending_items) are unioned; the
//     order of first appearance is preserved.
//   - background_activity is a ring buffer managed by the
//     AppendBackgroundActivity* helpers and is NOT touched by UpdateMerge.
//   - last_updated is server-stamped on every call.
func (s *AgentChatContextStore) UpdateMerge(page string, update *apiv1.ChatContext) (*apiv1.ChatContext, error) {
	if update == nil {
		return nil, errors.New("update is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return nil, fmt.Errorf(errReadFrontmatterFmt, page, err)
	}

	existing, err := decodeChatContext(fm)
	if err != nil {
		return nil, err
	}

	merged := mergeChatContext(existing, update)
	merged.LastUpdated = timestamppb.New(time.Now().UTC())

	if err := writeChatContext(fm, merged); err != nil {
		return nil, err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return nil, fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return merged, nil
}

// AppendBackgroundActivityAutomatic adds a new BackgroundActivityEntry to the
// page's chat context. The list is capped at BackgroundActivityMax; older
// entries (front of the slice) drop when the cap is exceeded.
//
// Called by the schedule store on every terminal status transition; the agent
// itself calls AppendBackgroundActivitySummary later to attach a one-sentence
// summary to the entry it just produced.
func (s *AgentChatContextStore) AppendBackgroundActivityAutomatic(page string, entry *apiv1.BackgroundActivityEntry) error {
	if entry == nil {
		return errors.New("entry is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	existing, err := decodeChatContext(fm)
	if err != nil {
		return err
	}

	existing.BackgroundActivity = append(existing.BackgroundActivity, entry)
	if len(existing.BackgroundActivity) > BackgroundActivityMax {
		drop := len(existing.BackgroundActivity) - BackgroundActivityMax
		existing.BackgroundActivity = existing.BackgroundActivity[drop:]
	}

	if err := writeChatContext(fm, existing); err != nil {
		return err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// AppendBackgroundActivitySummary attaches summary to the most recent
// background-activity entry whose schedule_id matches scheduleID. Returns a
// SummaryTargetNotFoundError when no matching entry exists (e.g. the agent
// called the summary tool out of order or after the entry rolled out of the
// ring buffer).
func (s *AgentChatContextStore) AppendBackgroundActivitySummary(page, scheduleID, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, fm, err := s.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return fmt.Errorf(errReadFrontmatterFmt, page, err)
	}
	existing, err := decodeChatContext(fm)
	if err != nil {
		return err
	}

	// Walk newest-first (i.e. from the back of the slice) and stop at the
	// first matching schedule_id.
	target := -1
	for i := len(existing.BackgroundActivity) - 1; i >= 0; i-- {
		if existing.BackgroundActivity[i].GetScheduleId() == scheduleID {
			target = i
			break
		}
	}
	if target == -1 {
		return &SummaryTargetNotFoundError{Page: page, ScheduleID: scheduleID}
	}

	existing.BackgroundActivity[target].Summary = summary
	if err := writeChatContext(fm, existing); err != nil {
		return err
	}
	if err := s.pages.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf(errWriteFrontmatterFmt, page, err)
	}
	return nil
}

// decodeChatContext extracts agent.chat_context out of a frontmatter map and
// returns a typed ChatContext. Returns an empty ChatContext when missing.
func decodeChatContext(fm wikipage.FrontMatter) (*apiv1.ChatContext, error) {
	agent, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		return &apiv1.ChatContext{}, nil
	}
	raw, ok := agent[agentChatContextKey]
	if !ok {
		return &apiv1.ChatContext{}, nil
	}
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("agent.chat_context has unexpected type %T", raw)
	}
	bytes, err := mapToJSONBytes(rawMap)
	if err != nil {
		return nil, err
	}
	out := &apiv1.ChatContext{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(bytes, out); err != nil {
		return nil, err
	}
	return out, nil
}

// writeChatContext replaces agent.chat_context in fm with the supplied
// ChatContext. Other top-level keys are left untouched.
func writeChatContext(fm wikipage.FrontMatter, ctx *apiv1.ChatContext) error {
	bytes, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}.Marshal(ctx)
	if err != nil {
		return err
	}
	rawMap, err := jsonBytesToMap(bytes)
	if err != nil {
		return err
	}

	agent, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		agent = map[string]any{}
	}
	agent[agentChatContextKey] = rawMap
	fm[AgentNamespaceKey] = agent
	return nil
}

// mergeChatContext combines an existing context with an update, applying the
// merge rules documented on UpdateMerge.
func mergeChatContext(existing, update *apiv1.ChatContext) *apiv1.ChatContext {
	if existing == nil {
		existing = &apiv1.ChatContext{}
	}
	merged := &apiv1.ChatContext{
		LastConversationSummary: existing.GetLastConversationSummary(),
		UserGoals:               append([]string{}, existing.GetUserGoals()...),
		PendingItems:            append([]string{}, existing.GetPendingItems()...),
		KeyContext:              existing.GetKeyContext(),
		LastUpdated:             existing.GetLastUpdated(),
		// background_activity is preserved verbatim; it is not part of the
		// merge contract.
		BackgroundActivity: existing.GetBackgroundActivity(),
	}

	if update.GetLastConversationSummary() != "" {
		merged.LastConversationSummary = update.GetLastConversationSummary()
	}
	if update.GetKeyContext() != "" {
		merged.KeyContext = update.GetKeyContext()
	}
	merged.UserGoals = unionStrings(merged.UserGoals, update.GetUserGoals())
	merged.PendingItems = unionStrings(merged.PendingItems, update.GetPendingItems())
	return merged
}

// unionStrings appends elements from add that are not already in base. Order
// of first appearance is preserved.
func unionStrings(base, add []string) []string {
	seen := map[string]struct{}{}
	for _, s := range base {
		seen[s] = struct{}{}
	}
	out := append([]string{}, base...)
	for _, s := range add {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
